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
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import test from 'node:test'

const localesDir = path.resolve('src/i18n/locales')

async function loadTranslations(filename) {
  const raw = await fs.readFile(path.join(localesDir, filename), 'utf8')
  return JSON.parse(raw).translation ?? {}
}

test('all locale files expose the same translation keys', async () => {
  const filenames = (await fs.readdir(localesDir))
    .filter((filename) => filename.endsWith('.json'))
    .sort()
  const translations = Object.fromEntries(
    await Promise.all(
      filenames.map(async (filename) => [
        filename,
        await loadTranslations(filename),
      ])
    )
  )
  const allKeys = new Set(
    Object.values(translations).flatMap((translation) =>
      Object.keys(translation)
    )
  )
  const missingByLocale = Object.fromEntries(
    Object.entries(translations)
      .map(([filename, translation]) => [
        filename,
        [...allKeys].filter((key) => !(key in translation)),
      ])
      .filter(([, missing]) => missing.length > 0)
  )

  const summary = Object.fromEntries(
    Object.entries(missingByLocale).map(([filename, missing]) => [
      filename,
      { count: missing.length, sample: missing.slice(0, 5) },
    ])
  )
  assert.equal(
    Object.keys(missingByLocale).length,
    0,
    `Locale key sets differ: ${JSON.stringify(summary)}`
  )
})
