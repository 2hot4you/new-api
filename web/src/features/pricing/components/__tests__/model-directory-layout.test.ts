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
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, test } from 'vitest'

const pricingSource = readFileSync(
  resolve(process.cwd(), 'src/features/pricing/index.tsx'),
  'utf8'
)
const gridSource = readFileSync(
  resolve(process.cwd(), 'src/features/pricing/components/model-card-grid.tsx'),
  'utf8'
)
const toolbarSource = readFileSync(
  resolve(process.cwd(), 'src/features/pricing/components/pricing-toolbar.tsx'),
  'utf8'
)

describe('model directory layout contract', () => {
  test('keeps a sticky desktop sidebar and a responsive continuous grid', () => {
    assert.match(pricingSource, /sticky top-16 hidden/)
    assert.match(pricingSource, /xl:grid-cols-\[250px_minmax\(0,1fr\)\]/)
    assert.match(gridSource, /data-model-directory-grid/)
    assert.match(gridSource, /grid-cols-1/)
    assert.match(gridSource, /md:grid-cols-2/)
    assert.match(gridSource, /xl:grid-cols-3/)
    assert.doesNotMatch(pricingSource, /data-model-vendor-section/)
    assert.doesNotMatch(pricingSource, /gradient/)
  })

  test('keeps mobile filters and independent detail navigation', () => {
    assert.match(toolbarSource, /xl:hidden/)
    assert.match(toolbarSource, /setMobileFiltersOpen\(true\)/)
    assert.match(pricingSource, /to: '\/pricing\/\$modelId'/)
    assert.match(pricingSource, /replace: true/)
  })

  test('provides translated directory labels in supported locales', () => {
    for (const locale of ['en', 'zh', 'zh-TW']) {
      const messages = JSON.parse(
        readFileSync(
          resolve(process.cwd(), `src/i18n/locales/${locale}.json`),
          'utf8'
        )
      ).translation as Record<string, string>

      for (const key of [
        'Model Categories',
        'Input Types',
        'Context Length',
        'Supported Capabilities',
        'Supported Protocols',
        'Related Models',
        'Model detail sections',
        'API Request',
        'Open Playground',
      ]) {
        assert.ok(messages[key], `${locale} is missing ${key}`)
      }
    }
  })
})
