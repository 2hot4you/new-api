/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getVideoStudioTask, getVideoStudioTasks } from '../api'

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
