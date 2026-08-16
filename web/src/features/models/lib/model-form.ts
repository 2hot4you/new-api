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
import { z } from 'zod'

import type { Model } from '../types'
import { parseModelTags as parseTagsFromUtils } from './model-utils'

// ============================================================================
// Model Form Schema
// ============================================================================

export const MODEL_MODALITY_OPTIONS = [
  'text',
  'image',
  'audio',
  'video',
  'file',
] as const

export const MODEL_CAPABILITY_OPTIONS = [
  'function_calling',
  'streaming',
  'vision',
  'json_mode',
  'structured_output',
  'reasoning',
  'tools',
  'system_prompt',
  'web_search',
  'code_interpreter',
  'caching',
  'embeddings',
  'image_generation',
  'image_editing',
  'video_generation',
  'video_editing',
  'audio_generation',
] as const

export const MODEL_PARAMETER_OPTIONS = [
  'stream',
  'temperature',
  'top_p',
  'max_tokens',
  'tools',
  'tool_choice',
  'reasoning_effort',
  'response_format',
] as const

export const MODEL_OUTPUT_FORMAT_OPTIONS = ['url', 'b64_json'] as const

export const MODEL_REFERENCE_MODALITY_OPTIONS = [
  'image',
  'video',
  'audio',
] as const

export type MarketplaceCategory =
  | 'llm'
  | 'image'
  | 'video'
  | 'audio'
  | 'embedding'

type CategorySource = {
  capabilities: readonly string[]
  output_modalities: readonly string[]
}

export function inferMarketplaceCategory(
  values: CategorySource
): MarketplaceCategory {
  if (
    values.capabilities.includes('video_generation') ||
    values.output_modalities.includes('video')
  ) {
    return 'video'
  }
  if (
    values.capabilities.includes('image_generation') ||
    values.output_modalities.includes('image')
  ) {
    return 'image'
  }
  if (
    values.capabilities.includes('audio_generation') ||
    values.output_modalities.includes('audio')
  ) {
    return 'audio'
  }
  if (values.capabilities.includes('embeddings')) return 'embedding'
  return 'llm'
}

/**
 * Model form validation schema
 */
export const modelFormSchema = z
  .object({
    id: z.number().optional(),
    model_name: z.string().min(1, 'Model name is required'),
    display_name: z.string(),
    description: z.string(),
    description_en: z.string(),
    icon: z.string(),
    tags: z.array(z.string()),
    vendor_id: z.number().int().positive().optional(),
    endpoints: z.string(),
    name_rule: z.number().min(0).max(3),
    status: z.boolean(),
    sync_official: z.boolean(),
    marketplace_enabled: z.boolean(),
    context_length: z.number().int().nonnegative(),
    max_output_tokens: z.number().int().nonnegative(),
    knowledge_cutoff: z.string(),
    release_date: z.string(),
    input_modalities: z.array(z.enum(MODEL_MODALITY_OPTIONS)),
    output_modalities: z.array(z.enum(MODEL_MODALITY_OPTIONS)),
    capabilities: z.array(z.enum(MODEL_CAPABILITY_OPTIONS)),
    supported_parameters: z.array(z.enum(MODEL_PARAMETER_OPTIONS)),
    supported_resolutions: z.array(z.string()),
    supported_aspect_ratios: z.array(z.string()),
    max_input_images: z.number().int().nonnegative(),
    output_formats: z.array(z.enum(MODEL_OUTPUT_FORMAT_OPTIONS)),
    min_duration: z.number().int().nonnegative(),
    max_duration: z.number().int().nonnegative(),
    reference_modalities: z.array(z.enum(MODEL_REFERENCE_MODALITY_OPTIONS)),
    enable_groups: z.array(z.string()),
    quota_types: z.array(z.number()),
  })
  .superRefine((values, context) => {
    const category = inferMarketplaceCategory(values)
    if (
      category === 'llm' &&
      values.context_length > 0 &&
      values.max_output_tokens > values.context_length
    ) {
      context.addIssue({
        code: 'custom',
        path: ['max_output_tokens'],
        message: 'Maximum output tokens must not exceed context length',
      })
    }
    if (values.max_duration > 0 && values.min_duration > values.max_duration) {
      for (const path of ['min_duration', 'max_duration'] as const) {
        context.addIssue({
          code: 'custom',
          path: [path],
          message: 'Minimum duration must not exceed maximum duration',
        })
      }
    }
    if (!values.marketplace_enabled) return

    const requireField = (condition: boolean, path: keyof typeof values) => {
      if (condition) return
      context.addIssue({
        code: 'custom',
        path: [path],
        message: 'Required before publishing to the model marketplace',
      })
    }

    requireField(values.model_name.trim().length > 0, 'model_name')
    requireField(values.display_name.trim().length > 0, 'display_name')
    requireField(values.description.trim().length > 0, 'description')
    requireField((values.vendor_id ?? 0) > 0, 'vendor_id')
    requireField(values.release_date.trim().length > 0, 'release_date')
    requireField(values.input_modalities.length > 0, 'input_modalities')
    requireField(values.output_modalities.length > 0, 'output_modalities')
    requireField(values.capabilities.length > 0, 'capabilities')

    if (category === 'llm') {
      requireField(
        values.supported_parameters.length > 0,
        'supported_parameters'
      )
      requireField(values.context_length > 0, 'context_length')
      requireField(values.max_output_tokens > 0, 'max_output_tokens')
      requireField(values.input_modalities.includes('text'), 'input_modalities')
      requireField(
        values.output_modalities.includes('text'),
        'output_modalities'
      )
    }
    if (category === 'image' || category === 'video') {
      requireField(
        values.supported_resolutions.length > 0,
        'supported_resolutions'
      )
      requireField(
        values.supported_aspect_ratios.length > 0,
        'supported_aspect_ratios'
      )
    }
    if (category === 'image') {
      requireField(values.output_formats.length > 0, 'output_formats')
      requireField(
        values.output_modalities.includes('image'),
        'output_modalities'
      )
      if (values.capabilities.includes('image_editing')) {
        requireField(
          values.input_modalities.includes('image'),
          'input_modalities'
        )
        requireField(values.max_input_images > 0, 'max_input_images')
      }
    }
    if (category === 'video') {
      requireField(values.min_duration > 0, 'min_duration')
      requireField(values.max_duration > 0, 'max_duration')
      requireField(
        values.output_modalities.includes('video'),
        'output_modalities'
      )
      requireField(
        values.reference_modalities.every((modality) =>
          values.input_modalities.includes(modality)
        ),
        'reference_modalities'
      )
    }
  })

export type ModelFormValues = z.infer<typeof modelFormSchema>

function includesCatalogValue<const T extends readonly string[]>(
  options: T,
  value: string
): value is T[number] {
  return options.includes(value as T[number])
}

// ============================================================================
// Vendor Form Schema
// ============================================================================

/**
 * Vendor form validation schema
 */
export const vendorFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Vendor name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  status: z.number().default(1),
})

export type VendorFormValues = z.infer<typeof vendorFormSchema>

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform model to form default values
 */
export function transformModelToFormDefaults(model: Model): ModelFormValues {
  return {
    id: model.id,
    model_name: model.model_name,
    display_name: model.display_name ?? '',
    description: model.description ?? '',
    description_en: model.description_en ?? '',
    icon: model.icon ?? '',
    tags: parseTagsFromUtils(model.tags),
    vendor_id: model.vendor_id,
    endpoints: model.endpoints ?? '',
    name_rule: model.name_rule ?? 0,
    status: model.status === 1,
    sync_official: model.sync_official === 1,
    marketplace_enabled: model.marketplace_enabled ?? false,
    context_length: model.context_length ?? 0,
    max_output_tokens: model.max_output_tokens ?? 0,
    knowledge_cutoff: model.knowledge_cutoff ?? '',
    release_date: model.release_date ?? '',
    input_modalities: model.input_modalities ?? [],
    output_modalities: model.output_modalities ?? [],
    capabilities: model.capabilities ?? [],
    supported_parameters: (model.supported_parameters ?? []).filter((value) =>
      includesCatalogValue(MODEL_PARAMETER_OPTIONS, value)
    ),
    supported_resolutions: model.supported_resolutions ?? [],
    supported_aspect_ratios: model.supported_aspect_ratios ?? [],
    max_input_images: model.max_input_images ?? 0,
    output_formats: (model.output_formats ?? []).filter((value) =>
      includesCatalogValue(MODEL_OUTPUT_FORMAT_OPTIONS, value)
    ),
    min_duration: model.min_duration ?? 0,
    max_duration: model.max_duration ?? 0,
    reference_modalities: (model.reference_modalities ?? []).filter((value) =>
      includesCatalogValue(MODEL_REFERENCE_MODALITY_OPTIONS, value)
    ),
    enable_groups: model.enable_groups ?? [],
    quota_types: model.quota_types ?? [],
  }
}

/**
 * Transform form data to model create/update payload
 */
export function transformFormDataToModelPayload(
  formData: ModelFormValues
): Partial<Model> {
  return {
    id: formData.id,
    model_name: formData.model_name,
    display_name: formData.display_name,
    description: formData.description,
    description_en: formData.description_en,
    icon: formData.icon,
    tags: formatTagsArray(formData.tags),
    vendor_id: formData.vendor_id,
    endpoints: formData.endpoints,
    name_rule: formData.name_rule,
    status: formData.status ? 1 : 0,
    sync_official: formData.sync_official ? 1 : 0,
    context_length: formData.context_length,
    max_output_tokens: formData.max_output_tokens,
    marketplace_enabled: formData.marketplace_enabled,
    knowledge_cutoff: formData.knowledge_cutoff,
    release_date: formData.release_date,
    input_modalities: formData.input_modalities,
    output_modalities: formData.output_modalities,
    capabilities: formData.capabilities,
    supported_parameters: formData.supported_parameters,
    supported_resolutions: formData.supported_resolutions,
    supported_aspect_ratios: formData.supported_aspect_ratios,
    max_input_images: formData.max_input_images,
    output_formats: formData.output_formats,
    min_duration: formData.min_duration,
    max_duration: formData.max_duration,
    reference_modalities: formData.reference_modalities,
    enable_groups: formData.enable_groups,
    quota_types: formData.quota_types,
  }
}

// ============================================================================
// Parsing and Formatting Helpers
// ============================================================================

/**
 * Format tags array to string
 */
export function formatTagsArray(tags: string[]): string {
  return tags.filter(Boolean).join(',')
}

/**
 * Validate JSON string
 */
export function validateJSON(value: string): boolean {
  if (!value || value.trim() === '') return true

  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

/**
 * Validate endpoints JSON
 */
export function validateEndpoints(endpoints: string): boolean {
  return validateJSON(endpoints)
}
