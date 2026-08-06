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

import { TASK_PLATFORMS } from '../../../constants'
import { shouldShowVideoTechnicalMetadata } from '../video-preview-metadata'

describe('video preview metadata visibility', () => {
  test('hides dimensions and frame rate for Molii Grok tasks', () => {
    assert.equal(
      shouldShowVideoTechnicalMetadata(TASK_PLATFORMS.MOLII_GROK),
      false
    )
  })

  test('preserves dimensions and frame rate for Seedance and other platforms', () => {
    assert.equal(shouldShowVideoTechnicalMetadata(TASK_PLATFORMS.STARAI), true)
    assert.equal(shouldShowVideoTechnicalMetadata('runway'), true)
  })
})
