/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { fireEvent, render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import type { VideoStudioTask } from '../../types'
import { VideoStudioCurrentPreview } from '../current-video-preview'
import { VideoStudioTaskHistory } from '../task-history'

function task(overrides: Partial<VideoStudioTask['task']>): VideoStudioTask {
  return {
    task: {
      id: 1,
      user_id: 7,
      platform: '54',
      task_id: 'task-platform-001',
      action: 'videoGenerate',
      channel_id: 2,
      token_name: 'Seedance production',
      group: 'ByteDance',
      submit_time: 1_000,
      start_time: 1_012,
      finish_time: 1_072,
      status: 'SUCCESS',
      result_url: '/v1/videos/task-platform-001/content',
      properties: { origin_model_name: 'doubao-seedance-2-5-260628' },
      video_params: {
        resolution: '720p',
        ratio: '16:9',
        seconds: 15,
        fps: 24,
        width: 1280,
        height: 720,
        has_video: true,
        input_image_count: 3,
        input_video_count: 1,
        input_audio_count: 2,
      },
      billing: {
        state: 'settled',
        mode: 'seedance',
        model: 'doubao-seedance-2-5-260628',
        final_cost: 15.5569,
        group_ratio: 0.77,
        detail_available: true,
      },
      ...overrides,
    },
    request: {
      version: 1,
      model: 'doubao-seedance-2-5-260628',
      prompt: 'A slow dolly shot through a misty mountain valley.',
      resolution: '720p',
      ratio: '16:9',
      duration: 15,
      generate_audio: true,
      watermark: false,
      web_search: false,
    },
  }
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Video preview': 'Video preview',
    'Generating video': 'Generating video',
    'The task is being processed. The preview will appear automatically.':
      'The task is being processed. The preview will appear automatically.',
  })
})

describe('current Seedance video preview', () => {
  test('shows animated progress instead of compressing a list of task cards', () => {
    render(
      <VideoStudioCurrentPreview
        task={task({ status: 'IN_PROGRESS', progress: '42%' })}
        loading={false}
        onReuse={() => undefined}
      />
    )

    expect(
      screen.getByRole('heading', { name: 'Generating video' })
    ).toBeInTheDocument()
    expect(screen.getByText('42%')).toBeInTheDocument()
    expect(screen.getByTestId('video-generation-progress')).toHaveClass(
      'animate-pulse'
    )
    expect(screen.queryByRole('video')).not.toBeInTheDocument()
  })

  test('shows the completed current task as a video player', () => {
    const current = task({ status: 'SUCCESS' })
    const { container } = render(
      <VideoStudioCurrentPreview
        task={current}
        loading={false}
        onReuse={() => undefined}
      />
    )

    const video = container.querySelector('video')
    expect(video).not.toBeNull()
    expect(video).toHaveAttribute('src', '/v1/videos/task-platform-001/content')
    expect(screen.getByText('task-platform-001')).toBeInTheDocument()
  })
})

describe('Seedance generation history', () => {
  test('shows complete task facts and only the approved page-size choices', () => {
    render(
      <VideoStudioTaskHistory
        items={[task({})]}
        total={61}
        pageIndex={0}
        pageSize={20}
        loading={false}
        onPageChange={() => undefined}
        onPageSizeChange={() => undefined}
        onPreview={() => undefined}
        onReuse={() => undefined}
      />
    )

    const row = screen.getByRole('row', { name: /task-platform-001/ })
    expect(row).toHaveTextContent('A slow dolly shot')
    expect(row).toHaveTextContent('3 / 1 / 2')
    expect(row).toHaveTextContent('1280×720')
    expect(row).toHaveTextContent('720p')
    expect(row).toHaveTextContent('16:9')
    expect(row).toHaveTextContent('15s')
    expect(row).toHaveTextContent('72s')
    expect(row).toHaveTextContent('15.5569')
    expect(row).toHaveTextContent('Yes')
    expect(row).toHaveTextContent('No')

    const pageSize = screen.getByRole('combobox', { name: 'Rows per page' })
    fireEvent.click(pageSize)
    const listbox = screen.getByRole('listbox')
    expect(
      within(listbox).getByRole('option', { name: '10' })
    ).toBeInTheDocument()
    expect(
      within(listbox).getByRole('option', { name: '50' })
    ).toBeInTheDocument()
    expect(
      within(listbox).getByRole('option', { name: '100' })
    ).toBeInTheDocument()
    expect(
      within(listbox).queryByRole('option', { name: '30' })
    ).not.toBeInTheDocument()
  })

  test('previews and safely restores a history row without submitting it', () => {
    const current = task({})
    const onPreview = vi.fn()
    const onReuse = vi.fn()
    render(
      <VideoStudioTaskHistory
        items={[current]}
        total={1}
        pageIndex={0}
        pageSize={20}
        loading={false}
        onPageChange={() => undefined}
        onPageSizeChange={() => undefined}
        onPreview={onPreview}
        onReuse={onReuse}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Preview video' }))
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))

    expect(onPreview).toHaveBeenCalledExactlyOnceWith(current)
    expect(onReuse).toHaveBeenCalledExactlyOnceWith(current)
  })
})
