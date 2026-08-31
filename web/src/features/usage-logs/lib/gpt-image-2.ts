import type { GPTImage2LogV1 } from '../types'

type GPTImage2LogLike = {
  model_name?: unknown
  other?: unknown
}

export type GPTImage2LogState =
  | { kind: 'current'; model: 'gpt-image-2'; snapshot: GPTImage2LogV1 }
  | { kind: 'history'; model: 'gpt-image-2' }
  | { kind: 'not-gpt-image-2' }

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
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

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim() !== ''
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isInteger(value) && value > 0
}

export function parseGPTImage2Log(other: unknown): GPTImage2LogV1 | null {
  const value = parseOther(other)?.gpt_image_2
  if (!isRecord(value)) return null
  if (
    value.version !== 1 ||
    value.model !== 'gpt-image-2' ||
    (value.operation !== 'generation' && value.operation !== 'edit') ||
    !isNonEmptyString(value.quality) ||
    !isNonEmptyString(value.background) ||
    (value.output_format !== 'png' &&
      value.output_format !== 'jpeg' &&
      value.output_format !== 'webp') ||
    !isNonEmptyString(value.moderation) ||
    !isNonEmptyString(value.size) ||
    (value.user !== undefined && typeof value.user !== 'string') ||
    !isPositiveInteger(value.requested_output_count) ||
    !isPositiveInteger(value.output_count)
  ) {
    return null
  }
  return value as unknown as GPTImage2LogV1
}

export function isGPTImage2Log(log: GPTImage2LogLike): boolean {
  return log.model_name === 'gpt-image-2'
}

export function getGPTImage2LogState(log: GPTImage2LogLike): GPTImage2LogState {
  if (!isGPTImage2Log(log)) return { kind: 'not-gpt-image-2' }
  const snapshot = parseGPTImage2Log(log.other)
  if (!snapshot) return { kind: 'history', model: 'gpt-image-2' }
  return { kind: 'current', model: 'gpt-image-2', snapshot }
}

export function getGPTImage2ListSummary(log: GPTImage2LogLike): string | null {
  const state = getGPTImage2LogState(log)
  if (state.kind !== 'current') return null
  const { snapshot } = state
  const noun = snapshot.output_count === 1 ? 'image' : 'images'
  return `${snapshot.quality.toUpperCase()} · ${snapshot.size} · ${snapshot.output_format.toUpperCase()} · ${snapshot.output_count} ${noun}`
}
