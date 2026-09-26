/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type {
  SeedancePayload,
  VideoStudioCapability,
  VideoStudioFormValues,
  VideoStudioMedia,
  VideoStudioMode,
} from '../types'

export type MediaValidationResult =
  | { valid: true }
  | {
      valid: false
      reason:
        | 'frame_required'
        | 'frame_count'
        | 'text_media_not_allowed'
        | 'reference_required'
        | 'audio_requires_visual'
        | 'image_limit'
        | 'video_limit'
        | 'audio_limit'
    }

export function validateMediaSelection(
  mode: VideoStudioMode,
  media: VideoStudioMedia[],
  capability: VideoStudioCapability
): MediaValidationResult {
  const images = media.filter((item) => item.type === 'image')
  const videos = media.filter((item) => item.type === 'video')
  const audio = media.filter((item) => item.type === 'audio')
  if (images.length > capability.max_images) {
    return { valid: false, reason: 'image_limit' }
  }
  if (videos.length > capability.max_videos) {
    return { valid: false, reason: 'video_limit' }
  }
  if (audio.length > capability.max_audio_files) {
    return { valid: false, reason: 'audio_limit' }
  }
  if (mode === 'text' && media.length > 0) {
    return { valid: false, reason: 'text_media_not_allowed' }
  }
  if (mode === 'frames') {
    const firstFrames = images.filter((item) => item.role === 'first_frame')
    const lastFrames = images.filter((item) => item.role === 'last_frame')
    if (firstFrames.length !== 1) {
      return { valid: false, reason: 'frame_required' }
    }
    if (
      images.length > 2 ||
      lastFrames.length > 1 ||
      videos.length > 0 ||
      audio.length > 0
    ) {
      return { valid: false, reason: 'frame_count' }
    }
  }
  if (mode === 'references') {
    if (media.length === 0) {
      return { valid: false, reason: 'reference_required' }
    }
    if (audio.length > 0 && images.length + videos.length === 0) {
      return { valid: false, reason: 'audio_requires_visual' }
    }
  }
  return { valid: true }
}

function mediaContent(item: VideoStudioMedia): Record<string, unknown> {
  const url = item.source === 'asset' ? `asset://${item.value}` : item.value
  const key = `${item.type}_url`
  return { type: key, role: item.role, [key]: { url } }
}

export function buildSeedancePayload(
  values: Omit<VideoStudioFormValues, 'tokenId'> & {
    media: VideoStudioMedia[]
  }
): SeedancePayload {
  const content: Array<Record<string, unknown>> = []
  const prompt = values.prompt.trim()
  if (prompt) content.push({ type: 'text', text: prompt })
  content.push(...values.media.map(mediaContent))
  const payload: SeedancePayload = {
    model: values.model,
    content,
    resolution: values.resolution,
    ratio: values.mode === 'frames' ? 'adaptive' : values.ratio,
    duration: values.duration,
    generate_audio: values.generateAudio,
    watermark: values.watermark,
  }
  if (values.webSearch) payload.tools = [{ type: 'web_search' }]
  return payload
}
