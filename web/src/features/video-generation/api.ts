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
  VideoStudioTask,
} from './types'

type ApiEnvelope<T> = { success: boolean; message?: string; data: T }
type PageEnvelope<T> = { items: T[]; total: number }

export async function getVideoStudioOptions(): Promise<VideoStudioOptions> {
  const response = await api.get<ApiEnvelope<VideoStudioOptions>>(
    '/api/video-studio/options'
  )
  return response.data.data
}

export async function getVideoStudioTasks(): Promise<VideoStudioTask[]> {
  const response = await api.get<ApiEnvelope<PageEnvelope<VideoStudioTask>>>(
    '/api/video-studio/tasks?page=1&page_size=30'
  )
  return response.data.data.items ?? []
}

export async function submitVideoStudioTask(input: {
  tokenId: number
  requestId: string
  payload: SeedancePayload
}): Promise<unknown> {
  const response = await api.post('/api/video-studio/tasks', input.payload, {
    headers: {
      'X-Video-Studio-Token-ID': String(input.tokenId),
      'X-Video-Studio-Request-ID': input.requestId,
    },
  })
  return response.data
}
