package ratio_setting

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

type GroupMetadata struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
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
	return common.Marshal(store.entries)
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
	if err := common.Unmarshal(value, &entries); err != nil {
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
		seen[entry.Name] = struct{}{}
	}
	return entries, nil
}

func NormalizeGroupMetadataJSONString(jsonStr string) (string, error) {
	entries, err := parseGroupMetadata([]byte(jsonStr))
	if err != nil {
		return "", err
	}
	value, err := common.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(value), nil
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
