import { describe, expect, test } from 'bun:test'

import manifest from './source-manifest.json'
import { validateManifest } from './validate-manifest'

type Manifest = typeof manifest

function cloneManifest(): Manifest {
  return structuredClone(manifest)
}

describe('validateManifest', () => {
  test('accepts the complete curated 17-model manifest', () => {
    expect(validateManifest(cloneManifest())).toEqual([])
  })

  test('rejects a missing target model', () => {
    const input = cloneManifest()
    input.models = input.models.filter((model) => model.model_name !== 'glm-5.2')

    expect(validateManifest(input)).toContain('missing target model: glm-5.2')
  })

  test('rejects a duplicate target model', () => {
    const input = cloneManifest()
    input.models.push(structuredClone(input.models[0]))

    expect(validateManifest(input)).toContain('duplicate model: minimax-m3')
  })

  test('rejects unsupported enum values with their field name', () => {
    const input = cloneManifest()
    input.models[0].input_modalities.push('binary' as never)
    input.models[0].capabilities.push('magic' as never)
    input.models[0].supported_parameters.push('vendor_flag' as never)

    expect(validateManifest(input)).toEqual(
      expect.arrayContaining([
        'minimax-m3.input_modalities contains unsupported value: binary',
        'minimax-m3.capabilities contains unsupported value: magic',
        'minimax-m3.supported_parameters contains unsupported value: vendor_flag',
      ])
    )
  })

  test('rejects invalid token limits', () => {
    const input = cloneManifest()
    input.models[0].context_length = -1
    input.models[1].context_length = 1024
    input.models[1].max_output_tokens = 2048

    expect(validateManifest(input)).toEqual(
      expect.arrayContaining([
        'minimax-m3.context_length must be a non-negative integer',
        'qwen3.5-flash.max_output_tokens cannot exceed context_length',
      ])
    )
  })

  test('rejects missing bilingual descriptions and source evidence', () => {
    const input = cloneManifest()
    input.models[0].description = ''
    input.models[0].description_en = ''
    input.models[0].sources = []

    expect(validateManifest(input)).toEqual(
      expect.arrayContaining([
        'minimax-m3.description is required',
        'minimax-m3.description_en is required',
        'minimax-m3.sources must contain at least one evidence entry',
      ])
    )
  })

  test('rejects evidence mapped to a different model id', () => {
    const input = cloneManifest()
    input.models[0].sources[0].source_model_id = 'minimax/minimax-m2.7'

    expect(validateManifest(input)).toContain(
      'minimax-m3.sources[0] does not map to the exact model id: minimax/minimax-m2.7'
    )
  })
})
