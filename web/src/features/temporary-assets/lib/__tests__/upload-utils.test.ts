/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import {
  fileNameWithoutExtension,
  formatUploadSize,
  inferAssetType,
} from '../upload-utils'

describe('temporary asset upload helpers', () => {
  test('infers the asset type from MIME first and extension as fallback', () => {
    assert.equal(inferAssetType('clip.mov', 'video/quicktime'), 'video')
    assert.equal(inferAssetType('reference.HEIC', ''), undefined)
    assert.equal(
      inferAssetType('voice.mp3', 'application/octet-stream'),
      'audio'
    )
    assert.equal(inferAssetType('payload.zip', 'application/zip'), undefined)
  })

  test('uses the source filename as the default asset name', () => {
    assert.equal(fileNameWithoutExtension('opening.scene.mp4'), 'opening.scene')
    assert.equal(fileNameWithoutExtension('README'), 'README')
  })

  test('formats upload limits without abbreviating the actual byte contract', () => {
    assert.equal(formatUploadSize(30 * 1024 * 1024), '30.0 MB')
    assert.equal(formatUploadSize(512 * 1024), '512 KB')
  })
})
