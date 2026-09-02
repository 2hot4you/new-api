package controller

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestMain(m *testing.M) {
	common.SetBackgroundTaskSynchronous(true)
	os.Exit(m.Run())
}
