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
import { useEffect, useMemo, useRef, useState } from 'react'

import { useMediaQuery } from '@/hooks/use-media-query'

const MAX_TYPING_DURATION_MS = 2800
const MAX_TYPING_INTERVAL_MS = 120
const HOLD_DURATION_MS = 700
const DELETE_INTERVAL_MS = 45

export type ModelPlaceholderPhase = 'typing' | 'holding' | 'deleting'

export interface ModelPlaceholderState {
  modelIndex: number
  phase: ModelPlaceholderPhase
  text: string
}

interface ModelPlaceholderSnapshot {
  key: string
  state: ModelPlaceholderState
}

export function normalizeModelIds(modelIds: string[]): string[] {
  return [...new Set(modelIds.map((modelId) => modelId.trim()).filter(Boolean))]
}

export function createModelPlaceholderState(): ModelPlaceholderState {
  return { modelIndex: 0, phase: 'typing', text: '' }
}

export function advanceModelPlaceholder(
  state: ModelPlaceholderState,
  modelIds: string[]
): ModelPlaceholderState {
  if (modelIds.length === 0) return createModelPlaceholderState()

  const modelIndex = state.modelIndex % modelIds.length
  const modelId = modelIds[modelIndex]

  if (state.phase === 'typing') {
    const text = modelId.slice(0, state.text.length + 1)
    return {
      modelIndex,
      phase: text.length >= modelId.length ? 'holding' : 'typing',
      text,
    }
  }

  if (state.phase === 'holding') {
    return { modelIndex, phase: 'deleting', text: state.text }
  }

  const text = state.text.slice(0, -1)
  if (text.length > 0) {
    return { modelIndex, phase: 'deleting', text }
  }

  return {
    modelIndex: (modelIndex + 1) % modelIds.length,
    phase: 'typing',
    text: '',
  }
}

export function getModelPlaceholderDelay(
  state: ModelPlaceholderState,
  modelIds: string[]
): number {
  if (state.phase === 'holding') return HOLD_DURATION_MS
  if (state.phase === 'deleting') return DELETE_INTERVAL_MS

  const modelId = modelIds[state.modelIndex % Math.max(modelIds.length, 1)]
  const modelLength = Math.max(modelId?.length ?? 0, 1)
  return Math.max(
    1,
    Math.min(
      MAX_TYPING_INTERVAL_MS,
      Math.floor(MAX_TYPING_DURATION_MS / modelLength)
    )
  )
}

export function useModelPlaceholderTypewriter(
  modelIds: string[],
  enabled: boolean
): string {
  const normalizedModelIds = useMemo(
    () => normalizeModelIds(modelIds),
    [modelIds]
  )
  const prefersReducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
  const wasEnabledRef = useRef(false)
  const runRef = useRef(0)
  const [snapshot, setSnapshot] = useState<ModelPlaceholderSnapshot>(() => ({
    key: '',
    state: createModelPlaceholderState(),
  }))

  if (enabled && !wasEnabledRef.current) runRef.current += 1
  wasEnabledRef.current = enabled

  const modelKey = normalizedModelIds.join('\u0000')
  const activeKey = `${runRef.current}:${modelKey}`
  const state =
    snapshot.key === activeKey
      ? snapshot.state
      : advanceModelPlaceholder(
          createModelPlaceholderState(),
          normalizedModelIds
        )

  useEffect(() => {
    if (!enabled || prefersReducedMotion || normalizedModelIds.length === 0) {
      return
    }

    const timeout = window.setTimeout(
      () => {
        setSnapshot({
          key: activeKey,
          state: advanceModelPlaceholder(state, normalizedModelIds),
        })
      },
      getModelPlaceholderDelay(state, normalizedModelIds)
    )

    return () => window.clearTimeout(timeout)
  }, [activeKey, enabled, normalizedModelIds, prefersReducedMotion, state])

  if (!enabled || normalizedModelIds.length === 0) return ''
  if (prefersReducedMotion) return normalizedModelIds[0]
  return state.text
}
