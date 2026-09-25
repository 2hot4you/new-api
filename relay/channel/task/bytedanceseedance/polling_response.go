package bytedanceseedance

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	"github.com/QuantumNous/new-api/service"
)

// Exactly the known wire paths: OpenAI video; TaskResponse<TaskDto>; and
// TaskDto.Data containing StarAI's response with up to two data wrappers.
// No recursive search through diagnostics, arrays or arbitrary JSON strings.
func pollingNodes(body []byte) ([]map[string]json.RawMessage, error) {
	var root map[string]json.RawMessage
	if len(body) > 1<<20 || seedanceprotocol.DecodeResponse(body, &root) != nil || root == nil {
		return nil, errors.New("invalid Molii video task response")
	}
	nodes := []map[string]json.RawMessage{root}
	for depth := 0; depth < 4; depth++ {
		var child map[string]json.RawMessage
		if json.Unmarshal(nodes[depth]["data"], &child) != nil || child == nil {
			break
		}
		nodes = append(nodes, child)
	}
	return nodes, nil
}

func nodeString(node map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(node[key], &value)
	return value
}

func publicPollingFacts(nodes []map[string]json.RawMessage) service.SeedanceTaskFacts {
	var facts service.SeedanceTaskFacts
	timestamp := func(raw json.RawMessage) int64 {
		var n int64
		if json.Unmarshal(raw, &n) == nil && n > 0 && n <= 32503680000 {
			return n
		}
		return 0
	}
	// Prefer the immediate upstream's public task timestamps over underlying
	// provider data, so queue/generation measurements belong to that hop.
	order := append([]map[string]json.RawMessage{}, nodes[1:]...)
	order = append(order, nodes[0])
	for _, n := range order {
		if facts.SubmitTime == 0 {
			facts.SubmitTime = timestamp(n["submit_time"])
		}
		if facts.StartTime == 0 {
			facts.StartTime = timestamp(n["start_time"])
		}
		if facts.FinishTime == 0 {
			facts.FinishTime = timestamp(n["finish_time"])
		}
	}
	for _, n := range order {
		if facts.SubmitTime == 0 {
			facts.SubmitTime = timestamp(n["created_at"])
		}
		if facts.FinishTime == 0 {
			facts.FinishTime = timestamp(n["completed_at"])
		}
	}
	if facts.StartTime > 0 && facts.StartTime < facts.SubmitTime {
		facts.StartTime = 0
	}
	if facts.FinishTime > 0 && (facts.FinishTime < facts.SubmitTime || facts.FinishTime < facts.StartTime) {
		facts.FinishTime = 0
	}
	for i := len(nodes) - 1; i >= 0; i-- {
		n := nodes[i]
		mergePublicVideoFacts(&facts, n)
		var params map[string]json.RawMessage
		if json.Unmarshal(n["video_params"], &params) == nil {
			mergePublicVideoFacts(&facts, params)
		}
	}
	return facts
}

func mergePublicVideoFacts(f *service.SeedanceTaskFacts, n map[string]json.RawMessage) {
	if f.Duration == 0 {
		for _, key := range []string{"duration", "seconds"} {
			var value float64
			if json.Unmarshal(n[key], &value) == nil && value > 0 && value <= 300 {
				f.Duration = value
				break
			}
		}
	}
	if f.Resolution == "" {
		f.Resolution = safeResolution(nodeString(n, "resolution"))
	}
	if f.Ratio == "" {
		switch value := nodeString(n, "ratio"); value {
		case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "3:2", "2:3", "adaptive":
			f.Ratio = value
		}
	}
	for _, field := range []struct {
		key    string
		target **int
	}{{"input_image_count", &f.InputImageCount}, {"input_video_count", &f.InputVideoCount}, {"input_audio_count", &f.InputAudioCount}} {
		var value *int
		if *field.target == nil && json.Unmarshal(n[field.key], &value) == nil && value != nil && *value >= 0 && *value <= 50 {
			*field.target = value
		}
	}
}
