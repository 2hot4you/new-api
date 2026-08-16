/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { test } from 'node:test'

const modelsRoot = join(import.meta.dirname, '../..')

test('models metadata uses the local editor without external sync controls', () => {
  const sources = [
    'api.ts',
    'components/models-primary-buttons.tsx',
    'components/models-dialogs.tsx',
    'components/models-provider.tsx',
  ].map((file) => readFileSync(join(modelsRoot, file), 'utf8'))

  for (const source of sources) {
    assert.doesNotMatch(source, /models\.dev/i)
    assert.doesNotMatch(source, /sync_upstream/)
    assert.doesNotMatch(source, /sync-wizard/)
  }
})
