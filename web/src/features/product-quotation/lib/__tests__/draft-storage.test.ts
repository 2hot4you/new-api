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

import { describe, test } from 'vitest'

import type { QuotationDraft } from '../../types'
import {
  clearQuotationDraft,
  createDefaultQuotationDraft,
  loadQuotationDraft,
  saveQuotationDraft,
} from '../draft-storage'

const STORAGE_KEY = 'new-api:product-quotation:draft:v1'

type MemoryStorage = {
  values: Map<string, string>
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
}

function memoryStorage(initial: Record<string, string> = {}): MemoryStorage {
  const values = new Map(Object.entries(initial))
  return {
    values,
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => void values.set(key, value),
    removeItem: (key) => void values.delete(key),
  }
}

function quotationDraft(
  overrides: Partial<QuotationDraft> = {}
): QuotationDraft {
  return {
    title: 'Acme annual quotation',
    customer: 'Acme, Inc.',
    quotedBy: 'Molii',
    quoteDate: '2026-09-08',
    globalDiscount: 8,
    priceBasis: { type: 'group', group: 'vip' },
    selectedModelIds: ['model-a', 'model-b'],
    providerOverrides: {
      '10': { discount: 5, note: 'Partner terms' },
    },
    ...overrides,
  }
}

describe('quotation draft defaults', () => {
  test('creates an independent, input-only draft for the supplied date', () => {
    const first = createDefaultQuotationDraft(new Date(2026, 8, 8))
    const second = createDefaultQuotationDraft('2026-09-09')

    assert.deepEqual(first, {
      title: '',
      customer: '',
      quotedBy: '',
      quoteDate: '2026-09-08',
      globalDiscount: null,
      priceBasis: { type: 'raw' },
      selectedModelIds: [],
      providerOverrides: {},
    })
    assert.equal(second.quoteDate, '2026-09-09')
    assert.notEqual(first.selectedModelIds, second.selectedModelIds)
    assert.notEqual(first.providerOverrides, second.providerOverrides)
  })

  test('uses an empty date instead of throwing for an invalid date', () => {
    assert.equal(createDefaultQuotationDraft(new Date('invalid')).quoteDate, '')
    assert.equal(createDefaultQuotationDraft('not-a-date').quoteDate, '')
    assert.equal(createDefaultQuotationDraft('2026-02-30').quoteDate, '')
  })
})

describe('versioned quotation draft persistence', () => {
  test('round-trips a valid draft through the namespaced v1 envelope', () => {
    const storage = memoryStorage()
    const draft = quotationDraft()

    assert.equal(saveQuotationDraft(storage, draft), true)
    assert.deepEqual(loadQuotationDraft(storage), draft)
    assert.deepEqual(JSON.parse(storage.values.get(STORAGE_KEY) ?? ''), {
      version: 1,
      draft,
    })
  })

  test('stores only user inputs and identifiers, never attached catalog data or credentials', () => {
    const storage = memoryStorage()
    const draft = {
      ...quotationDraft(),
      livePrices: [{ modelId: 'model-a', quoteAmount: 1.23 }],
      apiKey: 'secret',
      providerOverrides: {
        '10': {
          discount: 5,
          note: 'Partner terms',
          credential: 'provider-secret',
          livePrice: { amount: 1.23 },
        },
      },
    } as QuotationDraft

    assert.equal(saveQuotationDraft(storage, draft), true)
    const persisted = storage.values.get(STORAGE_KEY) ?? ''
    assert.equal(persisted.includes('secret'), false)
    assert.equal(persisted.includes('livePrice'), false)
    assert.deepEqual(loadQuotationDraft(storage), quotationDraft())
  })

  test('rejects corrupt JSON, a different schema version, and invalid field types', () => {
    const invalidValues = [
      '{not-json',
      JSON.stringify({ version: 2, draft: quotationDraft() }),
      JSON.stringify({
        version: 1,
        draft: quotationDraft({
          selectedModelIds: [42] as unknown as string[],
        }),
      }),
      JSON.stringify({
        version: 1,
        draft: quotationDraft({ globalDiscount: 0 }),
      }),
      JSON.stringify({
        version: 1,
        draft: quotationDraft({ quoteDate: '2026-02-30' }),
      }),
    ]

    for (const value of invalidValues) {
      assert.equal(
        loadQuotationDraft(memoryStorage({ [STORAGE_KEY]: value })),
        null
      )
    }
  })

  test('rejects oversized raw payloads and oversized user fields on read and write', () => {
    const oversizedPayload = JSON.stringify({
      version: 1,
      draft: quotationDraft({ title: 'x'.repeat(300_000) }),
    })
    assert.equal(
      loadQuotationDraft(memoryStorage({ [STORAGE_KEY]: oversizedPayload })),
      null
    )

    const storage = memoryStorage()
    assert.equal(
      saveQuotationDraft(
        storage,
        quotationDraft({
          providerOverrides: {
            '10': { discount: 5, note: 'x'.repeat(70_000) },
          },
        })
      ),
      false
    )
    assert.equal(storage.values.has(STORAGE_KEY), false)
  })

  test('contains storage access failures and reports failed writes', () => {
    const blocked = {
      getItem: () => {
        throw new Error('blocked read')
      },
      setItem: () => {
        throw new Error('blocked write')
      },
      removeItem: () => {
        throw new Error('blocked remove')
      },
    }

    assert.equal(loadQuotationDraft(blocked), null)
    assert.equal(saveQuotationDraft(blocked, quotationDraft()), false)
    assert.doesNotThrow(() => clearQuotationDraft(blocked))
  })

  test('clears only the quotation draft key', () => {
    const storage = memoryStorage({
      [STORAGE_KEY]: JSON.stringify({ version: 1, draft: quotationDraft() }),
      unrelated: 'keep-me',
    })

    clearQuotationDraft(storage)

    assert.equal(storage.values.has(STORAGE_KEY), false)
    assert.equal(storage.values.get('unrelated'), 'keep-me')
  })
})
