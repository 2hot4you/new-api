package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestMoliiGrokFixedPriceBillingContextKeepsVideoAuditFieldsWithoutTokenEstimate(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		ModelPrice:          0.5,
		GroupRatio:          1,
		PerCallBilling:      true,
		EstimatedTokens:     0,
		EstimatedSeconds:    5,
		EstimatedResolution: "480p",
		EstimatedRatio:      "16:9",
	}}}

	other := taskBillingOther(task)
	assert.Equal(t, 5, other["estimated_seconds"])
	assert.Equal(t, "480p", other["estimated_resolution"])
	assert.Equal(t, "16:9", other["estimated_ratio"])
	assert.NotContains(t, other, "estimated_tokens")
}
