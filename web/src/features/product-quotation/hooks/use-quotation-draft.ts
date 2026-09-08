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
import { useCallback, useEffect, useRef, useState } from 'react'

import {
  clearQuotationDraft,
  createDefaultQuotationDraft,
  loadQuotationDraft,
  saveQuotationDraft,
  type QuotationDraftStorage,
} from '../lib/draft-storage'
import type { QuotationDraft } from '../types'

export type QuotationDraftSaveStatus = 'saved' | 'unsaved'

function browserStorage(): QuotationDraftStorage | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

export function useQuotationDraft(today = new Date()) {
  const [draft, setDraft] = useState<QuotationDraft>(() => {
    const storage = browserStorage()
    return loadQuotationDraft(storage) ?? createDefaultQuotationDraft(today)
  })
  const [saveStatus, setSaveStatus] =
    useState<QuotationDraftSaveStatus>('saved')
  const suppressNextSave = useRef(false)

  useEffect(() => {
    if (suppressNextSave.current) {
      suppressNextSave.current = false
      setSaveStatus('saved')
      return
    }
    setSaveStatus(
      saveQuotationDraft(browserStorage(), draft) ? 'saved' : 'unsaved'
    )
  }, [draft])

  const updateDraft = useCallback(
    (
      update:
        | Partial<QuotationDraft>
        | ((current: QuotationDraft) => QuotationDraft)
    ) => {
      setDraft((current) =>
        typeof update === 'function'
          ? update(current)
          : { ...current, ...update }
      )
    },
    []
  )

  const clearDraft = useCallback(() => {
    clearQuotationDraft(browserStorage())
    suppressNextSave.current = true
    setDraft(createDefaultQuotationDraft(new Date()))
  }, [])

  return { draft, setDraft, updateDraft, clearDraft, saveStatus }
}
