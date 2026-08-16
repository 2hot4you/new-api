import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildChatSample } from '../../components/model-details-api'
import type { PricingModel } from '../../types'
import { buildSupportedParameters } from '../mock-stats'

const MODEL_NAMES = ['minimax-m3', 'qwen3.5-flash', 'qwen3.5-plus']

function model(modelName: string): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 0,
    completion_ratio: 0,
    enable_groups: ['default'],
    supported_endpoint_types: ['openai'],
    capabilities: ['reasoning'],
    supported_parameters: [
      'stream',
      'tools',
      'tool_choice',
      'reasoning_effort',
    ],
  }
}

describe('three-model Chat Completions API details', () => {
  test('includes required model and messages with verified chat parameters', () => {
    for (const modelName of MODEL_NAMES) {
      const parameterNames = buildSupportedParameters(model(modelName)).map(
        (parameter) => parameter.name
      )
      assert.deepEqual(parameterNames, [
        'model',
        'messages',
        'stream',
        'tools',
        'tool_choice',
        'reasoning_effort',
      ])
      const parameters = buildSupportedParameters(model(modelName))
      assert.equal(parameters[0]?.required, true)
      assert.deepEqual(parameters[0]?.enumValues, [modelName])
      assert.equal(parameters[1]?.required, true)
    }
  })

  test('renders each exact model against the Chat Completions endpoint', () => {
    for (const modelName of MODEL_NAMES) {
      const sample = buildChatSample('curl', {
        baseUrl: 'https://api.example.com',
        apiKeyEnv: 'NEW_API_KEY',
        modelName,
        endpointType: 'openai',
        endpointPath: '/v1/chat/completions',
      })
      assert.match(sample, /https:\/\/api\.example\.com\/v1\/chat\/completions/)
      assert.match(sample, new RegExp(`"model": "${modelName}"`))
      assert.match(sample, /"messages":/)
    }
  })
})
