package model

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/constant"
)

const StarAIVideoSlowTaskSeconds int64 = 10 * 60

type VideoPerformanceAggregate struct {
	SubmittedCount         int64   `json:"submitted_count"`
	SuccessCount           int64   `json:"success_count"`
	FailureCount           int64   `json:"failure_count"`
	PendingCount           int64   `json:"pending_count"`
	SuccessRate            float64 `json:"success_rate"`
	AverageDurationSeconds int64   `json:"average_duration_seconds"`
	P50DurationSeconds     int64   `json:"p50_duration_seconds"`
	P95DurationSeconds     int64   `json:"p95_duration_seconds"`
	SlowTaskCount          int64   `json:"slow_task_count"`
	durations              []int64
}

type VideoPerformanceGroup struct {
	Group string `json:"group"`
	VideoPerformanceAggregate
}

type VideoPerformancePoint struct {
	Timestamp int64 `json:"ts"`
	VideoPerformanceAggregate
}

type VideoPerformanceResult struct {
	ModelName string                    `json:"model_name"`
	Hours     int                       `json:"hours"`
	Summary   VideoPerformanceAggregate `json:"summary"`
	Groups    []VideoPerformanceGroup   `json:"groups"`
	Series    []VideoPerformancePoint   `json:"series"`
}

func (aggregate *VideoPerformanceAggregate) addTask(task *Task) {
	aggregate.SubmittedCount++
	switch task.Status {
	case TaskStatusSuccess:
		aggregate.SuccessCount++
		if task.FinishTime > task.SubmitTime {
			duration := task.FinishTime - task.SubmitTime
			aggregate.durations = append(aggregate.durations, duration)
			if duration > StarAIVideoSlowTaskSeconds {
				aggregate.SlowTaskCount++
			}
		}
	case TaskStatusFailure:
		aggregate.FailureCount++
	default:
		aggregate.PendingCount++
	}
}

func (aggregate *VideoPerformanceAggregate) finish() {
	terminalCount := aggregate.SuccessCount + aggregate.FailureCount
	if terminalCount > 0 {
		aggregate.SuccessRate = math.Round(float64(aggregate.SuccessCount)/float64(terminalCount)*10000) / 100
	}
	if len(aggregate.durations) == 0 {
		aggregate.durations = nil
		return
	}

	sort.Slice(aggregate.durations, func(i, j int) bool {
		return aggregate.durations[i] < aggregate.durations[j]
	})
	var total int64
	for _, duration := range aggregate.durations {
		total += duration
	}
	aggregate.AverageDurationSeconds = total / int64(len(aggregate.durations))
	aggregate.P50DurationSeconds = percentileDuration(aggregate.durations, 0.50)
	aggregate.P95DurationSeconds = percentileDuration(aggregate.durations, 0.95)
	aggregate.durations = nil
}

func percentileDuration(sortedDurations []int64, percentile float64) int64 {
	if len(sortedDurations) == 0 {
		return 0
	}
	index := int(math.Ceil(percentile*float64(len(sortedDurations)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sortedDurations) {
		index = len(sortedDurations) - 1
	}
	return sortedDurations[index]
}

func videoPerformanceTaskModel(task *Task) string {
	if task.PrivateData.BillingContext != nil && task.PrivateData.BillingContext.OriginModelName != "" {
		return task.PrivateData.BillingContext.OriginModelName
	}
	return task.Properties.OriginModelName
}

func GetStarAIVideoPerformance(modelName string, hours int) (VideoPerformanceResult, error) {
	result := VideoPerformanceResult{ModelName: modelName, Hours: hours}
	if hours <= 0 {
		return result, nil
	}

	endTimestamp := time.Now().Unix()
	startTimestamp := endTimestamp - int64(hours)*3600
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))
	var tasks []*Task
	err := DB.Where("platform = ?", platform).
		Where("submit_time >= ? AND submit_time <= ?", startTimestamp, endTimestamp).
		Find(&tasks).Error
	if err != nil {
		return result, err
	}

	groups := make(map[string]*VideoPerformanceAggregate)
	series := make(map[int64]*VideoPerformanceAggregate)
	for _, task := range tasks {
		if videoPerformanceTaskModel(task) != modelName {
			continue
		}
		group := task.Group
		if group == "" {
			group = "default"
		}
		bucketTimestamp := task.SubmitTime - task.SubmitTime%3600
		if groups[group] == nil {
			groups[group] = &VideoPerformanceAggregate{}
		}
		if series[bucketTimestamp] == nil {
			series[bucketTimestamp] = &VideoPerformanceAggregate{}
		}
		result.Summary.addTask(task)
		groups[group].addTask(task)
		series[bucketTimestamp].addTask(task)
	}

	result.Summary.finish()
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)
	for _, group := range groupNames {
		groups[group].finish()
		result.Groups = append(result.Groups, VideoPerformanceGroup{
			Group:                     group,
			VideoPerformanceAggregate: *groups[group],
		})
	}

	timestamps := make([]int64, 0, len(series))
	for timestamp := range series {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	for _, timestamp := range timestamps {
		series[timestamp].finish()
		result.Series = append(result.Series, VideoPerformancePoint{
			Timestamp:                 timestamp,
			VideoPerformanceAggregate: *series[timestamp],
		})
	}
	return result, nil
}
