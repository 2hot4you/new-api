package controller

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaginateSeedanceTasksDoesNotStopAtTheFirstThousandPlatformTasks(t *testing.T) {
	tasks := make([]*model.Task, 0, 1006)
	for index := 0; index < 1005; index++ {
		tasks = append(tasks, &model.Task{
			ID:         int64(2000 - index),
			TaskID:     fmt.Sprintf("unrelated-%d", index),
			Properties: model.Properties{OriginModelName: "unrelated-video-model"},
		})
	}
	tasks = append(tasks, &model.Task{
		ID:         1,
		TaskID:     "seedance-after-1000",
		Properties: model.Properties{OriginModelName: "doubao-seedance-2-5-260628"},
	})

	fetch := func(offset, limit int) ([]*model.Task, error) {
		if offset >= len(tasks) {
			return []*model.Task{}, nil
		}
		end := min(offset+limit, len(tasks))
		return tasks[offset:end], nil
	}

	page, total, err := paginateSeedanceTasks(0, 20, fetch)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, page, 1)
	assert.Equal(t, "seedance-after-1000", page[0].TaskID)
}

func TestPaginateSeedanceTasksReturnsTheRequestedFilteredPageAndExactTotal(t *testing.T) {
	tasks := []*model.Task{
		{TaskID: "seedance-1", Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-260128"}},
		{TaskID: "other", Properties: model.Properties{OriginModelName: "other-model"}},
		{TaskID: "seedance-2", Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-fast-260128"}},
		{TaskID: "seedance-3", Properties: model.Properties{OriginModelName: "doubao-seedance-2-0-mini-260615"}},
	}
	fetch := func(offset, limit int) ([]*model.Task, error) {
		if offset >= len(tasks) {
			return []*model.Task{}, nil
		}
		end := min(offset+limit, len(tasks))
		return tasks[offset:end], nil
	}

	page, total, err := paginateSeedanceTasks(1, 1, fetch)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, page, 1)
	assert.Equal(t, "seedance-2", page[0].TaskID)
}
