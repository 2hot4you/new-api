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
import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { PricingModel } from '../../types'
import { useInfiniteModels } from '../use-infinite-models'

function createModels(count: number, prefix = 'model'): PricingModel[] {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    model_name: `${prefix}-${index + 1}`,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: [],
  }))
}

function InfiniteModelsHarness(props: {
  models: PricingModel[]
  batchSize?: number
}) {
  const { visibleModels, hasMore, sentinelRef } = useInfiniteModels(
    props.models,
    props.batchSize
  )

  return (
    <div>
      <output data-testid='visible-count'>{visibleModels.length}</output>
      <output data-testid='has-more'>{String(hasMore)}</output>
      <div ref={sentinelRef} data-testid='sentinel' />
    </div>
  )
}

describe('useInfiniteModels', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('reveals models in batches when the sentinel intersects', () => {
    let callback: IntersectionObserverCallback | undefined
    const observe = vi.fn()
    const disconnect = vi.fn()

    class IntersectionObserverMock {
      constructor(nextCallback: IntersectionObserverCallback) {
        callback = nextCallback
      }

      observe = observe
      disconnect = disconnect
      unobserve = vi.fn()
      takeRecords = vi.fn(() => [])
      root = null
      rootMargin = '600px 0px'
      thresholds = [0]
    }

    vi.stubGlobal('IntersectionObserver', IntersectionObserverMock)

    const { unmount } = render(
      <InfiniteModelsHarness models={createModels(45)} batchSize={20} />
    )

    expect(screen.getByTestId('visible-count')).toHaveTextContent('20')
    expect(screen.getByTestId('has-more')).toHaveTextContent('true')
    expect(observe).toHaveBeenCalledWith(screen.getByTestId('sentinel'))

    act(() => {
      callback?.(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver
      )
    })
    expect(screen.getByTestId('visible-count')).toHaveTextContent('40')

    act(() => {
      callback?.(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver
      )
    })
    expect(screen.getByTestId('visible-count')).toHaveTextContent('45')
    expect(screen.getByTestId('has-more')).toHaveTextContent('false')

    unmount()
    expect(disconnect).toHaveBeenCalled()
  })

  it('resets to the first batch when filtering changes the model collection', async () => {
    let callback: IntersectionObserverCallback | undefined

    class IntersectionObserverMock {
      observe = vi.fn()
      disconnect = vi.fn()
      unobserve = vi.fn()
      takeRecords = vi.fn(() => [])
      root = null
      rootMargin = '600px 0px'
      thresholds = [0]

      constructor(nextCallback: IntersectionObserverCallback) {
        callback = nextCallback
      }
    }

    vi.stubGlobal('IntersectionObserver', IntersectionObserverMock)

    const { rerender } = render(
      <InfiniteModelsHarness models={createModels(45)} batchSize={20} />
    )
    expect(screen.getByTestId('visible-count')).toHaveTextContent('20')

    act(() => {
      callback?.(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver
      )
    })
    expect(screen.getByTestId('visible-count')).toHaveTextContent('40')

    rerender(
      <InfiniteModelsHarness
        models={createModels(30, 'filtered')}
        batchSize={20}
      />
    )

    await waitFor(() =>
      expect(screen.getByTestId('visible-count')).toHaveTextContent('20')
    )
  })

  it('shows every model when IntersectionObserver is unavailable', async () => {
    vi.stubGlobal('IntersectionObserver', undefined)

    render(<InfiniteModelsHarness models={createModels(45)} batchSize={20} />)

    await waitFor(() =>
      expect(screen.getByTestId('visible-count')).toHaveTextContent('45')
    )
    expect(screen.getByTestId('has-more')).toHaveTextContent('false')
  })
})
