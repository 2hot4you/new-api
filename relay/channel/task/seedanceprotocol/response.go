package seedanceprotocol

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func DecodeResponse(body []byte, target any) error {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}))
	if err := common.Unmarshal(trimmed, target); err == nil {
		return nil
	}

	text := strings.TrimSpace(string(trimmed))
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
			if err := common.Unmarshal([]byte(text), target); err == nil {
				return nil
			}
		}
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if err := common.Unmarshal([]byte(payload), target); err == nil {
			return nil
		}
	}
	return fmt.Errorf("response is not valid JSON")
}
