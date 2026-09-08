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
import type { PricingModel, PricingVendor } from '@/features/pricing/types'

export type QuoteCurrency = 'USD' | 'CNY'

export type QuotationPriceBasis =
  | { type: 'raw' }
  | { type: 'group'; group: string }

export type QuoteProviderOverride = {
  discount: number | null
  note: string
}

export type QuotationDraft = {
  title: string
  customer: string
  quotedBy: string
  quoteDate: string
  globalDiscount: number | null
  priceBasis: QuotationPriceBasis
  selectedModelIds: string[]
  providerOverrides: Record<string, QuoteProviderOverride>
}

export type QuotePriceSource =
  | 'fixed_token'
  | 'request'
  | 'dynamic'
  | 'task_usage'
  | 'video'
  | 'grok'

export type QuotePriceStatus = 'ready' | 'needs_confirmation'

export type QuotePriceDimension = {
  key: string
  label: string
  sourceType: QuotePriceSource
  /** Unadjusted numeric value returned by the pricing catalog. */
  catalogAmount: number | null
  /** Catalog value after the explicitly selected raw/group price basis. */
  sourceAmount: number | null
  /** Source value after applying the quotation discount exactly once. */
  quoteAmount: number | null
  currency: QuoteCurrency
  unit: string
  condition: string | null
  status: QuotePriceStatus
}

export type QuoteModelSection = {
  modelId: string
  displayName: string
  available: boolean
  unavailableReason: 'missing' | 'group_unavailable' | null
  dimensions: QuotePriceDimension[]
}

export type QuoteProviderSection = {
  providerId: number | null
  providerName: string
  note: string
  discount: number | null
  discountCoefficient: number | null
  models: QuoteModelSection[]
}

export type QuotationSnapshotPriceBasis = {
  type: QuotationPriceBasis['type']
  group: string | null
  ratio: number | null
}

export type QuotationSnapshot = {
  title: string
  customer: string
  quotedBy: string
  quoteDate: string
  pricingVersion: string | null
  fetchedAt: string
  globalDiscount: number | null
  priceBasis: QuotationSnapshotPriceBasis
  providers: QuoteProviderSection[]
}

export type QuotationValidationErrorCode =
  | 'missing_title'
  | 'missing_quote_date'
  | 'no_models'
  | 'invalid_discount'
  | 'invalid_provider_discount'
  | 'invalid_price_basis'
  | 'model_unavailable'
  | 'price_needs_confirmation'

export type QuotationValidationError = {
  code: QuotationValidationErrorCode
  message: string
  providerId?: number | null
  modelId?: string
  dimensionKey?: string
}

export type QuotationValidation = {
  valid: boolean
  errors: QuotationValidationError[]
}

export type BuildQuotationSnapshotInput = {
  draft: QuotationDraft
  models: PricingModel[]
  vendors: PricingVendor[]
  groupRatio: Record<string, number>
  pricingVersion?: string | null
  fetchedAt: string | number | Date
}
