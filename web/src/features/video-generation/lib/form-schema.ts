/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { z } from 'zod'

export const videoStudioFormSchema = z.object({
  tokenId: z.string().min(1),
  model: z.string().min(1),
  prompt: z.string().trim().max(20_000),
  mode: z.enum(['text', 'frames', 'references']),
  resolution: z.string().min(1),
  ratio: z.string().min(1),
  duration: z.number().int().min(4).max(30),
  generateAudio: z.boolean(),
  watermark: z.boolean(),
  webSearch: z.boolean(),
})
