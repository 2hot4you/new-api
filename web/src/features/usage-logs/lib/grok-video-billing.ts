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
import type { GrokVideoBillingV1 } from '../types'

const GROK_VIDEO_MODELS = new Set([
  'grok-imagine-video',
  'grok-imagine-video-1.5',
])

type GrokVideoLogLike = {
  type?: unknown
  model_name?: unknown
  other?: unknown
}

export type GrokVideoBillingState =
  | { kind: 'current'; model: string; billing: GrokVideoBillingV1 }
  | { kind: 'history'; model: string }
  | { kind: 'not-grok-video' }

export function isGrokVideoModel(
  model: unknown
): model is GrokVideoBillingV1['model'] {
  return typeof model === 'string' && GROK_VIDEO_MODELS.has(model)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isNonNegativeFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

function isNonNegativeInteger(value: unknown): value is number {
  return Number.isInteger(value) && isNonNegativeFiniteNumber(value)
}

function isOperationInputPair(operation: unknown, inputType: unknown): boolean {
  return (
    (operation === 'text_to_video' && inputType === 'text') ||
    (operation === 'image_to_video' && inputType === 'image') ||
    (operation === 'video_edit' && inputType === 'video')
  )
}

function parseOther(value: unknown): Record<string, unknown> | null {
  if (isRecord(value)) return value
  if (typeof value !== 'string' || value === '') return null
  try {
    const parsed: unknown = JSON.parse(value)
    return isRecord(parsed) ? parsed : null
  } catch {
    return null
  }
}

/** Parse only the complete, versioned v1 contract; never synthesize fields. */
export function parseGrokVideoBilling(
  other: unknown
): GrokVideoBillingV1 | null {
  const root = parseOther(other)
  const value = root?.grok_video_billing
  if (!isRecord(value)) return null

  if (
    value.version !== 1 ||
    !isGrokVideoModel(value.model) ||
    (value.operation !== 'text_to_video' &&
      value.operation !== 'image_to_video' &&
      value.operation !== 'video_edit') ||
    (value.input_type !== 'text' &&
      value.input_type !== 'image' &&
      value.input_type !== 'video') ||
    !isOperationInputPair(value.operation, value.input_type) ||
    !isNonNegativeFiniteNumber(value.requested_duration_seconds) ||
    !isNonNegativeFiniteNumber(value.estimated_duration_seconds) ||
    !isNonNegativeFiniteNumber(value.actual_duration_seconds) ||
    typeof value.requested_resolution !== 'string' ||
    typeof value.estimated_resolution !== 'string' ||
    value.estimated_resolution.trim() === '' ||
    typeof value.actual_resolution !== 'string' ||
    value.actual_resolution.trim() === '' ||
    typeof value.aspect_ratio !== 'string' ||
    !isNonNegativeInteger(value.input_image_count) ||
    !isNonNegativeFiniteNumber(value.video_input_billed_seconds) ||
    !isNonNegativeFiniteNumber(value.output_unit_price) ||
    !isNonNegativeFiniteNumber(value.image_input_unit_price) ||
    !isNonNegativeFiniteNumber(value.video_input_unit_price) ||
    !isNonNegativeFiniteNumber(value.output_cost) ||
    !isNonNegativeFiniteNumber(value.image_input_cost) ||
    !isNonNegativeFiniteNumber(value.video_input_cost) ||
    !isNonNegativeFiniteNumber(value.subtotal) ||
    !isNonNegativeFiniteNumber(value.group_ratio) ||
    !isNonNegativeFiniteNumber(value.final_cost)
  ) {
    return null
  }

  return value as unknown as GrokVideoBillingV1
}

export function getGrokVideoBillingState(
  log: GrokVideoLogLike
): GrokVideoBillingState {
  if (!isGrokVideoModel(log.model_name)) return { kind: 'not-grok-video' }

  const billing = parseGrokVideoBilling(log.other)
  if (billing && billing.model === log.model_name) {
    return { kind: 'current', model: log.model_name, billing }
  }
  return { kind: 'history', model: log.model_name }
}

export function isGrokVideoLog(log: GrokVideoLogLike): boolean {
  return isGrokVideoModel(log.model_name)
}

/** Detailed billing is only valid for terminal consumption records. */
export function isGrokVideoBillingLog(log: GrokVideoLogLike): boolean {
  return log.type === 2 && isGrokVideoModel(log.model_name)
}

export function formatGrokVideoCny(value: number): string {
  return `¥${value.toFixed(6)}`
}

export function formatGrokVideoFormula(billing: GrokVideoBillingV1): string {
  const output = `${formatGrokVideoCny(billing.output_unit_price)} × ${billing.actual_duration_seconds}`
  let terms = output
  if (billing.operation === 'image_to_video') {
    terms = `${output} + ${formatGrokVideoCny(billing.image_input_unit_price)} × ${billing.input_image_count}`
  } else if (billing.operation === 'video_edit') {
    terms = `${output} + ${formatGrokVideoCny(billing.video_input_unit_price)} × ${billing.video_input_billed_seconds}`
  }
  return `(${terms}) × ${billing.group_ratio.toFixed(4)} = ${formatGrokVideoCny(billing.final_cost)}`
}

export function getGrokVideoListSummary(log: GrokVideoLogLike): string | null {
  const state = getGrokVideoBillingState(log)
  if (state.kind === 'not-grok-video') return null
  if (state.kind === 'history') return state.model
  const { billing } = state
  return `${billing.actual_resolution.toUpperCase()} · ${billing.actual_duration_seconds}s · ${billing.operation}`
}
