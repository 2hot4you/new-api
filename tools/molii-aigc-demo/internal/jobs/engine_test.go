package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	mu        sync.Mutex
	jobs      []Job
	updates   []Update
	cancelled []string
}

func (r *fakeRepository) ListRunnable(context.Context, time.Time, int) ([]Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Job(nil), r.jobs...), nil
}
func (r *fakeRepository) SavePoll(_ context.Context, update Update) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, update)
	return nil
}
func (r *fakeRepository) Cancel(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancelled = append(r.cancelled, id)
	return nil
}

type fakePoller struct {
	result  PollResult
	err     error
	started chan struct{}
}

func (p *fakePoller) Poll(ctx context.Context, _ Job) (PollResult, error) {
	if p.started != nil {
		close(p.started)
		<-ctx.Done()
		return PollResult{}, ctx.Err()
	}
	return p.result, p.err
}

func TestRunOnceResumesRepositoryJobsAndSchedulesBoundedBackoff(t *testing.T) {
	repository := &fakeRepository{jobs: []Job{{ID: "run_1", TaskID: "task_1", Attempt: 2}}}
	engine, err := New(repository, &fakePoller{result: PollResult{Status: "in_progress", Progress: 42}}, Config{InitialDelay: time.Second, MaxDelay: 5 * time.Second, MaxAttempts: 5})
	require.NoError(t, err)
	now := time.Unix(100, 0)
	engine.now = func() time.Time { return now }
	require.NoError(t, engine.RunOnce(context.Background()))
	require.Len(t, repository.updates, 1)
	require.Equal(t, 3, repository.updates[0].Attempt)
	require.Equal(t, now.Add(4*time.Second), repository.updates[0].NextPollAt)
	require.Equal(t, 5*time.Second, engine.Backoff(20))
}

func TestAttemptLimitMakesPollingTerminal(t *testing.T) {
	repository := &fakeRepository{jobs: []Job{{ID: "run_1", TaskID: "task_1", Attempt: 1}}}
	engine, err := New(repository, &fakePoller{err: errors.New("temporary")}, Config{MaxAttempts: 2})
	require.NoError(t, err)
	require.NoError(t, engine.RunOnce(context.Background()))
	require.True(t, repository.updates[0].Exhausted)
	require.True(t, repository.updates[0].Result.Terminal)
}

func TestCancelPersistsAndInterruptsInflightPoll(t *testing.T) {
	repository := &fakeRepository{jobs: []Job{{ID: "run_1", TaskID: "task_1"}}}
	poller := &fakePoller{started: make(chan struct{})}
	engine, err := New(repository, poller, Config{PollTimeout: time.Minute})
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- engine.RunOnce(context.Background()) }()
	<-poller.started
	require.NoError(t, engine.Cancel(context.Background(), "run_1"))
	require.NoError(t, <-done)
	require.Equal(t, []string{"run_1"}, repository.cancelled)
	require.Empty(t, repository.updates, "cancel must not be overwritten by a late poll update")
}
