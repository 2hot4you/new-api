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
import { useSearch } from '@tanstack/react-router'
import { useMemo, useCallback, useState } from 'react'

import {
  FILTER_ALL,
  SORT_OPTIONS,
  QUOTA_TYPES,
  ENDPOINT_TYPES,
  DEFAULT_TOKEN_UNIT,
  VIEW_MODES,
  type ViewMode,
} from '../constants'
import { filterAndSortModels, extractAllTags } from '../lib/filters'
import type { ContextBucketId, ModelCategoryId } from '../lib/model-directory'
import type {
  Modality,
  ModelCapability,
  PricingModel,
  TokenUnit,
} from '../types'

type FilterState = {
  search?: string
  sort?: string
  vendor?: string
  group?: string
  quotaType?: string
  endpointType?: string
  tag?: string
  tokenUnit?: TokenUnit
  view?: ViewMode
  rechargePrice?: boolean
  category?: ModelCategoryId
  input?: Modality
  context?: ContextBucketId
  capability?: ModelCapability
}

function normalizeViewMode(value: unknown): ViewMode {
  if (value === VIEW_MODES.TABLE) {
    return VIEW_MODES.TABLE
  }
  return VIEW_MODES.CARD
}

export function useFilters(models: PricingModel[]) {
  const search = useSearch({ from: '/pricing/' })
  const [filterState, setFilterState] = useState<FilterState>(() => ({
    search: search.search,
    sort: search.sort,
    vendor: search.vendor,
    group: search.group,
    quotaType: search.quotaType,
    endpointType: search.endpointType,
    tag: search.tag,
    tokenUnit: search.tokenUnit,
    view: search.view,
    rechargePrice: search.rechargePrice,
    category: search.category as ModelCategoryId | undefined,
    input: search.input as Modality | undefined,
    context: search.context as ContextBucketId | undefined,
    capability: search.capability as ModelCapability | undefined,
  }))

  const searchInput = filterState.search || ''
  const sortBy = filterState.sort || SORT_OPTIONS.RELEASE_DATE
  const vendorFilter = filterState.vendor || FILTER_ALL
  const groupFilter = filterState.group || FILTER_ALL
  const quotaTypeFilter = filterState.quotaType || QUOTA_TYPES.ALL
  const endpointTypeFilter = filterState.endpointType || ENDPOINT_TYPES.ALL
  const tagFilter = filterState.tag || FILTER_ALL
  const tokenUnit: TokenUnit =
    filterState.tokenUnit === 'K' ? 'K' : DEFAULT_TOKEN_UNIT
  const viewMode = normalizeViewMode(filterState.view)
  const showRechargePrice = filterState.rechargePrice === true
  const categoryFilter = filterState.category || FILTER_ALL
  const inputModalityFilter = filterState.input || FILTER_ALL
  const contextFilter = filterState.context || FILTER_ALL
  const capabilityFilter = filterState.capability || FILTER_ALL

  const updateFilters = useCallback((updates: Record<string, unknown>) => {
    setFilterState((prev) => {
      const next: Record<string, unknown> = { ...prev, ...updates }
      for (const key of Object.keys(next)) {
        if (next[key] === undefined || next[key] === null) {
          delete next[key]
        }
      }
      return next as FilterState
    })
  }, [])

  const setSearchInput = useCallback(
    (v: string) => updateFilters({ search: v || undefined }),
    [updateFilters]
  )
  const setSortBy = useCallback(
    (v: string) =>
      updateFilters({
        sort: v === SORT_OPTIONS.RELEASE_DATE ? undefined : v,
      }),
    [updateFilters]
  )
  const setVendorFilter = useCallback(
    (v: string) => updateFilters({ vendor: v === FILTER_ALL ? undefined : v }),
    [updateFilters]
  )
  const setGroupFilter = useCallback(
    (v: string) => updateFilters({ group: v === FILTER_ALL ? undefined : v }),
    [updateFilters]
  )
  const setQuotaTypeFilter = useCallback(
    (v: string) =>
      updateFilters({ quotaType: v === QUOTA_TYPES.ALL ? undefined : v }),
    [updateFilters]
  )
  const setEndpointTypeFilter = useCallback(
    (v: string) =>
      updateFilters({
        endpointType: v === ENDPOINT_TYPES.ALL ? undefined : v,
      }),
    [updateFilters]
  )
  const setTagFilter = useCallback(
    (v: string) => updateFilters({ tag: v === FILTER_ALL ? undefined : v }),
    [updateFilters]
  )
  const setTokenUnit = useCallback(
    (v: TokenUnit) =>
      updateFilters({ tokenUnit: v === DEFAULT_TOKEN_UNIT ? undefined : v }),
    [updateFilters]
  )
  const setViewMode = useCallback(
    (v: ViewMode) =>
      updateFilters({ view: v === VIEW_MODES.CARD ? undefined : v }),
    [updateFilters]
  )
  const setShowRechargePrice = useCallback(
    (v: boolean) => updateFilters({ rechargePrice: v || undefined }),
    [updateFilters]
  )
  const setCategoryFilter = useCallback(
    (v: string) =>
      updateFilters({
        category: v === FILTER_ALL ? undefined : (v as ModelCategoryId),
      }),
    [updateFilters]
  )
  const setInputModalityFilter = useCallback(
    (v: string) =>
      updateFilters({
        input: v === FILTER_ALL ? undefined : (v as Modality),
      }),
    [updateFilters]
  )
  const setContextFilter = useCallback(
    (v: string) =>
      updateFilters({
        context: v === FILTER_ALL ? undefined : (v as ContextBucketId),
      }),
    [updateFilters]
  )
  const setCapabilityFilter = useCallback(
    (v: string) =>
      updateFilters({
        capability: v === FILTER_ALL ? undefined : (v as ModelCapability),
      }),
    [updateFilters]
  )

  const availableTags = useMemo(() => {
    if (!models || models.length === 0) return []
    return extractAllTags(models)
  }, [models])

  const filteredModels = useMemo(() => {
    if (!models || models.length === 0) return []

    return filterAndSortModels(models, {
      search: searchInput,
      vendor: vendorFilter,
      group: groupFilter,
      quotaType: quotaTypeFilter,
      endpointType: endpointTypeFilter,
      tag: tagFilter,
      sortBy,
      category:
        categoryFilter === FILTER_ALL
          ? undefined
          : (categoryFilter as ModelCategoryId),
      inputModality:
        inputModalityFilter === FILTER_ALL
          ? undefined
          : (inputModalityFilter as Modality),
      contextBucket:
        contextFilter === FILTER_ALL
          ? undefined
          : (contextFilter as ContextBucketId),
      capability:
        capabilityFilter === FILTER_ALL
          ? undefined
          : (capabilityFilter as ModelCapability),
    })
  }, [
    models,
    searchInput,
    vendorFilter,
    groupFilter,
    quotaTypeFilter,
    endpointTypeFilter,
    tagFilter,
    sortBy,
    categoryFilter,
    inputModalityFilter,
    contextFilter,
    capabilityFilter,
  ])

  const hasActiveFilters = useMemo(
    () =>
      vendorFilter !== FILTER_ALL ||
      groupFilter !== FILTER_ALL ||
      quotaTypeFilter !== QUOTA_TYPES.ALL ||
      endpointTypeFilter !== ENDPOINT_TYPES.ALL ||
      tagFilter !== FILTER_ALL ||
      categoryFilter !== FILTER_ALL ||
      inputModalityFilter !== FILTER_ALL ||
      contextFilter !== FILTER_ALL ||
      capabilityFilter !== FILTER_ALL,
    [
      vendorFilter,
      groupFilter,
      quotaTypeFilter,
      endpointTypeFilter,
      tagFilter,
      categoryFilter,
      inputModalityFilter,
      contextFilter,
      capabilityFilter,
    ]
  )

  const activeFilterCount = useMemo(
    () =>
      (vendorFilter !== FILTER_ALL ? 1 : 0) +
      (groupFilter !== FILTER_ALL ? 1 : 0) +
      (quotaTypeFilter !== QUOTA_TYPES.ALL ? 1 : 0) +
      (endpointTypeFilter !== ENDPOINT_TYPES.ALL ? 1 : 0) +
      (tagFilter !== FILTER_ALL ? 1 : 0) +
      (categoryFilter !== FILTER_ALL ? 1 : 0) +
      (inputModalityFilter !== FILTER_ALL ? 1 : 0) +
      (contextFilter !== FILTER_ALL ? 1 : 0) +
      (capabilityFilter !== FILTER_ALL ? 1 : 0),
    [
      vendorFilter,
      groupFilter,
      quotaTypeFilter,
      endpointTypeFilter,
      tagFilter,
      categoryFilter,
      inputModalityFilter,
      contextFilter,
      capabilityFilter,
    ]
  )

  const clearFilters = useCallback(() => {
    updateFilters({
      vendor: undefined,
      group: undefined,
      quotaType: undefined,
      endpointType: undefined,
      tag: undefined,
      category: undefined,
      input: undefined,
      context: undefined,
      capability: undefined,
    })
  }, [updateFilters])

  const clearSearch = useCallback(() => {
    updateFilters({ search: undefined })
  }, [updateFilters])

  return {
    searchInput,
    sortBy,
    vendorFilter,
    groupFilter,
    quotaTypeFilter,
    endpointTypeFilter,
    tagFilter,
    categoryFilter,
    inputModalityFilter,
    contextFilter,
    capabilityFilter,
    tokenUnit,
    viewMode,
    showRechargePrice,
    setSearchInput,
    setSortBy,
    setVendorFilter,
    setGroupFilter,
    setQuotaTypeFilter,
    setEndpointTypeFilter,
    setTagFilter,
    setCategoryFilter,
    setInputModalityFilter,
    setContextFilter,
    setCapabilityFilter,
    setTokenUnit,
    setViewMode,
    setShowRechargePrice,
    filteredModels,
    hasActiveFilters,
    activeFilterCount,
    availableTags,
    clearFilters,
    clearSearch,
  }
}
