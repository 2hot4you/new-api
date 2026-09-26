/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { expect, test } from 'vitest'

import { videoStudioFormSchema } from '../form-schema'

test('accepts the Seedance automatic duration protocol value', () => {
  const result = videoStudioFormSchema.safeParse({
    tokenId: '1',
    model: 'doubao-seedance-2-0-260128',
    prompt: 'A slow camera move',
    mode: 'text',
    resolution: '720p',
    ratio: 'adaptive',
    duration: -1,
    generateAudio: true,
    watermark: false,
    webSearch: false,
  })

  expect(result.success).toBe(true)
})
