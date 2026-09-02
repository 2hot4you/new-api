package common

import (
	"sync/atomic"

	"github.com/bytedance/gopkg/util/gopool"
)

var backgroundTaskSynchronous atomic.Bool

// SetBackgroundTaskSynchronous configures request-scoped background work to
// finish inline. Production keeps the default asynchronous mode; test
// processes can enable this once before running cases that replace global
// database or Redis clients.
func SetBackgroundTaskSynchronous(enabled bool) {
	backgroundTaskSynchronous.Store(enabled)
}

// RunBackgroundTask dispatches short request-scoped follow-up work. Keeping
// this boundary explicit lets tests finish side effects before their database
// and Redis fixtures are replaced without changing production scheduling.
func RunBackgroundTask(task func()) {
	if task == nil {
		return
	}
	if backgroundTaskSynchronous.Load() {
		task()
		return
	}
	gopool.Go(task)
}
