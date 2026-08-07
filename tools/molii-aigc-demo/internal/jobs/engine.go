package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID            string
	EnvironmentID string
	TaskID        string
	Operation     string
	Attempt       int
	CreatedAt     time.Time
	NextPollAt    time.Time
}

type PollResult struct {
	Status    string
	Progress  int
	ResultURL string
	Error     string
	Terminal  bool
	Success   bool
	Raw       []byte
}

type Update struct {
	JobID      string
	Attempt    int
	PolledAt   time.Time
	NextPollAt time.Time
	Result     PollResult
	PollError  string
	Exhausted  bool
}

// Repository is deliberately persistence-agnostic. ListRunnable is the
// restart boundary: every engine start asks durable storage for unfinished
// work instead of relying on in-memory timers.
type Repository interface {
	ListRunnable(ctx context.Context, due time.Time, limit int) ([]Job, error)
	SavePoll(ctx context.Context, update Update) error
	Cancel(ctx context.Context, jobID string) error
}

type Poller interface {
	Poll(ctx context.Context, job Job) (PollResult, error)
}

type Config struct {
	ScanInterval time.Duration
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxAttempts  int
	BatchSize    int
	PollTimeout  time.Duration
}

func (c Config) withDefaults() Config {
	if c.ScanInterval <= 0 {
		c.ScanInterval = time.Second
	}
	if c.InitialDelay <= 0 {
		c.InitialDelay = 3 * time.Second
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 30 * time.Second
	}
	if c.MaxDelay < c.InitialDelay {
		c.MaxDelay = c.InitialDelay
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 120
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 32
	}
	if c.PollTimeout <= 0 {
		c.PollTimeout = 15 * time.Second
	}
	return c
}

type Engine struct {
	repository Repository
	poller     Poller
	config     Config
	now        func() time.Time

	mu       sync.Mutex
	active   map[string]context.CancelFunc
	canceled map[string]bool
	running  bool
}

func New(repository Repository, poller Poller, config Config) (*Engine, error) {
	if repository == nil || poller == nil {
		return nil, errors.New("repository and poller are required")
	}
	return &Engine{repository: repository, poller: poller, config: config.withDefaults(), now: time.Now, active: make(map[string]context.CancelFunc), canceled: make(map[string]bool)}, nil
}

// Start runs until ctx is canceled. Its immediate RunOnce is what resumes
// durable unfinished jobs after a process restart.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("polling engine is already running")
	}
	e.running = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running = false
		for _, cancel := range e.active {
			cancel()
		}
		e.active = make(map[string]context.CancelFunc)
		e.mu.Unlock()
	}()

	if err := e.RunOnce(ctx); err != nil && ctx.Err() == nil {
		return err
	}
	ticker := time.NewTicker(e.config.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := e.RunOnce(ctx); err != nil && ctx.Err() == nil {
				return err
			}
		}
	}
}

func (e *Engine) RunOnce(ctx context.Context) error {
	now := e.now()
	list, err := e.repository.ListRunnable(ctx, now, e.config.BatchSize)
	if err != nil {
		return fmt.Errorf("list runnable jobs: %w", err)
	}
	for _, job := range list {
		if err := e.pollOne(ctx, now, job); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) pollOne(parent context.Context, now time.Time, job Job) error {
	if job.ID == "" || job.TaskID == "" {
		return errors.New("repository returned a job without ID or task ID")
	}
	e.mu.Lock()
	if _, exists := e.active[job.ID]; exists {
		e.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, e.config.PollTimeout)
	e.active[job.ID] = cancel
	e.mu.Unlock()

	result, pollErr := e.poller.Poll(ctx, job)
	cancel()
	if parent.Err() != nil {
		e.mu.Lock()
		delete(e.active, job.ID)
		delete(e.canceled, job.ID)
		e.mu.Unlock()
		return parent.Err()
	}
	attempt := job.Attempt + 1
	update := Update{JobID: job.ID, Attempt: attempt, PolledAt: now, Result: result}
	if pollErr != nil {
		update.PollError = pollErr.Error()
	}
	if !result.Terminal {
		if attempt >= e.config.MaxAttempts {
			update.Exhausted = true
			update.Result = PollResult{Status: "failed", Terminal: true, Error: "polling attempt limit reached"}
		} else {
			update.NextPollAt = now.Add(e.Backoff(attempt))
		}
	}
	// Serialize persistence with Cancel: either the poll is saved first and a
	// later cancellation wins, or cancellation is observed and no stale poll
	// update is written.
	e.mu.Lock()
	delete(e.active, job.ID)
	wasCanceled := e.canceled[job.ID]
	delete(e.canceled, job.ID)
	if wasCanceled {
		e.mu.Unlock()
		return nil
	}
	err := e.repository.SavePoll(parent, update)
	e.mu.Unlock()
	if err != nil {
		return fmt.Errorf("save poll result for %s: %w", job.ID, err)
	}
	return nil
}

// Backoff returns an exponential delay capped at MaxDelay. There is no random
// component so persisted schedules remain predictable across restarts.
func (e *Engine) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := e.config.InitialDelay
	for i := 1; i < attempt && delay < e.config.MaxDelay; i++ {
		if delay > e.config.MaxDelay/2 {
			return e.config.MaxDelay
		}
		delay *= 2
	}
	if delay > e.config.MaxDelay {
		return e.config.MaxDelay
	}
	return delay
}

// Cancel persists cancellation first, then interrupts an in-flight HTTP poll.
func (e *Engine) Cancel(ctx context.Context, jobID string) error {
	if jobID == "" {
		return errors.New("job ID is required")
	}
	e.mu.Lock()
	cancel := e.active[jobID]
	if cancel != nil {
		e.canceled[jobID] = true
	}
	if err := e.repository.Cancel(ctx, jobID); err != nil {
		delete(e.canceled, jobID)
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}
