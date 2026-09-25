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
import { describe, expect, test } from 'vitest'

import { mustSaveBeforeModelFetch } from '../channel-model-fetch'

describe('edit model discovery safety', () => {
  test('checks live discovery fields even when programmatic edits do not mark the form dirty', () => {
    const saved = {
      savedBaseURL: 'https://upstream.example',
      savedModels: 'old',
      baseURL: 'https://upstream.example',
      models: 'old',
      key: '',
    }
    for (const change of [
      { baseURL: 'https://other.example' },
      { models: 'new' },
      { key: 'replacement-key' },
    ]) {
      expect(
        mustSaveBeforeModelFetch(64, 64, false, { ...saved, ...change })
      ).toBe(true)
    }
    expect(mustSaveBeforeModelFetch(64, 64, false, saved)).toBe(false)
  })
  test('does not fetch stale persisted reseller credentials or model values', () => {
    expect(mustSaveBeforeModelFetch(64, 64, true)).toBe(true)
    expect(mustSaveBeforeModelFetch(64, 64, false)).toBe(false)
  })
  test('never queries the old provider after an unsaved type switch', () => {
    expect(mustSaveBeforeModelFetch(61, 64, false)).toBe(true)
    expect(mustSaveBeforeModelFetch(64, 1, false)).toBe(true)
    expect(mustSaveBeforeModelFetch(1, 45, true)).toBe(true)
  })
  test('new channels and unchanged ordinary providers keep discovery available', () => {
    expect(mustSaveBeforeModelFetch(undefined, 64, true)).toBe(false)
    expect(mustSaveBeforeModelFetch(1, 1, true)).toBe(false)
  })
})
