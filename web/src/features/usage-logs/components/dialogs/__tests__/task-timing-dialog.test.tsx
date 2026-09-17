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
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { TaskLog } from '../../../types'
import { TaskDurationCell } from '../../columns/task-logs-columns'

const log: TaskLog = {
  id: 1,
  user_id: 7,
  platform: '61',
  task_id: 'task_timing_public',
  action: 'TEXT_TO_VIDEO',
  channel_id: 2,
  group: 'ByteDance',
  submit_time: 1_000,
  start_time: 1_133,
  finish_time: 2_107,
  status: 'SUCCESS',
  admin_info: {
    timing: {
      platform_submitted_at: 1_000,
      upstream_submitted_at: 1_002,
      upstream_started_at: 1_131,
      upstream_finished_at: 2_105,
      platform_first_in_progress_at: 1_133,
      platform_finished_observed_at: 2_107,
      submission_seconds: 2,
      upstream_queue_seconds: 129,
      upstream_generation_seconds: 974,
      upstream_total_seconds: 1_103,
      start_detection_delay_seconds: 2,
      finish_detection_delay_seconds: 2,
      platform_total_seconds: 1_107,
    },
  },
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Task Timing': 'Task Timing',
    'Upstream Queue': 'Upstream Queue',
    'Upstream Generation': 'Upstream Generation',
    'Completion Polling Delay': 'Completion Polling Delay',
  })
})

describe('Seedance task timing', () => {
  test('lets an administrator open the full timing breakdown', () => {
    render(<TaskDurationCell log={log} isAdmin />)

    fireEvent.click(screen.getByRole('button', { name: /1107\.0s/ }))

    expect(screen.getByText('Task Timing')).toBeInTheDocument()
    expect(screen.getByText('Upstream Queue')).toBeInTheDocument()
    expect(screen.getByText('129s')).toBeInTheDocument()
    expect(screen.getByText('Upstream Generation')).toBeInTheDocument()
    expect(screen.getByText('974s')).toBeInTheDocument()
    expect(screen.getByText('Completion Polling Delay')).toBeInTheDocument()
  })

  test('keeps the total duration non-interactive for ordinary users', () => {
    render(<TaskDurationCell log={log} isAdmin={false} />)

    expect(screen.getByText('1107.0s')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /1107\.0s/ })).toBeNull()
    expect(screen.queryByText('Task Timing')).toBeNull()
  })
})
