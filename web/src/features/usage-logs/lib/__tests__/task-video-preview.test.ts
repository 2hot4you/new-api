/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { TASK_PLATFORMS, TASK_STATUS } from '../../constants'
import { taskActionMapper, taskPlatformMapper } from '../mappers'
import {
  canPreviewVideoTask,
  isGeneratedVideoTask,
} from '../task-video-preview'

describe('task video preview', () => {
  test('maps Grok video edit tasks to their user-facing labels', () => {
    assert.equal(taskActionMapper.getLabel('video_edit'), 'Video Editing')
    assert.equal(taskPlatformMapper.getLabel('62'), 'Grok')
  })

  test('allows successful Grok video tasks with a signed result URL to preview', () => {
    const task = {
      platform: TASK_PLATFORMS.MOLII_GROK,
      status: TASK_STATUS.SUCCESS,
      result_url:
        '/v1/videos/task_rR90qPDjBcnNcJP7cOGvcu2NhADWecJw/content?expires=1&signature=test',
    }

    assert.equal(isGeneratedVideoTask(task), true)
    assert.equal(canPreviewVideoTask(task), true)
  })

  test('does not expose a preview action without a usable result URL', () => {
    assert.equal(
      canPreviewVideoTask({
        platform: TASK_PLATFORMS.MOLII_GROK,
        status: TASK_STATUS.SUCCESS,
        result_url: '   ',
      }),
      false
    )
  })

  test('keeps unfinished Grok tasks in the progress branch', () => {
    assert.equal(
      isGeneratedVideoTask({
        platform: TASK_PLATFORMS.MOLII_GROK,
        status: TASK_STATUS.IN_PROGRESS,
      }),
      false
    )
  })

  test('preserves Seedance generated video preview behavior', () => {
    assert.equal(
      canPreviewVideoTask({
        platform: TASK_PLATFORMS.STARAI,
        status: TASK_STATUS.SUCCESS,
        result_url: '/v1/videos/seedance-task/content?signature=test',
      }),
      true
    )
  })
})
