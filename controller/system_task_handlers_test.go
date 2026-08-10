package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAsyncTaskBillingHandlerTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TaskBillingJob{}))
	previousDB := model.DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})
	return db
}

func TestAsyncTaskBillingHandlerScheduleContract(t *testing.T) {
	handler := asyncTaskBillingReconcileHandler{}
	assert.Equal(t, model.SystemTaskTypeAsyncTaskBillingReconcile, handler.Type())
	assert.Equal(t, 15*time.Second, handler.Interval())
	assert.Nil(t, handler.NewPayload())
}

func TestAsyncTaskBillingHandlerEnabledWhenPollingDisabled(t *testing.T) {
	db := setupAsyncTaskBillingHandlerTest(t)
	previousUpdateTask := constant.UpdateTask
	constant.UpdateTask = false
	t.Cleanup(func() { constant.UpdateTask = previousUpdateTask })

	now := time.Now().Unix()
	job := &model.TaskBillingJob{
		TaskID:         88001,
		IdempotencyKey: "controller-reconcile-pending",
		Operation:      model.TaskBillingOperationRefund,
		FromQuota:      10,
		Status:         model.TaskBillingJobStatusPending,
		NextAttemptAt:  now - 1,
	}
	require.NoError(t, db.Create(job).Error)

	assert.True(t, (asyncTaskBillingReconcileHandler{}).Enabled())
}

func TestAsyncTaskBillingHandlerEnabledForExpiredLeaseButNotFutureWork(t *testing.T) {
	db := setupAsyncTaskBillingHandlerTest(t)
	now := time.Now().Unix()
	future := &model.TaskBillingJob{
		TaskID:         88002,
		IdempotencyKey: "controller-reconcile-future",
		Operation:      model.TaskBillingOperationRefund,
		FromQuota:      10,
		Status:         model.TaskBillingJobStatusPending,
		NextAttemptAt:  now + 3600,
	}
	require.NoError(t, db.Create(future).Error)
	assert.False(t, (asyncTaskBillingReconcileHandler{}).Enabled())

	expired := &model.TaskBillingJob{
		TaskID:         88003,
		IdempotencyKey: "controller-reconcile-expired",
		Operation:      model.TaskBillingOperationRefund,
		FromQuota:      10,
		Status:         model.TaskBillingJobStatusProcessing,
		Attempts:       1,
		LockedBy:       "dead-runner",
		LockedUntil:    now - 1,
	}
	require.NoError(t, db.Create(expired).Error)
	assert.True(t, (asyncTaskBillingReconcileHandler{}).Enabled())
}
