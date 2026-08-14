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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  DEFAULT_MODEL_METADATA_SYNC_MODE,
  MODEL_METADATA_SYNC_MODES,
} from '../model-metadata-sync-mode'

describe('model metadata sync modes', () => {
  test('defaults to local-first and exposes both API values once', () => {
    assert.equal(DEFAULT_MODEL_METADATA_SYNC_MODE, 'local_first')
    assert.deepEqual(
      MODEL_METADATA_SYNC_MODES.map((option) => option.value),
      ['local_first', 'models_dev_first']
    )
  })

  test('marks only models.dev-first as destructive', () => {
    assert.deepEqual(
      MODEL_METADATA_SYNC_MODES.filter((option) => option.destructive).map(
        (option) => option.value
      ),
      ['models_dev_first']
    )
  })
})
