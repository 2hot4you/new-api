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
import { useCallback, useEffect, useMemo, useState } from 'react'

import { DEFAULT_PRICING_PAGE_SIZE } from '../constants'
import type { PricingModel } from '../types'

export function useInfiniteModels(
  models: PricingModel[],
  batchSize = DEFAULT_PRICING_PAGE_SIZE
) {
  const safeBatchSize = Math.max(1, batchSize)
  const [visibleCount, setVisibleCount] = useState(safeBatchSize)
  const [sentinel, setSentinel] = useState<HTMLDivElement | null>(null)
  const supportsIntersectionObserver =
    typeof window !== 'undefined' &&
    typeof window.IntersectionObserver === 'function'

  useEffect(() => {
    setVisibleCount(safeBatchSize)
  }, [models, safeBatchSize])

  useEffect(() => {
    if (!supportsIntersectionObserver) {
      setVisibleCount(models.length)
      return
    }

    if (!sentinel || visibleCount >= models.length) return

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) return
        setVisibleCount((current) =>
          Math.min(current + safeBatchSize, models.length)
        )
      },
      { rootMargin: '600px 0px' }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [
    models.length,
    safeBatchSize,
    sentinel,
    supportsIntersectionObserver,
    visibleCount,
  ])

  const visibleModels = useMemo(
    () => models.slice(0, visibleCount),
    [models, visibleCount]
  )

  const sentinelRef = useCallback((node: HTMLDivElement | null) => {
    setSentinel(node)
  }, [])

  return {
    visibleModels,
    hasMore: supportsIntersectionObserver && visibleCount < models.length,
    sentinelRef,
  }
}
