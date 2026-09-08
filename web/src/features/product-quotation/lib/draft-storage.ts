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

import type { QuotationDraft } from '../types'

const DRAFT_STORAGE_KEY = 'new-api:product-quotation:draft:v1'
const DRAFT_SCHEMA_VERSION = 1
const MAX_PERSISTED_BYTES = 256_000

function isValidDraftDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return (
    Number.isFinite(parsed.getTime()) &&
    parsed.toISOString().slice(0, 10) === value
  )
}

export interface QuotationDraftStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
}

const discountSchema = z.number().finite().positive().max(10).nullable()

const priceBasisSchema = z.discriminatedUnion('type', [
  z.object({ type: z.literal('raw') }),
  z.object({
    type: z.literal('group'),
    group: z.string().min(1).max(128),
  }),
])

const providerOverrideSchema = z.object({
  discount: discountSchema,
  note: z.string().max(50_000),
})

const providerOverridesSchema = z
  .record(z.string().min(1).max(128), providerOverrideSchema)
  .refine((overrides) => Object.keys(overrides).length <= 200)

const quotationDraftSchema = z.object({
  title: z.string().max(240),
  customer: z.string().max(240),
  quotedBy: z.string().max(200),
  quoteDate: z
    .string()
    .max(10)
    .refine((date) => date === '' || isValidDraftDate(date)),
  globalDiscount: discountSchema,
  priceBasis: priceBasisSchema,
  selectedModelIds: z.array(z.string().min(1).max(512)).max(1_000),
  providerOverrides: providerOverridesSchema,
})

const quotationDraftEnvelopeSchema = z.object({
  version: z.literal(DRAFT_SCHEMA_VERSION),
  draft: quotationDraftSchema,
})

function persistedByteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength
}

function formatDraftDate(today: Date | string): string {
  if (typeof today === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(today)) {
    return isValidDraftDate(today) ? today : ''
  }

  const date = typeof today === 'string' ? new Date(today) : today
  if (!Number.isFinite(date.getTime())) return ''

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function createDefaultQuotationDraft(
  today: Date | string
): QuotationDraft {
  return {
    title: '',
    customer: '',
    quotedBy: '',
    quoteDate: formatDraftDate(today),
    globalDiscount: null,
    priceBasis: { type: 'raw' },
    selectedModelIds: [],
    providerOverrides: {},
  }
}

export function loadQuotationDraft(
  storage: QuotationDraftStorage | null | undefined
): QuotationDraft | null {
  if (!storage) return null

  try {
    const persisted = storage.getItem(DRAFT_STORAGE_KEY)
    if (!persisted || persistedByteLength(persisted) > MAX_PERSISTED_BYTES) {
      return null
    }

    const parsed = quotationDraftEnvelopeSchema.safeParse(
      JSON.parse(persisted) as unknown
    )
    return parsed.success ? parsed.data.draft : null
  } catch {
    return null
  }
}

export function saveQuotationDraft(
  storage: QuotationDraftStorage | null | undefined,
  draft: QuotationDraft
): boolean {
  if (!storage) return false

  const parsed = quotationDraftSchema.safeParse(draft)
  if (!parsed.success) return false

  try {
    const persisted = JSON.stringify({
      version: DRAFT_SCHEMA_VERSION,
      draft: parsed.data,
    })
    if (persistedByteLength(persisted) > MAX_PERSISTED_BYTES) return false

    storage.setItem(DRAFT_STORAGE_KEY, persisted)
    return true
  } catch {
    return false
  }
}

export function clearQuotationDraft(
  storage: QuotationDraftStorage | null | undefined
): void {
  if (!storage) return

  try {
    storage.removeItem(DRAFT_STORAGE_KEY)
  } catch {
    // Storage can be unavailable or disabled; clearing remains best-effort.
  }
}
