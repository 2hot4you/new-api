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
import type { Modality, ModelCapability, PricingModel } from '../types'

export type ModelCategoryId =
  | 'all'
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'embedding'
  | 'rerank'

export type ModelCategory = {
  id: ModelCategoryId
  count: number
}

export type ContextBucketId = 'lte-128k' | 'lte-256k' | 'lte-1m' | 'gt-1m'

export type ContextBucket = {
  id: ContextBucketId
  count: number
  maxTokens?: number
}

export type DirectoryBillingType = 'token' | 'request' | 'dynamic'

export type DirectoryFilterState = {
  vendor?: string
  category?: ModelCategoryId
  inputModality?: Modality
  contextBucket?: ContextBucketId
  capability?: ModelCapability
  endpointType?: string
  billingType?: DirectoryBillingType
  group?: string
  tag?: string
}

const CATEGORY_ORDER: Exclude<ModelCategoryId, 'all'>[] = [
  'text',
  'image',
  'video',
  'audio',
  'embedding',
  'rerank',
]

const MODALITY_ORDER: Modality[] = ['text', 'image', 'file', 'audio', 'video']

const CONTEXT_BUCKETS: Array<{
  id: ContextBucketId
  maxTokens?: number
}> = [
  { id: 'lte-128k', maxTokens: 128 * 1024 },
  { id: 'lte-256k', maxTokens: 256 * 1024 },
  { id: 'lte-1m', maxTokens: 1024 * 1024 },
  { id: 'gt-1m' },
]

function endpointSet(model: PricingModel): Set<string> {
  return new Set(model.supported_endpoint_types ?? [])
}

function hasCapability(
  model: PricingModel,
  capability: ModelCapability
): boolean {
  return model.capabilities?.includes(capability) ?? false
}

export function getModelCategory(model: PricingModel): ModelCategoryId {
  const output = new Set(model.output_modalities ?? [])
  const endpoints = endpointSet(model)

  if (
    output.has('video') ||
    hasCapability(model, 'video_generation') ||
    endpoints.has('openai-video')
  ) {
    return 'video'
  }
  if (
    output.has('image') ||
    hasCapability(model, 'image_generation') ||
    endpoints.has('image-generation')
  ) {
    return 'image'
  }
  if (
    output.has('audio') ||
    hasCapability(model, 'audio_generation') ||
    endpoints.has('audio')
  ) {
    return 'audio'
  }
  if (hasCapability(model, 'embeddings') || endpoints.has('embeddings')) {
    return 'embedding'
  }
  if (endpoints.has('jina-rerank')) {
    return 'rerank'
  }
  return 'text'
}

export function getModelCategories(models: PricingModel[]): ModelCategory[] {
  const counts = new Map<Exclude<ModelCategoryId, 'all'>, number>()
  for (const model of models) {
    const category = getModelCategory(model)
    if (category === 'all') continue
    counts.set(category, (counts.get(category) ?? 0) + 1)
  }

  return [
    { id: 'all', count: models.length },
    ...CATEGORY_ORDER.flatMap((id) => {
      const count = counts.get(id) ?? 0
      return count > 0 ? [{ id, count }] : []
    }),
  ]
}

export function getModelInputModalities(model: PricingModel): Modality[] {
  const explicit = new Set(model.input_modalities ?? [])
  if (explicit.size > 0) {
    return MODALITY_ORDER.filter((modality) => explicit.has(modality))
  }

  const endpoints = endpointSet(model)
  if (
    endpoints.has('openai') ||
    endpoints.has('openai-response') ||
    endpoints.has('anthropic') ||
    endpoints.has('gemini') ||
    endpoints.has('image-generation') ||
    endpoints.has('openai-video')
  ) {
    return ['text']
  }
  return []
}

export function getContextBucketId(
  contextLength: number | undefined
): ContextBucketId | null {
  if (
    contextLength == null ||
    !Number.isFinite(contextLength) ||
    contextLength <= 0
  ) {
    return null
  }
  if (contextLength <= 128 * 1024) return 'lte-128k'
  if (contextLength <= 256 * 1024) return 'lte-256k'
  if (contextLength <= 1024 * 1024) return 'lte-1m'
  return 'gt-1m'
}

export function getContextBuckets(models: PricingModel[]): ContextBucket[] {
  const counts = new Map<ContextBucketId, number>()
  for (const model of models) {
    const bucket = getContextBucketId(model.context_length)
    if (bucket) counts.set(bucket, (counts.get(bucket) ?? 0) + 1)
  }

  return CONTEXT_BUCKETS.flatMap((bucket) => {
    const count = counts.get(bucket.id) ?? 0
    return count > 0 ? [{ ...bucket, count }] : []
  })
}

function parseReleaseDate(releaseDate: string | undefined): number | null {
  if (!releaseDate?.trim()) return null
  const parsed = Date.parse(releaseDate)
  return Number.isFinite(parsed) ? parsed : null
}

export function compareModelsByReleaseDate(
  left: PricingModel,
  right: PricingModel
): number {
  const leftTime = parseReleaseDate(left.release_date)
  const rightTime = parseReleaseDate(right.release_date)

  if (leftTime != null && rightTime != null && leftTime !== rightTime) {
    return rightTime - leftTime
  }
  if (leftTime != null && rightTime == null) return -1
  if (leftTime == null && rightTime != null) return 1
  return left.model_name.localeCompare(right.model_name)
}

export function sortModelsByReleaseDate(
  models: PricingModel[]
): PricingModel[] {
  return [...models].sort(compareModelsByReleaseDate)
}

function modelBillingType(model: PricingModel): DirectoryBillingType {
  if (model.billing_mode === 'tiered_expr') return 'dynamic'
  return model.quota_type === 1 ? 'request' : 'token'
}

function hasTag(model: PricingModel, tag: string): boolean {
  const expected = tag.toLowerCase()
  return (model.tags ?? '')
    .split(/[,;|\s]+/)
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
    .includes(expected)
}

export function filterModelsByDirectory(
  models: PricingModel[],
  filters: DirectoryFilterState
): PricingModel[] {
  return models.filter((model) => {
    if (filters.vendor && model.vendor_name !== filters.vendor) return false
    if (
      filters.category &&
      filters.category !== 'all' &&
      getModelCategory(model) !== filters.category
    ) {
      return false
    }
    if (
      filters.inputModality &&
      !getModelInputModalities(model).includes(filters.inputModality)
    ) {
      return false
    }
    if (
      filters.contextBucket &&
      getContextBucketId(model.context_length) !== filters.contextBucket
    ) {
      return false
    }
    if (
      filters.capability &&
      !(model.capabilities?.includes(filters.capability) ?? false)
    ) {
      return false
    }
    if (
      filters.endpointType &&
      !(model.supported_endpoint_types?.includes(filters.endpointType) ?? false)
    ) {
      return false
    }
    if (
      filters.billingType &&
      modelBillingType(model) !== filters.billingType
    ) {
      return false
    }
    if (filters.group && !model.enable_groups?.includes(filters.group)) {
      return false
    }
    if (filters.tag && !hasTag(model, filters.tag)) return false
    return true
  })
}
