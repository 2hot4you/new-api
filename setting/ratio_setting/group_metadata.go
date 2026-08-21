package ratio_setting

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

type GroupMetadata struct {
	Name           string  `json:"name"`
	Icon           string  `json:"icon"`
	Recommendation float64 `json:"recommendation"`
}

type groupMetadataStore struct {
	mutex   sync.RWMutex
	entries []GroupMetadata
}

func newGroupMetadataStore() *groupMetadataStore {
	return &groupMetadataStore{entries: make([]GroupMetadata, 0)}
}

func (store *groupMetadataStore) MarshalJSON() ([]byte, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	return json.Marshal(store.entries)
}

func (store *groupMetadataStore) UnmarshalJSON(value []byte) error {
	entries, err := parseGroupMetadata(value)
	if err != nil {
		return err
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.entries = entries
	return nil
}

func (store *groupMetadataStore) copy() []GroupMetadata {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	return append([]GroupMetadata(nil), store.entries...)
}

func (store *groupMetadataStore) jsonString() string {
	value, err := store.MarshalJSON()
	if err != nil {
		return "[]"
	}
	return string(value)
}

func parseGroupMetadata(value []byte) ([]GroupMetadata, error) {
	var entries []GroupMetadata
	if err := json.Unmarshal(value, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, fmt.Errorf("group metadata must be an array")
	}
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Name) == "" {
			return nil, fmt.Errorf("group metadata name must not be empty")
		}
		if _, exists := seen[entry.Name]; exists {
			return nil, fmt.Errorf("group metadata name must be unique: %s", entry.Name)
		}
		if utf8.RuneCountInString(entry.Icon) > 128 {
			return nil, fmt.Errorf("group metadata icon must be at most 128 characters: %s", entry.Name)
		}
		if entry.Recommendation < 0 || entry.Recommendation > 5 {
			return nil, fmt.Errorf("group metadata recommendation must be between 0 and 5: %s", entry.Name)
		}
		scaledRecommendation := entry.Recommendation * 10
		if math.Abs(scaledRecommendation-math.Round(scaledRecommendation)) > 1e-9 {
			return nil, fmt.Errorf("group metadata recommendation must have at most one decimal place: %s", entry.Name)
		}
		seen[entry.Name] = struct{}{}
	}
	return entries, nil
}

func GetGroupMetadataCopy() []GroupMetadata {
	return GetGroupRatioSetting().GroupMetadata.copy()
}

func GroupMetadata2JSONString() string {
	return GetGroupRatioSetting().GroupMetadata.jsonString()
}

func UpdateGroupMetadataByJSONString(jsonStr string) error {
	return GetGroupRatioSetting().GroupMetadata.UnmarshalJSON([]byte(jsonStr))
}

func ValidateGroupMetadataJSONString(jsonStr string) error {
	_, err := parseGroupMetadata([]byte(jsonStr))
	return err
}
