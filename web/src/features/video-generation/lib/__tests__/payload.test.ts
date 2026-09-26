/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, it } from 'vitest'

import type { VideoStudioCapability, VideoStudioMedia } from '../../types'
import { buildSeedancePayload, validateMediaSelection } from '../payload'

const capability: VideoStudioCapability = {
  max_duration: 15,
  max_images: 9,
  max_videos: 3,
  max_audio_files: 3,
  resolutions: ['480p', '720p'],
  ratios: ['16:9', 'adaptive'],
  supports_auto_duration: true,
  supports_web_search: true,
}

function media(overrides: Partial<VideoStudioMedia>): VideoStudioMedia {
  return {
    clientId: 'media-1',
    type: 'image',
    source: 'asset',
    role: 'reference_image',
    value: 'asset-123',
    name: 'reference',
    ...overrides,
  }
}

describe('Seedance video studio payload', () => {
  it('forces adaptive ratio for first-frame generation', () => {
    const payload = buildSeedancePayload({
      model: 'doubao-seedance-2-0-260128',
      prompt: 'Camera slowly moves forward',
      mode: 'frames',
      resolution: '720p',
      ratio: '16:9',
      duration: 6,
      generateAudio: true,
      watermark: false,
      webSearch: false,
      media: [media({ role: 'first_frame' })],
    })

    expect(payload.ratio).toBe('adaptive')
    expect(payload.content[1]).toEqual({
      type: 'image_url',
      role: 'first_frame',
      image_url: { url: 'asset://asset-123' },
    })
  })

  it('preserves public URLs and reference media roles', () => {
    const payload = buildSeedancePayload({
      model: 'doubao-seedance-2-5-260628',
      prompt: 'Animate the scene',
      mode: 'references',
      resolution: '480p',
      ratio: '9:16',
      duration: 15,
      generateAudio: true,
      watermark: false,
      webSearch: true,
      media: [
        media({
          type: 'video',
          source: 'url',
          role: 'reference_video',
          value: 'https://cdn.example/video.mp4',
        }),
      ],
    })

    expect(payload.content[1]).toEqual({
      type: 'video_url',
      role: 'reference_video',
      video_url: { url: 'https://cdn.example/video.mp4' },
    })
    expect(payload.tools).toEqual([{ type: 'web_search' }])
  })

  it('preserves automatic ratio and duration protocol values', () => {
    const payload = buildSeedancePayload({
      model: 'doubao-seedance-2-0-260128',
      prompt: 'A slow camera move',
      mode: 'text',
      resolution: '720p',
      ratio: 'adaptive',
      duration: -1,
      generateAudio: true,
      watermark: false,
      webSearch: false,
      media: [],
    })

    expect(payload.ratio).toBe('adaptive')
    expect(payload.duration).toBe(-1)
  })

  it('rejects media that exceed the selected model limits', () => {
    const items = Array.from({ length: 4 }, (_, index) =>
      media({
        clientId: `audio-${index}`,
        type: 'audio',
        role: 'reference_audio',
      })
    )

    expect(validateMediaSelection('references', items, capability)).toEqual({
      valid: false,
      reason: 'audio_limit',
    })
  })

  it('rejects reference media in text-to-video mode', () => {
    expect(validateMediaSelection('text', [media({})], capability)).toEqual({
      valid: false,
      reason: 'text_media_not_allowed',
    })
  })
})
