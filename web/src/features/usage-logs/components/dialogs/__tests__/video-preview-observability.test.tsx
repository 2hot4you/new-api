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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { TaskLog } from '../../../types'
import { VideoPreviewDialog } from '../video-preview-dialog'

const baseVideoParams = {
  resolution: '720p',
  ratio: '16:9',
  seconds: 15,
  has_video: true,
} satisfies NonNullable<TaskLog['video_params']>

const baseLog: TaskLog = {
  id: 1,
  user_id: 7,
  platform: '61',
  task_id: 'task_preview_counts',
  action: 'TEXT_TO_VIDEO',
  channel_id: 2,
  group: 'ByteDance',
  submit_time: 1_000,
  finish_time: 1_100,
  status: 'SUCCESS',
  result_url: '/v1/videos/task_preview_counts/content?signature=safe',
  video_params: baseVideoParams,
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    Images: 'Images',
    Videos: 'Videos',
    Audio: 'Audio',
    'Not recorded': 'Not recorded',
  })
})

describe('Seedance preview input media summary', () => {
  test('shows exact image, video and audio counts', () => {
    render(
      <VideoPreviewDialog
        open
        onOpenChange={() => undefined}
        log={{
          ...baseLog,
          video_params: {
            ...baseVideoParams,
            input_image_count: 11,
            input_video_count: 2,
            input_audio_count: 3,
          },
        }}
      />
    )

    expect(screen.getByText('Images').nextElementSibling).toHaveTextContent(
      '11'
    )
    expect(screen.getByText('Videos').nextElementSibling).toHaveTextContent('2')
    expect(screen.getByText('Audio').nextElementSibling).toHaveTextContent('3')
  })

  test('labels historical missing counts as not recorded', () => {
    render(
      <VideoPreviewDialog open onOpenChange={() => undefined} log={baseLog} />
    )

    expect(screen.getAllByText('Not recorded')).toHaveLength(3)
  })
})
