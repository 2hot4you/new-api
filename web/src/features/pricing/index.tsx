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
import { useNavigate } from '@tanstack/react-router'
import { LoaderCircle } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'

import {
  LoadingSkeleton,
  EmptyState,
  PricingSidebar,
  PricingToolbar,
  ModelCardGrid,
  ModelCategoryBar,
  PricingTable,
} from './components'
import {
  DEFAULT_TOKEN_UNIT,
  ENDPOINT_TYPES,
  FILTER_ALL,
  QUOTA_TYPES,
} from './constants'
import { serializeSortOption, useFilters } from './hooks/use-filters'
import { useInfiniteModels } from './hooks/use-infinite-models'
import { usePricingData } from './hooks/use-pricing-data'
import { getModelCategories } from './lib/model-directory'
import { getPricingFilterGroups } from './lib/model-helpers'

export function Pricing() {
  const { t } = useTranslation()
  const navigate = useNavigate({ from: '/pricing/' })
  const [viewMode, setViewMode] = useState<'card' | 'table'>('card')

  const {
    models,
    vendors,
    groupRatio,
    usableGroup,
    groupMetadata,
    isLoading,
    priceRate,
    usdExchangeRate,
  } = usePricingData()

  const {
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
    setShowRechargePrice,
    filteredModels,
    hasActiveFilters,
    activeFilterCount,
    availableTags,
    clearFilters,
    clearSearch,
  } = useFilters(models || [])
  const { visibleModels, hasMore, sentinelRef } =
    useInfiniteModels(filteredModels)

  const directorySearch = useMemo(
    () => ({
      search: searchInput || undefined,
      sort: serializeSortOption(sortBy),
      vendor: vendorFilter === FILTER_ALL ? undefined : vendorFilter,
      group: groupFilter === FILTER_ALL ? undefined : groupFilter,
      quotaType:
        quotaTypeFilter === QUOTA_TYPES.ALL ? undefined : quotaTypeFilter,
      endpointType:
        endpointTypeFilter === ENDPOINT_TYPES.ALL
          ? undefined
          : endpointTypeFilter,
      tag: tagFilter === FILTER_ALL ? undefined : tagFilter,
      category: categoryFilter === FILTER_ALL ? undefined : categoryFilter,
      input:
        inputModalityFilter === FILTER_ALL ? undefined : inputModalityFilter,
      context: contextFilter === FILTER_ALL ? undefined : contextFilter,
      capability:
        capabilityFilter === FILTER_ALL ? undefined : capabilityFilter,
      tokenUnit: tokenUnit === DEFAULT_TOKEN_UNIT ? undefined : tokenUnit,
      rechargePrice: showRechargePrice || undefined,
    }),
    [
      capabilityFilter,
      categoryFilter,
      contextFilter,
      endpointTypeFilter,
      groupFilter,
      inputModalityFilter,
      quotaTypeFilter,
      searchInput,
      showRechargePrice,
      sortBy,
      tagFilter,
      tokenUnit,
      vendorFilter,
    ]
  )

  const handleModelClick = useCallback(
    (modelName: string) => {
      navigate({
        to: '/pricing/$modelId',
        params: { modelId: modelName },
        search: directorySearch,
      })
    },
    [directorySearch, navigate]
  )

  useEffect(() => {
    void navigate({
      to: '/pricing',
      search: directorySearch,
      replace: true,
    })
  }, [directorySearch, navigate])

  const availableGroups = useMemo(
    () => getPricingFilterGroups(usableGroup || {}),
    [usableGroup]
  )
  const categories = useMemo(() => getModelCategories(models || []), [models])

  const handleClearAll = useCallback(() => {
    clearFilters()
    clearSearch()
  }, [clearFilters, clearSearch])

  const renderPricingContent = () => {
    if (filteredModels.length === 0) {
      return (
        <EmptyState
          searchQuery={searchInput}
          hasActiveFilters={hasActiveFilters}
          onClearFilters={handleClearAll}
        />
      )
    }

    if (viewMode === 'table') {
      return (
        <PricingTable
          models={visibleModels}
          priceRate={priceRate}
          usdExchangeRate={usdExchangeRate}
          tokenUnit={tokenUnit}
          showRechargePrice={showRechargePrice}
          selectedGroup={groupFilter}
          onModelClick={handleModelClick}
        />
      )
    }
    return (
      <ModelCardGrid
        models={visibleModels}
        onModelClick={handleModelClick}
        priceRate={priceRate}
        usdExchangeRate={usdExchangeRate}
        tokenUnit={tokenUnit}
        showRechargePrice={showRechargePrice}
        selectedGroup={groupFilter}
      />
    )
  }

  if (isLoading) {
    return (
      <PublicLayout showMainContainer={false}>
        <div className='mx-auto w-full max-w-[1920px] px-3 pt-20 pb-8 sm:px-5'>
          <LoadingSkeleton viewMode='card' />
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-[1920px] pt-16 pb-8 sm:pt-20'>
        <div className='grid min-h-[calc(100dvh-5rem)] xl:grid-cols-[250px_minmax(0,1fr)]'>
          <PricingSidebar
            quotaTypeFilter={quotaTypeFilter}
            endpointTypeFilter={endpointTypeFilter}
            vendorFilter={vendorFilter}
            groupFilter={groupFilter}
            tagFilter={tagFilter}
            inputModalityFilter={inputModalityFilter}
            contextFilter={contextFilter}
            capabilityFilter={capabilityFilter}
            onQuotaTypeChange={setQuotaTypeFilter}
            onEndpointTypeChange={setEndpointTypeFilter}
            onVendorChange={setVendorFilter}
            onGroupChange={setGroupFilter}
            onTagChange={setTagFilter}
            onInputModalityChange={setInputModalityFilter}
            onContextChange={setContextFilter}
            onCapabilityChange={setCapabilityFilter}
            vendors={vendors || []}
            groups={availableGroups}
            groupRatios={groupRatio}
            groupMetadata={groupMetadata}
            tags={availableTags}
            models={models || []}
            hasActiveFilters={hasActiveFilters}
            onClearFilters={clearFilters}
            className='hover-scrollbar sticky top-16 hidden max-h-[calc(100dvh-4rem)] self-start overflow-y-auto overscroll-contain xl:block'
          />

          <main className='min-w-0 border-r'>
            <header className='border-b px-4 pt-6 pb-4 sm:px-6'>
              <div className='flex items-end justify-between gap-4'>
                <div>
                  <h1 className='text-2xl font-semibold tracking-tight sm:text-3xl'>
                    {t('Models')}
                  </h1>
                  <p className='text-muted-foreground mt-1 text-sm tabular-nums'>
                    {t('{{count}} models', { count: models?.length || 0 })}
                  </p>
                </div>
              </div>
              <ModelCategoryBar
                categories={categories}
                value={categoryFilter}
                onChange={setCategoryFilter}
                className='mt-4'
              />
            </header>

            <PricingToolbar
              filteredCount={filteredModels.length}
              totalCount={models?.length}
              sortBy={sortBy}
              onSortChange={setSortBy}
              tokenUnit={tokenUnit}
              onTokenUnitChange={setTokenUnit}
              showRechargePrice={showRechargePrice}
              onRechargePriceChange={setShowRechargePrice}
              viewMode={viewMode}
              onViewModeChange={setViewMode}
              quotaTypeFilter={quotaTypeFilter}
              endpointTypeFilter={endpointTypeFilter}
              vendorFilter={vendorFilter}
              groupFilter={groupFilter}
              tagFilter={tagFilter}
              inputModalityFilter={inputModalityFilter}
              contextFilter={contextFilter}
              capabilityFilter={capabilityFilter}
              onQuotaTypeChange={setQuotaTypeFilter}
              onEndpointTypeChange={setEndpointTypeFilter}
              onVendorChange={setVendorFilter}
              onGroupChange={setGroupFilter}
              onTagChange={setTagFilter}
              onInputModalityChange={setInputModalityFilter}
              onContextChange={setContextFilter}
              onCapabilityChange={setCapabilityFilter}
              searchValue={searchInput}
              onSearchChange={setSearchInput}
              onClearSearch={clearSearch}
              vendors={vendors || []}
              groups={availableGroups}
              groupRatios={groupRatio}
              groupMetadata={groupMetadata}
              tags={availableTags}
              models={models || []}
              hasActiveFilters={hasActiveFilters}
              activeFilterCount={activeFilterCount}
              onClearFilters={clearFilters}
            />

            <div className='min-w-0'>{renderPricingContent()}</div>
            <div
              ref={sentinelRef}
              data-pricing-infinite-scroll-sentinel='true'
              className='flex min-h-8 items-center justify-center py-2'
            >
              {hasMore && (
                <div
                  role='status'
                  aria-label={t('Loading')}
                  className='text-muted-foreground'
                >
                  <LoaderCircle className='size-4 animate-spin' />
                </div>
              )}
            </div>
          </main>
        </div>
      </PageTransition>
    </PublicLayout>
  )
}
