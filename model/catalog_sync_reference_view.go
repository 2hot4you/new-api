package model

import (
	"crypto/sha256"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// Only this private projection sees local reference identities. Raw task JSON
// is parsed locally and discarded, never incorporated into a plan or backup.
type catalogReferenceView struct {
	Routes    []string
	Routing   []string
	Ambiguous bool
	Models    []Model
	Tasks     []catalogTaskReference
	Scheduled bool
}

type catalogTaskReference struct {
	ID             int64
	Names          []string
	Plugin         string
	Platform       string
	Frozen         bool
	Unknown        bool
	FrozenIdentity string
}

func captureCatalogReferencesTx(tx *gorm.DB, pin *jsplugin.GenerationPin) (catalogReferenceView, error) {
	var view catalogReferenceView
	var channels []Channel
	if err := tx.Select("id", "type", "status", "models", "model_mapping").Order("id").Find(&channels).Error; err != nil {
		return view, err
	}
	byChannel := make(map[int][]string)
	enabled := make(map[int]bool)
	for _, channel := range channels {
		names := []string{}
		for name := range strings.SplitSeq(channel.Models, ",") {
			if strings.TrimSpace(name) == "" {
				continue
			}
			names = append(names, name, strings.TrimSpace(name))
		}
		mapping := channel.GetModelMapping()
		bad := false
		if mapping != "" {
			canonical, err := catalogmanifest.CanonicalJSON(mapping)
			var raw map[string]common.RawMessage
			if err != nil || !catalogReferenceJSONUnambiguous(mapping) || common.UnmarshalJsonStr(canonical, &raw) != nil || raw == nil {
				bad = true
			} else {
				parsed := make(map[string]string)
				for from, value := range raw {
					var to string
					if common.GetJsonType(value) != "string" || common.Unmarshal(value, &to) != nil || strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
						bad = true
						continue
					}
					parsed[from] = to
					names = append(names, from, to, strings.TrimSpace(from), strings.TrimSpace(to))
				}
				for from := range parsed {
					seen := make(map[string]bool)
					for next := from; parsed[next] != "" && parsed[next] != next; next = parsed[next] {
						if seen[next] {
							bad = true
							break
						}
						seen[next] = true
					}
				}
			}
		}
		byChannel[channel.Id] = names
		if channel.Status == common.ChannelStatusEnabled {
			encoded, err := common.Marshal([]any{channel.Id, channel.Type, channel.Models, channel.ModelMapping})
			if err != nil {
				return view, err
			}
			view.Routing = append(view.Routing, string(encoded))
			enabled[channel.Id] = true
			view.Routes = append(view.Routes, names...)
			view.Ambiguous = view.Ambiguous || bad
		}
	}
	var abilities []Ability
	if err := tx.Select("model", "channel_id", "group").Where("enabled = ?", true).Find(&abilities).Error; err != nil {
		return view, err
	}
	for _, ability := range abilities {
		encoded, err := common.Marshal([]any{ability.ChannelId, ability.Group, ability.Model})
		if err != nil {
			return view, err
		}
		view.Routing = append(view.Routing, string(encoded))
		view.Routes = append(view.Routes, ability.Model, strings.TrimSpace(ability.Model))
		if !enabled[ability.ChannelId] || strings.TrimSpace(ability.Model) == "" {
			view.Ambiguous = true
		}
	}
	slices.Sort(view.Routes)
	view.Routes = slices.Compact(view.Routes)
	if err := tx.Select("id", "model_name", "name_rule", "vendor_id", "billing_currency", "status").Order("id").Find(&view.Models).Error; err != nil {
		return view, err
	}
	for _, model := range view.Models {
		if model.NameRule < NameRuleExact || model.NameRule > NameRuleSuffix {
			view.Ambiguous = true
		}
	}
	var tasks []struct {
		ID                      int64
		ChannelID               int
		Platform, Action        string
		Properties, PrivateData *string
	}
	if err := tx.Model(&Task{}).Select("id", "channel_id", "platform", "action", "properties", "private_data").Where("status IS NULL OR status NOT IN ?", []string{TaskStatusSuccess, TaskStatusFailure}).Scan(&tasks).Error; err != nil {
		return view, err
	}
	for _, task := range tasks {
		ref := catalogTaskReference{ID: task.ID, Platform: task.Platform}
		var properties struct {
			OriginModelName   string `json:"origin_model_name"`
			UpstreamModelName string `json:"upstream_model_name"`
		}
		var private struct {
			BillingContext *TaskBillingContext `json:"billing_context"`
			Execution      *struct {
				TaskPlugin *struct {
					Key string `json:"key"`
				} `json:"task_plugin"`
			} `json:"execution"`
			VideoStudioRequest *struct {
				Model string `json:"model"`
			} `json:"video_studio_request"`
		}
		for i, value := range []*string{task.Properties, task.PrivateData} {
			if value == nil || *value == "" {
				continue
			}
			canonical, err := catalogmanifest.CanonicalJSON(*value)
			if err != nil || !catalogReferenceJSONUnambiguous(*value) || common.GetJsonType([]byte(canonical)) != "object" {
				ref.Unknown = true
				continue
			}
			paths := []string{"origin_model_name", "upstream_model_name"}
			if i == 1 {
				paths = []string{"billing_context.origin_model_name", "billing_context.tiered_snapshot.model_name", "video_studio_request.model", "execution.task_plugin.key"}
			}
			for _, path := range paths {
				value := gjson.Get(*value, path)
				if value.Exists() && (value.Type != gjson.String || strings.TrimSpace(value.String()) == "" || strings.TrimSpace(value.String()) != value.String()) {
					ref.Unknown = true
				}
			}
			if i == 0 {
				err = common.UnmarshalJsonStr(canonical, &properties)
			} else {
				err = common.UnmarshalJsonStr(canonical, &private)
			}
			if err != nil {
				ref.Unknown = true
			}
		}
		ref.Names = append(ref.Names, properties.OriginModelName, properties.UpstreamModelName)
		if private.VideoStudioRequest != nil {
			ref.Names = append(ref.Names, private.VideoStudioRequest.Model)
		}
		if private.Execution != nil && private.Execution.TaskPlugin != nil {
			ref.Plugin = private.Execution.TaskPlugin.Key
		}
		if context := private.BillingContext; context != nil {
			ref.Names = append(ref.Names, context.OriginModelName)
			if snapshot := context.TieredSnapshot; snapshot != nil {
				ref.Names = append(ref.Names, snapshot.ModelName)
				// Existing settlement consumes this frozen expression and currency.
				ref.Frozen = snapshot.TaskUsageBilling && snapshot.BillingMode == "tiered_expr" && snapshot.ModelName != "" && snapshot.ExprString != "" && snapshot.ExprHash == billingexpr.ExprHashString(snapshot.ExprString) && snapshot.QuotaPerUnit > 0 && snapshot.GroupRatio >= 0 && (snapshot.SourceCurrency == "USD" || snapshot.SourceCurrency == "CNY" && snapshot.CNYPerUSD > 0)
				if snapshot.ModelName != context.OriginModelName {
					ref.Unknown = true
					ref.Frozen = false
				}
			}
			// Dedicated settlement dispatch precedes the generic tiered path.
			if task.Platform == fmt.Sprint(constant.ChannelTypeStarAI) && context.TieredSnapshot == nil {
				// StarAI's built-in BaseBilling returns no adjustment; the
				// generic token recalculation uses this saved positive anchor
				// and copied ratios. Live group fallback is not catalog data.
				ref.Frozen = !context.PerCallBilling && context.ModelRatio > 0 && !math.IsInf(context.ModelRatio, 0) && context.OriginModelName != ""
				for _, ratio := range context.OtherRatios {
					if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
						ref.Frozen = false
					}
				}
			}
			if task.Platform == fmt.Sprint(constant.ChannelTypeByteDanceSeedance) {
				ref.Frozen = context.TieredSnapshot == nil && !context.PerCallBilling && context.ModelRatio > 0 && !math.IsInf(context.ModelRatio, 0) && (context.GroupRatioCaptured || context.GroupRatio > 0) && context.GroupRatio >= 0 && !math.IsInf(context.GroupRatio, 0) && context.OriginModelName != ""
				for _, ratio := range context.OtherRatios {
					if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
						ref.Frozen = false
					}
				}
			}
			// Generic tiered settlement runs before the Grok adaptor. A
			// nonnil but malformed tiered snapshot must not fall back to V2.
			if task.Platform == fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC) && context.TieredSnapshot == nil {
				ref.Frozen = false
				if snapshot := context.GrokVideoBilling; snapshot != nil {
					ref.Names = append(ref.Names, snapshot.Model, snapshot.RequestedModel, snapshot.BilledModel)
					ref.Frozen = snapshot.Version == 2 && context.OriginModelName != "" && snapshot.Model != "" && snapshot.EstimatedDurationSeconds > 0 && snapshot.EstimatedResolution != "" && (snapshot.SourceCurrency == "USD" || snapshot.SourceCurrency == "CNY" && snapshot.CNYPerUSD > 0)
					ref.Frozen = ref.Frozen && catalogGrokSubmissionShape(snapshot)
					for _, number := range []float64{snapshot.OutputUnitPrice, snapshot.ImageInputUnitPrice, snapshot.VideoInputUnitPrice, context.GroupRatio, snapshot.CNYPerUSD} {
						if number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
							ref.Frozen = false
						}
					}
				}
			}
			if platform, err := strconv.Atoi(task.Platform); context.TieredSnapshot == nil && err == nil && (platform == constant.ChannelTypeStarAI || platform == constant.ChannelTypeByteDanceSeedance || platform == constant.ChannelTypeMoliiGrokAIGC) {
				_, overridden := pin.Generation.GetByChannelType(platform)
				if overridden || ref.Plugin != "" {
					ref.Frozen = false
				}
			}
			if properties.OriginModelName != "" && context.OriginModelName != "" && properties.OriginModelName != context.OriginModelName {
				ref.Unknown = true
				ref.Frozen = false
			}
			// Bind only selected frozen price facts, excluding task usage,
			// request body, prompt, credentials and output/private payloads.
			var expressionFacts any
			if snapshot := context.TieredSnapshot; snapshot != nil {
				expressionFacts = []any{snapshot.ModelName, snapshot.ExprHash, snapshot.ExprVersion, snapshot.QuotaPerUnit, snapshot.SourceCurrency, snapshot.CNYPerUSD, snapshot.GroupRatio}
			}
			var specialFacts any
			if snapshot := context.GrokVideoBilling; snapshot != nil {
				specialFacts = []any{snapshot.Version, snapshot.Model, snapshot.RequestedModel, snapshot.BilledModel, snapshot.OutputUnitPrice, snapshot.ImageInputUnitPrice, snapshot.VideoInputUnitPrice, snapshot.SourceCurrency, snapshot.CNYPerUSD}
			}
			encoded, err := common.Marshal([]any{expressionFacts, specialFacts, context.ModelRatio, context.OtherRatios, context.GroupRatio, context.GroupRatioCaptured})
			if err != nil {
				ref.Unknown = true
				ref.Frozen = false
			} else {
				ref.FrozenIdentity = fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
			}
		}
		ref.Names = slices.DeleteFunc(ref.Names, func(name string) bool { return strings.TrimSpace(name) == "" })
		if len(ref.Names) == 0 {
			ref.Names = append(ref.Names, byChannel[task.ChannelID]...)
			ref.Unknown = true
		}
		if task.Platform != "" && task.Action != "" {
			ref.Names = append(ref.Names, strings.ToLower(task.Platform+"_"+task.Action))
		}
		slices.Sort(ref.Names)
		ref.Names = slices.Compact(ref.Names)
		view.Tasks = append(view.Tasks, ref)
	}
	var legacy []struct{ ChannelID, BillingChannelID int }
	if err := tx.Model(&Midjourney{}).Select("channel_id", "billing_channel_id").Where("status IS NULL OR status NOT IN ?", []string{TaskStatusSuccess, TaskStatusFailure}).Scan(&legacy).Error; err != nil {
		return view, err
	}
	for _, task := range legacy {
		names := append(slices.Clone(byChannel[task.ChannelID]), byChannel[task.BillingChannelID]...)
		view.Tasks = append(view.Tasks, catalogTaskReference{Names: names, Platform: string(constant.TaskPlatformMidjourney), Unknown: true})
	}
	var scheduled []struct{ Type string }
	if err := tx.Model(&SystemTask{}).Select("type").Where("status IS NULL OR status NOT IN ?", []string{string(SystemTaskStatusSucceeded), string(SystemTaskStatusFailed)}).Scan(&scheduled).Error; err != nil {
		return view, err
	}
	for _, task := range scheduled {
		switch task.Type {
		case SystemTaskTypeLogCleanup, SystemTaskTypeStarAIResultCleanup, SystemTaskTypeAsyncTaskPoll, SystemTaskTypeMidjourneyPoll, SystemTaskTypeAsyncTaskBillingReconcile:
		default:
			view.Scheduled = true
		}
	}
	return view, nil
}

// Existing Grok submission/settlement dispatch shape, not a price evaluator.
// Incomplete historical records cannot become frozen merely by claiming V2.
func catalogGrokSubmissionShape(snapshot *GrokVideoBillingSnapshot) bool {
	model := snapshot.BilledModel
	if model == "" {
		model = snapshot.Model
	}
	if model != "grok-imagine-video" && model != "grok-imagine-video-1.5" {
		return false
	}
	switch snapshot.Operation {
	case "text_to_video":
		return snapshot.InputType == "text"
	case "image_to_video":
		return snapshot.InputType == "image" && snapshot.InputImageCount == 1
	case "reference_to_video":
		return snapshot.InputType == "image" && snapshot.InputImageCount >= 1 && snapshot.InputImageCount <= 7
	case "video_edit", "video_extension":
		resolution := strings.ToLower(strings.TrimSpace(snapshot.RequestedResolution))
		valid := snapshot.InputType == "video" && snapshot.VideoInputBilledSeconds > 0 && snapshot.RequestedDurationSeconds > 0 && (resolution == "480p" || resolution == "720p") && snapshot.ResolutionSource == "input_probe_v1"
		if snapshot.Operation == "video_extension" {
			valid = valid && snapshot.VideoInputBilledSeconds >= 2 && snapshot.VideoInputBilledSeconds <= 15
		}
		return valid
	}
	return false
}

func catalogReferenceMatches(name, candidate string, rule *Model) bool {
	if rule != nil && rule.MatchesName(name) {
		return true
	}
	return name == candidate || ratio_setting.RoutingMatchModelName(name) == ratio_setting.RoutingMatchModelName(candidate) || ratio_setting.FormatMatchingModelName(name) == ratio_setting.FormatMatchingModelName(candidate)
}

// Returns only public-safe reason codes and a digest of relevant minimal facts.
// Terminal/progress/private-result churn and unrelated tasks do not stale plans.
func catalogReferenceDecision(view catalogReferenceView, plan catalogmanifest.Plan, base catalogmanifest.Baseline) (map[string]string, string, error) {
	blocked := make(map[string]string)
	facts := []string{}
	for _, change := range plan.Changes {
		if change.Action == "preserve" || change.Action == "unchanged" || change.Action == "adopt" {
			continue
		}
		if change.Before != nil && change.After != nil && change.Before.Value == change.After.Value {
			continue
		}
		id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: change.Kind, Key: change.Key})
		deleting := change.After == nil && change.Before != nil
		name := change.Key
		option := ""
		metadataUnsafe := false
		var rule *Model
		var nextRule *Model
		if change.Kind == catalogmanifest.KindVendor {
			if deleting {
				for _, model := range view.Models {
					// Vendor identity is resolved against the full target entry below.
					for _, item := range plan.Changes {
						if item.Kind == catalogmanifest.KindModel && item.Before != nil {
							var value catalogmanifest.ModelValue
							if err := common.UnmarshalJsonStr(item.Before.Value, &value); err != nil {
								return nil, "", err
							}
							if item.Key == model.ModelName && value.Vendor == name {
								blocked[id] = "vendor_still_referenced"
							}
						}
					}
				}
			}
			continue
		}
		if change.Kind == catalogmanifest.KindModel {
			var old, next catalogmanifest.ModelValue
			if change.Before != nil {
				if err := common.UnmarshalJsonStr(change.Before.Value, &old); err != nil {
					return nil, "", err
				}
			}
			if change.After != nil {
				if err := common.UnmarshalJsonStr(change.After.Value, &next); err != nil {
					return nil, "", err
				}
			}
			metadataUnsafe = deleting || change.Before == nil || old.BillingCurrency != next.BillingCurrency || old.NameRule != next.NameRule
			if !metadataUnsafe {
				continue
			}
			for _, model := range view.Models {
				if model.ModelName == name {
					copy := model
					rule = &copy
					break
				}
			}
			if rule == nil || rule.NameRule == NameRuleExact {
				rule = &Model{ModelName: name, NameRule: next.NameRule}
			}
			if change.After != nil {
				nextRule = &Model{ModelName: name, NameRule: next.NameRule}
			}
		} else {
			key, err := catalogmanifest.DecodePriceKey(change.Key)
			if err != nil {
				return nil, "", err
			}
			name = key.Model
			option = key.Option
			if deleting {
				blocked[id] = "price_fallback_unproven"
			}
		}
		if (deleting || metadataUnsafe) && (view.Ambiguous || view.Scheduled) {
			blocked[id] = "reference_identity_unproven"
		}
		if deleting || metadataUnsafe && change.Before != nil {
			for _, route := range view.Routes {
				if catalogReferenceMatches(route, name, rule) || nextRule != nil && catalogReferenceMatches(route, name, nextRule) {
					blocked[id] = "enabled_route_reference"
					facts = append(facts, id+"/route/"+route)
				}
			}
		}
		for _, task := range view.Tasks {
			relevant := task.Unknown || len(task.Names) == 0
			if name == "" {
				// These task dispatch families are the actual special-price
				// consumers. Tool fees and currently unused task factors have
				// no durable async settlement reader in this checkout.
				// Model uncertainty cannot escape a known consumer family.
				// Unclassified families retain conservative identity handling.
				knownFamily := false
				switch task.Platform {
				case string(constant.TaskPlatformMidjourney), string(constant.TaskPlatformSuno), fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC), fmt.Sprint(constant.ChannelTypeStarAI), fmt.Sprint(constant.ChannelTypeByteDanceSeedance):
					knownFamily = true
				}
				switch {
				case strings.HasPrefix(option, "molii_grok_price."):
					relevant = !knownFamily && relevant || task.Platform == fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC)
				case strings.HasPrefix(option, "starai_video_price."):
					relevant = !knownFamily && relevant || task.Platform == fmt.Sprint(constant.ChannelTypeStarAI) || task.Platform == fmt.Sprint(constant.ChannelTypeByteDanceSeedance)
				default:
					relevant = false
				}
			}
			for _, model := range task.Names {
				if name != "" {
					relevant = relevant || catalogReferenceMatches(model, name, rule) || nextRule != nil && catalogReferenceMatches(model, name, nextRule)
				}
			}
			if !relevant {
				continue
			}
			encoded, err := common.Marshal(task)
			if err != nil {
				return nil, "", err
			}
			facts = append(facts, id+"/task/"+string(encoded))
			if deleting || !task.Frozen || task.Unknown {
				blocked[id] = "unfinished_task_reference"
			}
		}
	}
	slices.Sort(facts)
	facts = slices.Compact(facts)
	slices.Sort(view.Routing)
	encoded, err := common.Marshal([]any{view.Routing, view.Routes, view.Ambiguous, view.Scheduled, facts, base})
	if err != nil {
		return nil, "", err
	}
	return blocked, fmt.Sprintf("sha256:%x", sha256.Sum256(encoded)), nil
}

// gjson traversal detects duplicate members before typed decoding can discard
// an identity. No task payload is emitted or hashed by this local parser.
func catalogReferenceJSONUnambiguous(raw string) bool {
	if !gjson.Valid(raw) {
		return false
	}
	var visit func(gjson.Result) bool
	visit = func(value gjson.Result) bool {
		valid := true
		seen := make(map[string]bool)
		if value.IsObject() || value.IsArray() {
			value.ForEach(func(key, child gjson.Result) bool {
				if value.IsObject() {
					if seen[key.String()] {
						valid = false
						return false
					}
					seen[key.String()] = true
				}
				if !visit(child) {
					valid = false
					return false
				}
				return true
			})
		}
		return valid
	}
	return visit(gjson.Parse(raw))
}
