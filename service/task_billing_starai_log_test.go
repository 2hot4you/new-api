package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestSeedanceEstimateLogContentIncludesParametersAndFormula(t *testing.T) {
	info := &relaycommon.RelayInfo{
		EstimatedVideoTokens:     40595,
		EstimatedVideoPrice:      1.502015,
		EstimatedVideoWidth:      864,
		EstimatedVideoHeight:     496,
		EstimatedVideoFPS:        24,
		EstimatedVideoSeconds:    4,
		EstimatedVideoResolution: "480p",
		EstimatedVideoRatio:      "16:9",
		EstimatedVideoUnitPrice:  37,
	}

	content := seedanceEstimateLogContent(info)
	assert.Equal(t, "生成视频\n参数：480p · 16:9 · 4 秒 · 不含视频输入\n预估 Token：40595\n计算：864 × 496 × (24 × 4 + 1) ÷ 1024，结果向上取整\n提示：Seedance 会额外生成 1 帧用于起播，因此总帧数按 24 × 4 + 1 计算\n预估价格：40595 × ¥37.00 ÷ 1,000,000 = ¥1.502015", content)
}
