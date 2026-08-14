import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { MOLII_FAVICON_URL, resolveFaviconUrl } from '../dom-utils'

describe('favicon branding', () => {
  test('uses the dedicated Molii favicon for the default system logo', () => {
    assert.equal(resolveFaviconUrl('/logo.png'), MOLII_FAVICON_URL)
  })

  test('recognizes an absolute default system logo URL', () => {
    assert.equal(
      resolveFaviconUrl('http://127.0.0.1:3000/logo.png'),
      MOLII_FAVICON_URL
    )
  })

  test('preserves a custom brand favicon URL', () => {
    assert.equal(
      resolveFaviconUrl('https://cdn.example.com/custom-brand.png'),
      'https://cdn.example.com/custom-brand.png'
    )
  })
})
