/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { PricingModel } from '../types'

export const VIDEO_ENDPOINT_TYPE = 'openai-video'
export const VIDEO_SLOW_TASK_SECONDS = 10 * 60

export function isOpenAIVideoModel(model: PricingModel): boolean {
  return model.supported_endpoint_types?.includes(VIDEO_ENDPOINT_TYPE) ?? false
}

export function getVideoResolutions(model: PricingModel): string[] {
  if (!model.video_pricing) return []
  return model.video_pricing.rows.flatMap((row) => row.resolutions)
}

export function formatVideoDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'
  const rounded = Math.round(seconds)
  if (rounded < 60) return `${rounded}s`
  const minutes = Math.floor(rounded / 60)
  const remainingSeconds = rounded % 60
  if (remainingSeconds === 0) return `${minutes}m`
  return `${minutes}m ${remainingSeconds}s`
}
