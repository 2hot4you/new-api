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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { Message, ParameterEnabled, PlaygroundConfig } from '../../types'
import { buildChatCompletionPayload } from './payload-builder'

const messages: Message[] = [
  {
    key: 'message-1',
    from: 'user',
    versions: [{ id: 'version-1', content: 'Hello' }],
  },
]

const config: PlaygroundConfig = {
  model: 'test-model',
  group: 'test-group',
  temperature: 0,
  top_p: 0.5,
  max_tokens: 64,
  frequency_penalty: -2,
  presence_penalty: -1,
  seed: null,
  stream: false,
}

const allDisabled: ParameterEnabled = {
  temperature: false,
  top_p: false,
  max_tokens: false,
  frequency_penalty: false,
  presence_penalty: false,
  seed: false,
}

describe('chat completion payload advanced parameters', () => {
  test('omits every advanced parameter that is disabled', () => {
    assert.deepEqual(
      buildChatCompletionPayload(messages, config, allDisabled),
      {
        model: 'test-model',
        group: 'test-group',
        messages: [{ role: 'user', content: 'Hello' }],
        stream: false,
      }
    )
  })

  test('preserves enabled zero and negative parameter values', () => {
    assert.deepEqual(
      buildChatCompletionPayload(messages, config, {
        ...allDisabled,
        temperature: true,
        frequency_penalty: true,
        presence_penalty: true,
      }),
      {
        model: 'test-model',
        group: 'test-group',
        messages: [{ role: 'user', content: 'Hello' }],
        stream: false,
        temperature: 0,
        frequency_penalty: -2,
        presence_penalty: -1,
      }
    )
  })

  test('omits an enabled seed when its value is null', () => {
    assert.deepEqual(
      buildChatCompletionPayload(messages, config, {
        ...allDisabled,
        seed: true,
      }),
      {
        model: 'test-model',
        group: 'test-group',
        messages: [{ role: 'user', content: 'Hello' }],
        stream: false,
      }
    )
  })
})
