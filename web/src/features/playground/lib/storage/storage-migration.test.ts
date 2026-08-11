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
import { afterEach, beforeEach, describe, test } from 'node:test'

import { STORAGE_KEYS } from '../../constants'
import { getInitialParameterEnabled } from '../state/playground-state-utils'
import { loadParameterEnabled } from './storage'

class MemoryStorage {
  private values = new Map<string, string>()

  getItem(key: string) {
    return this.values.get(key) ?? null
  }

  setItem(key: string, value: string) {
    this.values.set(key, value)
  }

  removeItem(key: string) {
    this.values.delete(key)
  }

  clear() {
    this.values.clear()
  }
}

const memoryStorage = new MemoryStorage()
const originalLocalStorage = globalThis.localStorage

beforeEach(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: memoryStorage,
  })
  memoryStorage.clear()
})

afterEach(() => {
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: originalLocalStorage,
  })
})

const allDisabled = {
  temperature: false,
  top_p: false,
  max_tokens: false,
  frequency_penalty: false,
  presence_penalty: false,
  seed: false,
}

describe('parameter enabled storage migration', () => {
  test('starts a clean playground with every advanced parameter disabled', () => {
    assert.deepEqual(getInitialParameterEnabled(), allDisabled)
  })

  test('resets version-one enabled flags while preserving the other storage keys', () => {
    const config = {
      version: 1,
      data: {
        model: 'legacy-model',
        group: 'legacy-group',
        temperature: 0,
        top_p: 0.5,
        max_tokens: 99,
        frequency_penalty: -2,
        presence_penalty: -1,
        seed: 7,
        stream: false,
      },
    }
    const messages = {
      version: 1,
      data: [
        {
          key: 'message-1',
          from: 'user',
          versions: [{ id: 'version-1', content: 'Keep this message' }],
        },
      ],
    }

    localStorage.setItem(STORAGE_KEYS.CONFIG, JSON.stringify(config))
    localStorage.setItem(STORAGE_KEYS.MESSAGES, JSON.stringify(messages))
    localStorage.setItem(
      STORAGE_KEYS.PARAMETER_ENABLED,
      JSON.stringify({
        version: 1,
        data: {
          temperature: true,
          top_p: true,
          max_tokens: true,
          frequency_penalty: true,
          presence_penalty: true,
          seed: true,
        },
      })
    )

    assert.deepEqual(loadParameterEnabled(), allDisabled)
    assert.equal(
      localStorage.getItem(STORAGE_KEYS.CONFIG),
      JSON.stringify(config)
    )
    assert.equal(
      localStorage.getItem(STORAGE_KEYS.MESSAGES),
      JSON.stringify(messages)
    )
    assert.deepEqual(
      JSON.parse(localStorage.getItem(STORAGE_KEYS.PARAMETER_ENABLED) ?? '{}'),
      { version: 2, data: allDisabled }
    )
  })

  test('keeps choices stored with the current parameter-enabled version', () => {
    const currentChoices = {
      temperature: true,
      top_p: false,
      max_tokens: true,
      frequency_penalty: false,
      presence_penalty: true,
      seed: true,
    }
    const currentEnvelope = { version: 2, data: currentChoices }
    localStorage.setItem(
      STORAGE_KEYS.PARAMETER_ENABLED,
      JSON.stringify(currentEnvelope)
    )

    assert.deepEqual(loadParameterEnabled(), currentChoices)
    assert.equal(
      localStorage.getItem(STORAGE_KEYS.PARAMETER_ENABLED),
      JSON.stringify(currentEnvelope)
    )
  })
})
