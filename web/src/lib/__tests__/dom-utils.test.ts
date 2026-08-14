import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { Window } from 'happy-dom'

import {
  MOLII_FAVICON_URL,
  applySystemFaviconToDom,
  resolveFaviconUrl,
} from '../dom-utils'

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

  test('replaces a stale origin favicon when status has no logo', () => {
    const previousWindow = globalThis.window
    const previousDocument = globalThis.document
    const domWindow = new Window({ url: 'http://127.0.0.1:3000/' })
    try {
      Object.defineProperty(globalThis, 'window', {
        configurable: true,
        value: domWindow,
      })
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: domWindow.document,
      })
      domWindow.document.head.innerHTML =
        '<link rel="icon" href="/stale-origin-favicon.png">'

      applySystemFaviconToDom('')

      const icons = domWindow.document.querySelectorAll('link[rel~="icon"]')
      assert.equal(icons.length, 1)
      assert.equal(icons[0].getAttribute('href'), MOLII_FAVICON_URL)
    } finally {
      Object.defineProperty(globalThis, 'window', {
        configurable: true,
        value: previousWindow,
      })
      Object.defineProperty(globalThis, 'document', {
        configurable: true,
        value: previousDocument,
      })
      domWindow.close()
    }
  })
})
