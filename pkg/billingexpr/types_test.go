package billingexpr

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBillingSnapshotEvaluationTimeJSONRoundTrip(t *testing.T) {
	evaluationTime := time.Date(2026, time.October, 1, 15, 4, 5, 123456789, time.FixedZone("CST", 8*60*60))
	original := BillingSnapshot{EvaluationTime: evaluationTime}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"evaluation_time"`)

	var decoded BillingSnapshot
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, decoded.EvaluationTime.Equal(evaluationTime))

	var legacy BillingSnapshot
	require.NoError(t, json.Unmarshal([]byte(`{"expr_string":"tier(\"base\", 1)"}`), &legacy))
	require.True(t, legacy.EvaluationTime.IsZero())
}
