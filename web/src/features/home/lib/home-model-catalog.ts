/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  getModelCategory,
  sortModelsByReleaseDate,
} from '@/features/pricing/lib/model-directory'
import type {
  ModelCapability,
  PricingModel,
  PricingVendor,
} from '@/features/pricing/types'

export interface HomeVendor {
  id: number | null
  name: string
  icon?: string
  description?: string
  modelCount: number
}

export interface HomeModelCatalog {
  modelCount: number
  vendorCount: number
  capabilityCategoryCount: number
  vendors: HomeVendor[]
  latestModels: PricingModel[]
}

const CAPABILITY_SEARCH_ALIASES: Partial<Record<ModelCapability, string>> = {
  reasoning: 'reasoning 推理 思考',
  tools: 'tools 工具 function calling 函数调用',
  function_calling: 'function calling 函数调用 tools 工具',
  streaming: 'streaming stream 流式输出',
  vision: 'vision image input 视觉 图片理解',
  image_generation: 'image generation text to image 图片生成 生图 文生图',
  image_editing: 'image editing 图片编辑 改图',
  video_generation: 'video generation text to video 视频生成 生视频 文生视频',
  video_editing: 'video editing 视频编辑',
  audio_generation: 'audio generation 音频生成',
  structured_output: 'structured output 结构化输出',
  json_mode: 'json mode json 模式',
  web_search: 'web search 联网搜索',
  code_interpreter: 'code interpreter 代码执行',
  embeddings: 'embeddings embedding 向量嵌入',
  caching: 'cache caching 缓存',
  system_prompt: 'system prompt 系统提示词',
}

function normalize(value: string | undefined): string {
  return value?.trim().toLocaleLowerCase() ?? ''
}

function isValidReleaseDate(value: string | undefined): boolean {
  if (!value?.trim()) return false
  return Number.isFinite(Date.parse(value))
}

function vendorKey(name: string): string {
  return normalize(name)
}

export function buildHomeModelCatalog(
  models: PricingModel[],
  vendors: PricingVendor[]
): HomeModelCatalog {
  const configuredVendors = new Map(
    vendors.map((vendor) => [vendor.id, vendor])
  )
  const vendorEntries = new Map<string, HomeVendor>()
  const categories = new Set<string>()

  for (const model of models) {
    categories.add(getModelCategory(model))

    const configuredVendor =
      model.vendor_id == null
        ? undefined
        : configuredVendors.get(model.vendor_id)
    const name = (configuredVendor?.name || model.vendor_name || '').trim()
    if (!name) continue

    const key = vendorKey(name)
    const existing = vendorEntries.get(key)
    if (existing) {
      existing.modelCount += 1
      if (!existing.icon) {
        existing.icon = configuredVendor?.icon || model.vendor_icon
      }
      if (!existing.description) {
        existing.description =
          configuredVendor?.description || model.vendor_description
      }
      continue
    }

    vendorEntries.set(key, {
      id: configuredVendor?.id ?? model.vendor_id ?? null,
      name,
      icon: configuredVendor?.icon || model.vendor_icon,
      description: configuredVendor?.description || model.vendor_description,
      modelCount: 1,
    })
  }

  const activeVendors = [...vendorEntries.values()].sort((left, right) =>
    left.name.localeCompare(right.name)
  )
  const latestModels = sortModelsByReleaseDate(
    models.filter((model) => isValidReleaseDate(model.release_date))
  ).slice(0, 3)

  return {
    modelCount: models.length,
    vendorCount: activeVendors.length,
    capabilityCategoryCount: categories.size,
    vendors: activeVendors,
    latestModels,
  }
}

function modelSearchText(model: PricingModel): string {
  const capabilities = (model.capabilities ?? []).flatMap((capability) => [
    capability,
    CAPABILITY_SEARCH_ALIASES[capability] ?? '',
  ])

  return [
    model.model_name,
    model.display_name,
    model.vendor_name,
    model.description,
    model.description_en,
    ...(model.input_modalities ?? []),
    ...(model.output_modalities ?? []),
    ...capabilities,
    ...(model.supported_endpoint_types ?? []),
    model.tags,
  ]
    .filter(Boolean)
    .join(' ')
    .toLocaleLowerCase()
}

export function searchHomeModels(
  models: PricingModel[],
  query: string,
  limit: number = 6
): PricingModel[] {
  const normalizedQuery = normalize(query)
  if (!normalizedQuery || limit <= 0) return []

  return models
    .filter((model) => modelSearchText(model).includes(normalizedQuery))
    .slice(0, limit)
}
