/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getVideoStudioOptions, getVideoStudioTasks } from '../api'
import { VideoGenerationStudio } from '../index'

vi.mock('../api', () => ({
  estimateVideoStudioTask: vi.fn(),
  getVideoStudioOptions: vi.fn(),
  getVideoStudioTask: vi.fn(),
  getVideoStudioTasks: vi.fn(),
  submitVideoStudioTask: vi.fn(),
}))
vi.mock('../components/current-video-preview', () => ({
  VideoStudioCurrentPreview: () => <div />,
}))
vi.mock('../components/media-picker', () => ({
  VideoStudioMediaPicker: () => <div />,
}))
vi.mock('../components/paid-request-dialog', () => ({
  VideoStudioPaidRequestDialog: () => null,
}))
vi.mock('../components/task-history', () => ({
  VideoStudioTaskHistory: () => <div />,
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) =>
      ({
        'API Key': 'API 密钥',
        'Generation mode': '生成模式',
        'Text to video': '文生视频',
        'First / last frame': '首尾帧生成',
        'Multimodal references': '多模态参考',
        'Duration (seconds)': '时长（秒）',
        seconds: '秒',
      })[key] ?? key,
  }),
}))

beforeEach(() => {
  vi.mocked(getVideoStudioOptions).mockResolvedValue({
    tokens: [
      {
        id: 34,
        name: 'Seedance production',
        masked_key: 'sk-abcdxxxxefgh',
        group: 'ByteDance',
        unlimited_quota: true,
        available_models: ['doubao-seedance-2-0-260128'],
      },
    ],
    capabilities: {
      'doubao-seedance-2-0-260128': {
        max_duration: 15,
        max_images: 9,
        max_videos: 3,
        max_audio_files: 3,
        resolutions: ['720p'],
        ratios: ['16:9'],
        supports_web_search: false,
      },
    },
  })
  vi.mocked(getVideoStudioTasks).mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { data: url === '/api/assets/self' ? [] : {} },
  }))
})

function renderStudio() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <VideoGenerationStudio />
    </QueryClientProvider>
  )
}

test('shows the masked API key in both the trigger and option list', async () => {
  const user = userEvent.setup()
  renderStudio()

  const trigger = await screen.findByRole('combobox', { name: 'API 密钥' })
  await waitFor(() => expect(trigger).toHaveTextContent('sk-abcdxxxxefgh'))
  expect(trigger).not.toHaveTextContent('34')

  await user.click(trigger)
  expect(
    screen.getByRole('option', { name: 'sk-abcdxxxxefgh' })
  ).toBeInTheDocument()
})

test('shows Chinese generation modes and model-bounded duration choices', async () => {
  const user = userEvent.setup()
  renderStudio()

  const mode = await screen.findByRole('combobox', { name: '生成模式' })
  await waitFor(() => expect(mode).toHaveTextContent('文生视频'))
  expect(mode).not.toHaveTextContent('text')
  await user.click(mode)
  expect(screen.getByRole('option', { name: '文生视频' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: '首尾帧生成' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: '多模态参考' })).toBeInTheDocument()
  await user.keyboard('{Escape}')

  const duration = screen.getByRole('combobox', { name: '时长（秒）' })
  expect(duration).toHaveTextContent('6 秒')
  await user.click(duration)
  const options = screen.getByRole('listbox')
  expect(within(options).getByRole('option', { name: '4 秒' })).toBeVisible()
  expect(within(options).getByRole('option', { name: '15 秒' })).toBeVisible()
  expect(
    within(options).queryByRole('option', { name: '16 秒' })
  ).not.toBeInTheDocument()
})

test('keeps the prompt editor useful without dominating the form', async () => {
  renderStudio()

  const prompt = await screen.findByRole('textbox', { name: 'Prompt' })
  expect(prompt).toHaveClass('min-h-56')
  expect(prompt).toHaveClass('resize-y')
  expect(prompt).not.toHaveClass('min-h-[32rem]')
})
