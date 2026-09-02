package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackgroundTaskSynchronousModeCompletesBeforeReturning(t *testing.T) {
	SetBackgroundTaskSynchronous(true)
	t.Cleanup(func() { SetBackgroundTaskSynchronous(false) })

	completed := false
	RunBackgroundTask(func() {
		completed = true
	})

	require.True(t, completed)
}

func TestBackgroundTaskDefaultModeDoesNotBlockCaller(t *testing.T) {
	SetBackgroundTaskSynchronous(false)

	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})
	RunBackgroundTask(func() {
		close(started)
		<-release
		close(completed)
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background task did not start")
	}
	select {
	case <-completed:
		t.Fatal("background task completed before it was released")
	default:
	}
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("background task did not complete after release")
	}
}
