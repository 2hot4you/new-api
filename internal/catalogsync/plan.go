package catalogsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ChangeAction string

const (
	ChangeCreate ChangeAction = "create"
	ChangeUpdate ChangeAction = "update"
	ChangeDelete ChangeAction = "delete"
)

type VendorChange struct {
	Action ChangeAction  `json:"action"`
	Name   string        `json:"name"`
	Before *VendorRecord `json:"before,omitempty"`
	After  VendorRecord  `json:"after"`
}

type ModelChange struct {
	Action    ChangeAction `json:"action"`
	ModelName string       `json:"model_name"`
	Before    *ModelRecord `json:"before,omitempty"`
	After     ModelRecord  `json:"after"`
}

type OptionChange struct {
	Action ChangeAction `json:"action"`
	Key    string       `json:"key"`
	Before *string      `json:"before,omitempty"`
	After  *string      `json:"after,omitempty"`
}

type PlanSummary struct {
	VendorsCreated int `json:"vendors_created"`
	VendorsUpdated int `json:"vendors_updated"`
	VendorsDeleted int `json:"vendors_deleted"`
	ModelsCreated  int `json:"models_created"`
	ModelsUpdated  int `json:"models_updated"`
	ModelsDeleted  int `json:"models_deleted"`
	OptionsCreated int `json:"options_created"`
	OptionsUpdated int `json:"options_updated"`
	OptionsDeleted int `json:"options_deleted"`
}

type Plan struct {
	SchemaVersion      int            `json:"schema_version"`
	GeneratedAt        string         `json:"generated_at"`
	SnapshotDigest     string         `json:"snapshot_digest"`
	ConfirmationDigest string         `json:"confirmation_digest"`
	Summary            PlanSummary    `json:"summary"`
	Vendors            []VendorChange `json:"vendors"`
	Models             []ModelChange  `json:"models"`
	Options            []OptionChange `json:"options"`
}

func BuildPlan(ctx context.Context, db *gorm.DB, snapshot Snapshot) (Plan, error) {
	if err := snapshot.NormalizeAndValidate(); err != nil {
		return Plan{}, err
	}
	snapshotDigest, err := snapshot.contentDigest()
	if err != nil {
		return Plan{}, err
	}
	target, err := Export(ctx, db)
	if err != nil {
		return Plan{}, fmt.Errorf("load target catalog: %w", err)
	}

	plan := Plan{
		SchemaVersion:  SnapshotSchemaVersion,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SnapshotDigest: snapshotDigest,
		Vendors:        make([]VendorChange, 0),
		Models:         make([]ModelChange, 0),
		Options:        make([]OptionChange, 0),
	}

	targetVendors := make(map[string]VendorRecord, len(target.Vendors))
	for _, vendor := range target.Vendors {
		targetVendors[vendor.Name] = vendor
	}
	for _, desired := range snapshot.Vendors {
		current, exists := targetVendors[desired.Name]
		if !exists {
			plan.Vendors = append(plan.Vendors, VendorChange{Action: ChangeCreate, Name: desired.Name, After: desired})
			plan.Summary.VendorsCreated++
			continue
		}
		if recordsEqual(current, desired) {
			continue
		}
		before := current
		plan.Vendors = append(plan.Vendors, VendorChange{Action: ChangeUpdate, Name: desired.Name, Before: &before, After: desired})
		plan.Summary.VendorsUpdated++
	}

	targetModels := make(map[string]ModelRecord, len(target.Models))
	for _, entry := range target.Models {
		targetModels[entry.ModelName] = entry
	}
	for _, desired := range snapshot.Models {
		current, exists := targetModels[desired.ModelName]
		if !exists {
			plan.Models = append(plan.Models, ModelChange{Action: ChangeCreate, ModelName: desired.ModelName, After: desired})
			plan.Summary.ModelsCreated++
			continue
		}
		if recordsEqual(current, desired) {
			continue
		}
		before := current
		plan.Models = append(plan.Models, ModelChange{Action: ChangeUpdate, ModelName: desired.ModelName, Before: &before, After: desired})
		plan.Summary.ModelsUpdated++
	}

	managedModels := make(map[string]struct{}, len(snapshot.Models))
	for _, entry := range snapshot.Models {
		managedModels[entry.ModelName] = struct{}{}
	}
	for _, key := range modelPricingOptionKeys {
		desired, err := reconcileModelPricingOption(snapshot.Options[key], target.Options[key], managedModels)
		if err != nil {
			return Plan{}, fmt.Errorf("reconcile %s: %w", key, err)
		}
		before := target.Options[key]
		appendOptionChange(&plan, key, &before, &desired)
	}
	for key, desired := range snapshot.Options {
		if isModelPricingOption(key) {
			continue
		}
		var before *string
		if current, exists := target.Options[key]; exists {
			before = &current
		}
		after := desired
		appendOptionChange(&plan, key, before, &after)
	}
	for key, current := range target.Options {
		if isModelPricingOption(key) {
			continue
		}
		if _, exists := snapshot.Options[key]; !exists {
			before := current
			appendOptionChange(&plan, key, &before, nil)
		}
	}

	sort.Slice(plan.Vendors, func(i, j int) bool { return plan.Vendors[i].Name < plan.Vendors[j].Name })
	sort.Slice(plan.Models, func(i, j int) bool { return plan.Models[i].ModelName < plan.Models[j].ModelName })
	sort.Slice(plan.Options, func(i, j int) bool { return plan.Options[i].Key < plan.Options[j].Key })
	plan.ConfirmationDigest, err = plan.confirmationDigest()
	if err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func recordsEqual(left, right any) bool {
	leftJSON, leftErr := common.Marshal(left)
	rightJSON, rightErr := common.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func isModelPricingOption(key string) bool {
	for _, candidate := range modelPricingOptionKeys {
		if key == candidate {
			return true
		}
	}
	return false
}

func reconcileModelPricingOption(source, target string, managedModels map[string]struct{}) (string, error) {
	var sourceValues map[string]any
	if err := common.UnmarshalJsonStr(source, &sourceValues); err != nil {
		return "", fmt.Errorf("source must be a JSON object: %w", err)
	}
	if sourceValues == nil {
		return "", fmt.Errorf("source must be a JSON object")
	}
	var targetValues map[string]any
	if err := common.UnmarshalJsonStr(target, &targetValues); err != nil {
		return "", fmt.Errorf("target must be a JSON object: %w", err)
	}
	if targetValues == nil {
		targetValues = make(map[string]any)
	}
	for name := range managedModels {
		delete(targetValues, name)
		if value, exists := sourceValues[name]; exists {
			targetValues[name] = value
		}
	}
	encoded, err := common.Marshal(targetValues)
	if err != nil {
		return "", fmt.Errorf("encode reconciled option: %w", err)
	}
	return string(encoded), nil
}

func appendOptionChange(plan *Plan, key string, before, after *string) {
	if after == nil {
		if before == nil {
			return
		}
		plan.Options = append(plan.Options, OptionChange{Action: ChangeDelete, Key: key, Before: before})
		plan.Summary.OptionsDeleted++
		return
	}
	if before != nil && optionValuesEqual(key, *before, *after) {
		return
	}
	if before == nil {
		plan.Options = append(plan.Options, OptionChange{Action: ChangeCreate, Key: key, After: after})
		plan.Summary.OptionsCreated++
		return
	}
	plan.Options = append(plan.Options, OptionChange{Action: ChangeUpdate, Key: key, Before: before, After: after})
	plan.Summary.OptionsUpdated++
}

func optionValuesEqual(key, left, right string) bool {
	if !isModelPricingOption(key) {
		return left == right
	}
	var leftValue any
	var rightValue any
	if common.UnmarshalJsonStr(left, &leftValue) != nil || common.UnmarshalJsonStr(right, &rightValue) != nil {
		return left == right
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func (plan Plan) confirmationDigest() (string, error) {
	plan.GeneratedAt = ""
	plan.ConfirmationDigest = ""
	encoded, err := common.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode plan digest: %w", err)
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(encoded)), nil
}
