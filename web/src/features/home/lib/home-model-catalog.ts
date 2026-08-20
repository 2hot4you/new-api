/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  getModelCategory,
  sortModelsByReleaseDate,
} from '@/features/pricing/lib/model-directory'
import type { PricingModel, PricingVendor } from '@/features/pricing/types'

export interface HomeVendor {
  id: number | null
  name: string
  icon?: string
  description?: string
  modelCount: number
}

export interface HomeModelCatalog {
  modelCount: number
  vendorCount: number
  capabilityCategoryCount: number
  vendors: HomeVendor[]
  latestModels: PricingModel[]
}

function normalize(value: string | undefined): string {
  return value?.trim().toLocaleLowerCase() ?? ''
}

function isValidReleaseDate(value: string | undefined): boolean {
  if (!value?.trim()) return false
  return Number.isFinite(Date.parse(value))
}

function vendorKey(name: string): string {
  return normalize(name)
}

export function buildHomeModelCatalog(
  models: PricingModel[],
  vendors: PricingVendor[]
): HomeModelCatalog {
  const configuredVendors = new Map(
    vendors.map((vendor) => [vendor.id, vendor])
  )
  const vendorEntries = new Map<string, HomeVendor>()
  const categories = new Set<string>()

  for (const model of models) {
    categories.add(getModelCategory(model))

    const configuredVendor =
      model.vendor_id == null
        ? undefined
        : configuredVendors.get(model.vendor_id)
    const name = (configuredVendor?.name || model.vendor_name || '').trim()
    if (!name) continue

    const key = vendorKey(name)
    const existing = vendorEntries.get(key)
    if (existing) {
      existing.modelCount += 1
      if (!existing.icon) {
        existing.icon = configuredVendor?.icon || model.vendor_icon
      }
      if (!existing.description) {
        existing.description =
          configuredVendor?.description || model.vendor_description
      }
      continue
    }

    vendorEntries.set(key, {
      id: configuredVendor?.id ?? model.vendor_id ?? null,
      name,
      icon: configuredVendor?.icon || model.vendor_icon,
      description: configuredVendor?.description || model.vendor_description,
      modelCount: 1,
    })
  }

  const configuredVendorOrder = vendors
    .map((vendor, index) => ({ index, vendor }))
    .sort((left, right) => {
      const leftOrder = left.vendor.display_order
      const rightOrder = right.vendor.display_order
      if (leftOrder != null && rightOrder != null && leftOrder !== rightOrder) {
        return leftOrder - rightOrder
      }
      if (leftOrder != null && rightOrder == null) return -1
      if (leftOrder == null && rightOrder != null) return 1
      return left.index - right.index
    })
  const activeVendors: HomeVendor[] = []
  const configuredKeys = new Set<string>()

  for (const { vendor } of configuredVendorOrder) {
    const key = vendorKey(vendor.name)
    const entry = vendorEntries.get(key)
    if (!entry) continue
    activeVendors.push(entry)
    configuredKeys.add(key)
  }

  activeVendors.push(
    ...[...vendorEntries.entries()]
      .filter(([key]) => !configuredKeys.has(key))
      .map(([, vendor]) => vendor)
      .sort((left, right) => left.name.localeCompare(right.name))
  )
  const latestModels = sortModelsByReleaseDate(
    models.filter((model) => isValidReleaseDate(model.release_date))
  ).slice(0, 3)

  return {
    modelCount: models.length,
    vendorCount: activeVendors.length,
    capabilityCategoryCount: categories.size,
    vendors: activeVendors,
    latestModels,
  }
}

export function searchHomeModels(
  models: PricingModel[],
  query: string,
  limit?: number
): PricingModel[] {
  const normalizedQuery = normalize(query)
  if (!normalizedQuery || (limit != null && limit <= 0)) return []

  const matches = models.filter(
    (model) =>
      normalize(model.model_name).includes(normalizedQuery) ||
      normalize(model.vendor_name).includes(normalizedQuery)
  )
  return limit == null ? matches : matches.slice(0, limit)
}
