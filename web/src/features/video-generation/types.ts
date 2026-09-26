/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { TaskLog } from '@/features/usage-logs/types'

export type VideoStudioMode = 'text' | 'frames' | 'references'
export type VideoStudioMediaType = 'image' | 'video' | 'audio'
export type VideoStudioMediaRole =
  | 'first_frame'
  | 'last_frame'
  | 'reference_image'
  | 'reference_video'
  | 'reference_audio'

export type VideoStudioMedia = {
  clientId: string
  type: VideoStudioMediaType
  source: 'asset' | 'url'
  role: VideoStudioMediaRole
  value: string
  name: string
  expiresAt?: number
  mentionIndex?: number
}

export type VideoStudioCapability = {
  max_duration: number
  max_images: number
  max_videos: number
  max_audio_files: number
  resolutions: string[]
  ratios: string[]
  supports_auto_duration: boolean
  supports_web_search: boolean
}

export type VideoStudioToken = {
  id: number
  name: string
  masked_key: string
  group: string
  unlimited_quota: boolean
  remain_quota?: number
  available_models: string[]
}

export type VideoStudioOptions = {
  tokens: VideoStudioToken[]
  capabilities: Record<string, VideoStudioCapability>
}

export type VideoStudioRequestSnapshot = {
  version: number
  request_id?: string
  model: string
  prompt?: string
  resolution: string
  ratio: string
  duration: number
  generate_audio: boolean
  watermark: boolean
  web_search: boolean
  media?: Array<{
    type: VideoStudioMediaType
    role: VideoStudioMediaRole
    source: 'asset' | 'url'
    asset_id?: string
    url?: string
    name?: string
    expires_at?: number
  }>
}

export type VideoStudioTask = {
  task: TaskLog
  request?: VideoStudioRequestSnapshot
  unavailable_asset_ids?: string[]
}

export type VideoStudioTaskPage = {
  items: VideoStudioTask[]
  total: number
  page: number
  page_size: number
}

export type VideoStudioSubmission = {
  id?: string
  task_id?: string
  status?: string
  model?: string
  created_at?: number
}

export type VideoStudioEstimate = {
  model: string
  upstream_model?: string
  billing_model?: string
  quota: number
  estimated_cost: number
  estimated_tokens?: number
  group_ratio: number
  group_special_ratio?: number
  other_ratios?: Record<string, number>
  estimated: true
}

export type SeedancePayload = {
  model: string
  content: Array<Record<string, unknown>>
  resolution: string
  ratio: string
  duration: number
  generate_audio: boolean
  watermark: boolean
  tools?: Array<{ type: 'web_search' }>
}

export type VideoStudioFormValues = {
  tokenId: string
  model: string
  prompt: string
  mode: VideoStudioMode
  resolution: string
  ratio: string
  duration: number
  generateAudio: boolean
  watermark: boolean
  webSearch: boolean
}
