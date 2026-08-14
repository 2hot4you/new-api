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
import type { ModelMetadataSyncMode } from '../types'

export const DEFAULT_MODEL_METADATA_SYNC_MODE: ModelMetadataSyncMode =
  'local_first'

export const MODEL_METADATA_SYNC_MODES: ReadonlyArray<{
  value: ModelMetadataSyncMode
  titleKey: string
  descriptionKey: string
  destructive: boolean
}> = [
  {
    value: 'local_first',
    titleKey: 'Local metadata first',
    descriptionKey:
      'Keep current model metadata and fill only missing fields from models.dev.',
    destructive: false,
  },
  {
    value: 'models_dev_first',
    titleKey: 'models.dev metadata first',
    descriptionKey:
      'Replace existing model metadata with values supplied by models.dev.',
    destructive: true,
  },
]
