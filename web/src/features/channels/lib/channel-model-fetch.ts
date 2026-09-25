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
import { CHANNEL_TYPE_BYTEDANCE_SEEDANCE } from '../constants'

// Saved-key preview is only supported by Advanced Custom's dedicated backend
// endpoint. Seedance edits must be saved before the persisted-channel fetch.
export function mustSaveBeforeModelFetch(
  savedType: number | undefined,
  currentType: number,
  hasUnsavedChanges: boolean,
  values?: {
    savedBaseURL?: string | null
    savedModels?: string
    baseURL?: string
    models?: string
    key?: string
  }
): boolean {
  if (savedType == null) return false
  // Programmatic model-picker updates may not set react-hook-form's isDirty.
  const discoveryFieldsChanged =
    values != null &&
    ((values.baseURL ?? '').trim() !== (values.savedBaseURL ?? '').trim() ||
      (values.models ?? '') !== (values.savedModels ?? '') ||
      Boolean(values.key?.trim()))
  return (
    savedType !== currentType ||
    (currentType === CHANNEL_TYPE_BYTEDANCE_SEEDANCE &&
      (hasUnsavedChanges || discoveryFieldsChanged))
  )
}
