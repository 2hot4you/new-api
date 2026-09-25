/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  estimateVideoStudioTask,
  getVideoStudioTask,
  getVideoStudioTasks,
} from '../api'

afterEach(() => vi.restoreAllMocks())

test('requests the selected history page and preserves the page envelope', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { items: [], total: 61, page: 3, page_size: 50 },
    },
  })

  await expect(getVideoStudioTasks({ page: 3, pageSize: 50 })).resolves.toEqual(
    { items: [], total: 61, page: 3, page_size: 50 }
  )
  expect(get).toHaveBeenCalledExactlyOnceWith('/api/video-studio/tasks', {
    params: { p: 3, page_size: 50 },
  })
})

test('loads one current task by its encoded platform task id', async () => {
  const current = { task: { task_id: 'task/current' } }
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: current },
  })

  await expect(getVideoStudioTask('task/current')).resolves.toBe(current)
  expect(get).toHaveBeenCalledExactlyOnceWith(
    '/api/video-studio/tasks/task%2Fcurrent'
  )
})

test('requests a side-effect-free estimate with the selected key', async () => {
  const payload = {
    model: 'doubao-seedance-2-5-260628',
    content: [{ type: 'text', text: 'test' }],
    resolution: '720p',
    ratio: '16:9',
    duration: 6,
    generate_audio: true,
    watermark: false,
  }
  const estimate = { quota: 250000, estimated_cost: 0.5, estimated: true }
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: estimate },
  })

  await expect(estimateVideoStudioTask({ tokenId: 42, payload })).resolves.toBe(
    estimate
  )
  expect(post).toHaveBeenCalledExactlyOnceWith(
    '/api/video-studio/estimate',
    payload,
    { headers: { 'X-Video-Studio-Token-ID': '42' } }
  )
})
