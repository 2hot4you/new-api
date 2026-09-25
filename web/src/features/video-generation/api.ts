/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

import type {
  SeedancePayload,
  VideoStudioOptions,
  VideoStudioSubmission,
  VideoStudioTask,
  VideoStudioTaskPage,
} from './types'

type ApiEnvelope<T> = { success: boolean; message?: string; data: T }
export async function getVideoStudioOptions(): Promise<VideoStudioOptions> {
  const response = await api.get<ApiEnvelope<VideoStudioOptions>>(
    '/api/video-studio/options'
  )
  return response.data.data
}

export async function getVideoStudioTasks(input: {
  page: number
  pageSize: number
}): Promise<VideoStudioTaskPage> {
  const response = await api.get<ApiEnvelope<VideoStudioTaskPage>>(
    '/api/video-studio/tasks',
    { params: { p: input.page, page_size: input.pageSize } }
  )
  return response.data.data
}

export async function getVideoStudioTask(
  taskID: string
): Promise<VideoStudioTask> {
  const response = await api.get<ApiEnvelope<VideoStudioTask>>(
    `/api/video-studio/tasks/${encodeURIComponent(taskID)}`
  )
  return response.data.data
}

export async function submitVideoStudioTask(input: {
  tokenId: number
  requestId: string
  payload: SeedancePayload
}): Promise<VideoStudioSubmission> {
  const response = await api.post('/api/video-studio/tasks', input.payload, {
    headers: {
      'X-Video-Studio-Token-ID': String(input.tokenId),
      'X-Video-Studio-Request-ID': input.requestId,
    },
  })
  return response.data as VideoStudioSubmission
}
