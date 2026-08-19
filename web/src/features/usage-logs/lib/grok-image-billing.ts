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
import type { GrokImageBillingV1 } from '../types'

const GROK_IMAGE_MODELS = new Set([
  'grok-imagine-image',
  'grok-imagine-image-quality',
  'grok-imagine-image-2.0',
])

type GrokImageLogLike = {
  model_name?: unknown
  other?: unknown
}

export type GrokImageBillingState =
  | { kind: 'current'; model: string; billing: GrokImageBillingV1 }
  | { kind: 'history'; model: string }
  | { kind: 'not-grok-image' }

export function isGrokImageModel(
  model: unknown
): model is GrokImageBillingV1['model'] {
  return typeof model === 'string' && GROK_IMAGE_MODELS.has(model)
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
export function parseGrokImageBilling(
  other: unknown
): GrokImageBillingV1 | null {
  const root = parseOther(other)
  const value = root?.grok_image_billing
  if (!isRecord(value)) return null

  if (
    value.version !== 1 ||
    !isGrokImageModel(value.model) ||
    (value.operation !== 'generation' && value.operation !== 'edit') ||
    typeof value.resolution !== 'string' ||
    value.resolution.trim() === '' ||
    typeof value.aspect_ratio !== 'string' ||
    value.aspect_ratio.trim() === '' ||
    !isNonNegativeInteger(value.requested_output_count) ||
    value.requested_output_count === 0 ||
    !isNonNegativeInteger(value.output_count) ||
    value.output_count > value.requested_output_count ||
    !isNonNegativeInteger(value.input_image_count) ||
    !isNonNegativeFiniteNumber(value.output_unit_price) ||
    !isNonNegativeFiniteNumber(value.input_unit_price) ||
    !isNonNegativeFiniteNumber(value.output_cost) ||
    !isNonNegativeFiniteNumber(value.input_cost) ||
    !isNonNegativeFiniteNumber(value.subtotal) ||
    !isNonNegativeFiniteNumber(value.group_ratio) ||
    !isNonNegativeFiniteNumber(value.final_cost)
  ) {
    return null
  }
  if (
    value.model === 'grok-imagine-image-2.0' &&
    value.quality !== 'low' &&
    value.quality !== 'medium'
  ) {
    return null
  }

  return value as unknown as GrokImageBillingV1
}

export function getGrokImageBillingState(
  log: GrokImageLogLike
): GrokImageBillingState {
  if (!isGrokImageModel(log.model_name)) return { kind: 'not-grok-image' }

  const billing = parseGrokImageBilling(log.other)
  if (billing && billing.model === log.model_name) {
    return { kind: 'current', model: log.model_name, billing }
  }
  return { kind: 'history', model: log.model_name }
}

export function isGrokImageLog(log: GrokImageLogLike): boolean {
  return isGrokImageModel(log.model_name)
}

export function formatGrokImageCny(value: number): string {
  return `¥${value.toFixed(6)}`
}

export function formatGrokImageFormula(billing: GrokImageBillingV1): string {
  const output = `${formatGrokImageCny(billing.output_unit_price)} × ${billing.output_count}`
  const terms =
    billing.operation === 'edit'
      ? `${output} + ${formatGrokImageCny(billing.input_unit_price)} × ${billing.input_image_count}`
      : output
  return `(${terms}) × ${billing.group_ratio.toFixed(4)} = ${formatGrokImageCny(billing.final_cost)}`
}

export function getGrokImageListSummary(log: GrokImageLogLike): string | null {
  const state = getGrokImageBillingState(log)
  if (state.kind === 'not-grok-image') return null
  if (state.kind === 'history') return state.model
  const { billing } = state
  const noun = billing.output_count === 1 ? 'image' : 'images'
  const quality = billing.quality
    ? `${billing.quality.charAt(0).toUpperCase()}${billing.quality.slice(1)} · `
    : ''
  return `${quality}${billing.resolution.toUpperCase()} · ${billing.aspect_ratio} · ${billing.output_count} ${noun}`
}
