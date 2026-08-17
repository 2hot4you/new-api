import { resolve } from 'node:path'

export const TARGET_MODELS = [
  'deepseek-v4-flash-202605',
  'deepseek-v4-pro-202606',
  'doubao-seedance-2-0-260128',
  'doubao-seedance-2-0-fast-260128',
  'glm-5.2',
  'grok-imagine-image',
  'grok-imagine-image-quality',
  'grok-imagine-video',
  'grok-imagine-video-1.5',
  'kimi-k2.5',
  'kimi-k2.6',
  'kimi-k2.7-code',
  'kimi-k3',
  'minimax-m3',
  'qwen3.5-flash',
  'qwen3.5-plus',
  'qwen3.8-max',
] as const

const ALLOWED_MODALITIES = new Set(['text', 'image', 'audio', 'video', 'file'])
const ALLOWED_CAPABILITIES = new Set([
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
])
const ALLOWED_PARAMETERS = new Set([
  'stream',
  'temperature',
  'top_p',
  'max_tokens',
  'tools',
  'tool_choice',
  'reasoning_effort',
  'response_format',
])
const ALLOWED_ICONS = new Set([
  'Minimax',
  'Qwen',
  'DeepSeek',
  'Doubao',
  'Zhipu',
  'Grok',
  'Moonshot',
])

export type Evidence = {
  kind: string
  url: string
  source_model_id: string
  exact_match: boolean
}

export type CuratedModel = {
  model_name: string
  description: string
  description_en: string
  icon: string
  release_date: string
  knowledge_cutoff: string
  input_modalities: string[]
  output_modalities: string[]
  capabilities: string[]
  supported_parameters: string[]
  context_length: number
  max_output_tokens: number
  sources: Evidence[]
  notes: string[]
}

export type CuratedManifest = {
  captured_at: string
  models: CuratedModel[]
}

function validateEnumValues(
  errors: string[],
  modelName: string,
  field: string,
  values: string[],
  allowed: Set<string>
) {
  for (const value of values) {
    if (!allowed.has(value)) {
      errors.push(`${modelName}.${field} contains unsupported value: ${value}`)
    }
  }
}

function normalizedSourceModelID(sourceModelID: string): string {
  const segments = sourceModelID.split('/').filter(Boolean)
  return segments.at(-1) ?? ''
}

export function validateManifest(input: CuratedManifest): string[] {
  const errors: string[] = []
  const expected = new Set<string>(TARGET_MODELS)
  const counts = new Map<string, number>()

  for (const model of input.models) {
    counts.set(model.model_name, (counts.get(model.model_name) ?? 0) + 1)
  }

  for (const modelName of TARGET_MODELS) {
    if (!counts.has(modelName)) errors.push(`missing target model: ${modelName}`)
  }
  for (const [modelName, count] of counts) {
    if (!expected.has(modelName)) errors.push(`unexpected target model: ${modelName}`)
    if (count > 1) errors.push(`duplicate model: ${modelName}`)
  }

  for (const model of input.models) {
    const prefix = model.model_name
    if (!model.description.trim()) errors.push(`${prefix}.description is required`)
    if (!model.description_en.trim()) errors.push(`${prefix}.description_en is required`)
    if (!ALLOWED_ICONS.has(model.icon)) errors.push(`${prefix}.icon is not a resolvable LobeHub key: ${model.icon}`)

    validateEnumValues(errors, prefix, 'input_modalities', model.input_modalities, ALLOWED_MODALITIES)
    validateEnumValues(errors, prefix, 'output_modalities', model.output_modalities, ALLOWED_MODALITIES)
    validateEnumValues(errors, prefix, 'capabilities', model.capabilities, ALLOWED_CAPABILITIES)
    validateEnumValues(errors, prefix, 'supported_parameters', model.supported_parameters, ALLOWED_PARAMETERS)

    if (!Number.isInteger(model.context_length) || model.context_length < 0) {
      errors.push(`${prefix}.context_length must be a non-negative integer`)
    }
    if (!Number.isInteger(model.max_output_tokens) || model.max_output_tokens < 0) {
      errors.push(`${prefix}.max_output_tokens must be a non-negative integer`)
    } else if (model.context_length >= 0 && model.max_output_tokens > model.context_length) {
      errors.push(`${prefix}.max_output_tokens cannot exceed context_length`)
    }

    if (model.sources.length === 0) {
      errors.push(`${prefix}.sources must contain at least one evidence entry`)
    }
    model.sources.forEach((source, index) => {
      if (!source.url.startsWith('https://') && !source.url.startsWith('repo://')) {
        errors.push(`${prefix}.sources[${index}].url must use https:// or repo://`)
      }
      if (!source.exact_match || normalizedSourceModelID(source.source_model_id) !== model.model_name) {
        errors.push(`${prefix}.sources[${index}] does not map to the exact model id: ${source.source_model_id}`)
      }
    })
  }

  return errors
}

if (import.meta.main) {
  const manifestPath = process.argv[2]
  if (!manifestPath) {
    console.error('usage: bun validate-manifest.ts <manifest.json>')
    process.exit(2)
  }

  const manifest = (await Bun.file(resolve(manifestPath)).json()) as CuratedManifest
  const errors = validateManifest(manifest)
  if (errors.length > 0) {
    errors.forEach((error) => console.error(error))
    process.exit(1)
  }
  console.log(`${manifest.models.length} models validated`)
}
