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
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import type { HomeModelCatalog } from '../../lib/home-model-catalog'

function Counter(props: { value: number; disabled?: boolean }) {
  const ref = useRef<HTMLSpanElement>(null)
  const startedRef = useRef(false)
  const format = useCallback(
    (value: number) => Math.round(value).toLocaleString(),
    []
  )

  useEffect(() => {
    const element = ref.current
    if (!element || props.disabled) return
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      element.textContent = format(props.value)
      return
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting || startedRef.current) return
        startedRef.current = true
        const start = performance.now()
        const animate = (now: number) => {
          const progress = Math.min((now - start) / 900, 1)
          const eased = 1 - Math.pow(1 - progress, 3)
          element.textContent = format(props.value * eased)
          if (progress < 1) requestAnimationFrame(animate)
        }
        requestAnimationFrame(animate)
        observer.unobserve(element)
      },
      { threshold: 0.45 }
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [format, props.disabled, props.value])

  return <span ref={ref}>{props.disabled ? '—' : '0'}</span>
}

interface StatsProps {
  catalog: HomeModelCatalog
  isLoading?: boolean
  hasError?: boolean
}

export function Stats(props: StatsProps) {
  const { t } = useTranslation()
  const unavailable = props.isLoading || props.hasError
  const stats = [
    { value: props.catalog.modelCount, label: t('Enabled models') },
    { value: props.catalog.vendorCount, label: t('Model providers') },
    {
      value: props.catalog.capabilityCategoryCount,
      label: t('Capability categories'),
    },
  ]

  return (
    <section className='border-border/55 border-y'>
      <div className='mx-auto grid max-w-7xl grid-cols-1 divide-y px-6 md:grid-cols-3 md:divide-x md:divide-y-0'>
        {stats.map((stat) => (
          <div key={stat.label} className='px-4 py-9 text-center md:py-12'>
            <div className='text-4xl font-semibold tracking-[-0.05em] tabular-nums md:text-6xl'>
              <Counter value={stat.value} disabled={unavailable} />
            </div>
            <div className='text-muted-foreground mt-2 text-sm'>
              {stat.label}
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
