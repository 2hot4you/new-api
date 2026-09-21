import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http'
import type { AddressInfo } from 'node:net'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { Window } from 'happy-dom'
import { describe, test } from 'vitest'

import type { SiteBrand } from '../../../build/site-brand'
import { applyFaviconToDom, resolveFaviconUrl } from '../dom-utils'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

function readPngSize(path: string) {
  const buffer = readFileSync(path)
  assert.equal(buffer.subarray(1, 4).toString('ascii'), 'PNG')
  return {
    width: buffer.readUInt32BE(16),
    height: buffer.readUInt32BE(20),
  }
}

function sha256(path: string) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

function withFaviconDom(run: (domWindow: Window) => void) {
  const previousWindow = globalThis.window
  const previousDocument = globalThis.document
  const domWindow = new Window({ url: 'https://claudeye.test/' })
  try {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: domWindow,
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: domWindow.document,
    })
    run(domWindow)
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
}

async function withImageLoadingDom(
  url: string,
  run: (domWindow: Window) => Promise<void>
) {
  const previousWindow = globalThis.window
  const previousDocument = globalThis.document
  const domWindow = new Window({
    url,
    settings: { enableImageFileLoading: true },
  })
  const previousConsoleError = domWindow.console.error
  domWindow.console.error = () => undefined
  try {
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: domWindow,
    })
    Object.defineProperty(globalThis, 'document', {
      configurable: true,
      value: domWindow.document,
    })
    await run(domWindow)
  } finally {
    domWindow.console.error = previousConsoleError
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
}

async function listen(
  handler: (request: IncomingMessage, response: ServerResponse) => void
): Promise<{ close: () => Promise<void>; origin: string }> {
  const server = createServer(handler)
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  const address = server.address() as AddressInfo
  return {
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
    origin: `http://127.0.0.1:${address.port}`,
  }
}

const VALID_SVG =
  '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="#ff2d55"/></svg>'

const CLAUDEYE_BRAND: SiteBrand = {
  id: 'claudeye',
  title: 'Claudeye',
  description: 'Claudeye model gateway.',
  logo: '/logo.png',
  favicon: '/api/branding/claudeye/favicon.svg',
  faviconFallback: '/claudeye-static-favicon.png',
  appleTouchIcon: '/claudeye-apple-touch-icon.png',
  bannerBrand: 'Claudeye',
  defaultFont: 'sans',
}

describe('site favicon assets', () => {
  test('ships the approved neutral Claudeye wordmark fallback', () => {
    const wordmark = resolve(webRoot, 'public/claudeye-wordmark-neutral.png')
    const buffer = readFileSync(wordmark)

    assert.equal(
      sha256(wordmark),
      'f77944e3ea47969bf3774ca849648d478922d5e9414ac154f0818e92670b5d7d'
    )
    assert.deepEqual(readPngSize(wordmark), { width: 1995, height: 440 })
    assert.equal(buffer.readUInt8(25), 6)
  })

  test('keeps a dedicated Claudeye favicon unless the Logo is explicitly custom', () => {
    assert.equal(
      resolveFaviconUrl('/logo.png', CLAUDEYE_BRAND),
      '/api/branding/claudeye/favicon.svg'
    )
    assert.equal(
      resolveFaviconUrl('https://cdn.example/logo.png', CLAUDEYE_BRAND),
      'https://cdn.example/logo.png'
    )
    assert.equal(
      resolveFaviconUrl('https://cdn.example/custom.png', CLAUDEYE_BRAND),
      'https://cdn.example/custom.png'
    )
  })

  test('retains the initial static favicon while dynamic verification is pending', () => {
    withFaviconDom((domWindow) => {
      domWindow.document.head.innerHTML =
        '<link rel="icon" type="image/png" sizes="32x32" href="/claudeye-static-favicon.png">'

      applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)

      const fallback = domWindow.document.querySelector('link[rel~="icon"]')
      assert.ok(fallback)
      assert.equal(
        fallback.getAttribute('href'),
        CLAUDEYE_BRAND.faviconFallback
      )
      assert.equal(fallback.getAttribute('type'), 'image/png')
      assert.equal(fallback.getAttribute('sizes'), '32x32')

      applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)
      assert.equal(
        domWindow.document.querySelector('link[rel~="icon"]'),
        fallback
      )
    })
  })

  test('keeps the static PNG when the dynamic favicon probe receives a real HTTP failure', async () => {
    let dynamicRequests = 0
    const server = await listen((request, response) => {
      if (request.url === CLAUDEYE_BRAND.favicon) dynamicRequests += 1
      response.writeHead(503, { 'content-type': 'image/svg+xml' })
      response.end('unavailable')
    })

    try {
      await withImageLoadingDom(`${server.origin}/`, async (domWindow) => {
        domWindow.document.head.innerHTML =
          '<link rel="icon" type="image/png" sizes="32x32" href="/claudeye-static-favicon.png">'

        applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)

        const initial = domWindow.document.querySelectorAll('link[rel~="icon"]')
        assert.equal(initial.length, 1)
        assert.equal(
          initial[0].getAttribute('href'),
          CLAUDEYE_BRAND.faviconFallback
        )

        await domWindow.happyDOM.waitUntilComplete()

        const afterFailure =
          domWindow.document.querySelectorAll('link[rel~="icon"]')
        assert.equal(dynamicRequests, 1)
        assert.equal(afterFailure.length, 1)
        assert.equal(
          afterFailure[0].getAttribute('href'),
          CLAUDEYE_BRAND.faviconFallback
        )

        applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)
        await domWindow.happyDOM.waitUntilComplete()
        assert.equal(dynamicRequests, 1)
      })
    } finally {
      await server.close()
    }
  })

  test('adds a verified dynamic SVG while retaining the static PNG candidate', async () => {
    let dynamicRequests = 0
    const server = await listen((request, response) => {
      if (request.url === CLAUDEYE_BRAND.favicon) dynamicRequests += 1
      response.writeHead(200, { 'content-type': 'image/svg+xml' })
      response.end(VALID_SVG)
    })

    try {
      await withImageLoadingDom(`${server.origin}/`, async (domWindow) => {
        domWindow.document.head.innerHTML =
          '<link rel="icon" type="image/png" sizes="32x32" href="/claudeye-static-favicon.png">'

        applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)
        await domWindow.happyDOM.waitUntilComplete()

        const icons = domWindow.document.querySelectorAll('link[rel~="icon"]')
        assert.equal(dynamicRequests, 1)
        assert.equal(icons.length, 2)
        assert.equal(
          icons[0].getAttribute('href'),
          CLAUDEYE_BRAND.faviconFallback
        )
        assert.equal(icons[0].getAttribute('type'), 'image/png')
        assert.equal(icons[0].getAttribute('sizes'), '32x32')
        assert.equal(icons[1].getAttribute('href'), CLAUDEYE_BRAND.favicon)
        assert.equal(icons[1].getAttribute('type'), 'image/svg+xml')
        assert.equal(icons[1].getAttribute('sizes'), 'any')
      })
    } finally {
      await server.close()
    }
  })

  test('ignores a stale dynamic probe after a custom Logo becomes active', async () => {
    let pendingResponse: ServerResponse | undefined
    let requestStarted!: () => void
    const requestReceived = new Promise<void>((resolve) => {
      requestStarted = resolve
    })
    const server = await listen((_request, response) => {
      pendingResponse = response
      requestStarted()
    })

    try {
      await withImageLoadingDom(`${server.origin}/`, async (domWindow) => {
        domWindow.document.head.innerHTML =
          '<link rel="icon" type="image/png" sizes="32x32" href="/claudeye-static-favicon.png">'

        applyFaviconToDom(CLAUDEYE_BRAND.favicon, CLAUDEYE_BRAND)
        await requestReceived

        const customLogo = 'https://cdn.example/custom.png'
        applyFaviconToDom(customLogo, CLAUDEYE_BRAND)
        pendingResponse?.writeHead(200, { 'content-type': 'image/svg+xml' })
        pendingResponse?.end(VALID_SVG)
        await domWindow.happyDOM.waitUntilComplete()

        const icons = domWindow.document.querySelectorAll('link[rel~="icon"]')
        assert.equal(icons.length, 1)
        assert.equal(icons[0].getAttribute('href'), customLogo)
      })
    } finally {
      pendingResponse?.destroy()
      await server.close()
    }
  })

  test('keeps a custom Logo favicon outside the dynamic fallback lifecycle', () => {
    withFaviconDom((domWindow) => {
      domWindow.document.head.innerHTML =
        '<link rel="icon" type="image/png" sizes="32x32" href="/claudeye-static-favicon.png">'
      const customLogo = 'https://cdn.example/logo.png'

      applyFaviconToDom(customLogo, CLAUDEYE_BRAND)

      const custom = domWindow.document.querySelector('link[rel~="icon"]')
      assert.ok(custom)
      assert.equal(custom.getAttribute('href'), customLogo)
      assert.equal(custom.getAttribute('type'), null)
      assert.equal(custom.getAttribute('sizes'), null)

      custom.dispatchEvent(new domWindow.Event('error'))
      assert.equal(custom.getAttribute('href'), customLogo)
    })
  })

  test('templates browser and Apple icons from the active site profile', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(
      html,
      /type="image\/png"[\s\S]+sizes="32x32"[\s\S]+href="<%= siteBrand\.faviconFallback %>"/
    )
    assert.doesNotMatch(html, /href="<%= siteBrand\.favicon %>"/)
    assert.match(html, /href="<%= siteBrand\.appleTouchIcon %>"/)
    assert.doesNotMatch(html, /molii-favicon\.svg/)
    assert.doesNotMatch(html, /rel="icon"[^>]+href="\/logo\.png"/)
  })

  test('templates metadata before the application starts', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(html, /<title><%= siteBrand\.title %><\/title>/)
    assert.match(html, /content="<%= siteBrand\.title %>"/)
    assert.match(html, /content="<%= siteBrand\.description %>"/)
    assert.doesNotMatch(html, /<title>Molii Gateway<\/title>/)
    assert.doesNotMatch(html, /<title>New API<\/title>/)
  })

  test('uses the full-size pink Molii mark across favicon formats', () => {
    assert.equal(
      sha256(resolve(webRoot, 'public/molii-favicon-32.png')),
      '517fff7fbf2ad18a4337ba841006484d2c863f3e3c5aafd558dac44d4f89beb3'
    )
    assert.equal(
      sha256(resolve(webRoot, 'public/apple-touch-icon.png')),
      '390c79fd1ef7072f2c2067d7543e7761613897f4bab3dae2d9ea671ab2bb1671'
    )
    assert.equal(
      sha256(resolve(webRoot, 'public/favicon.ico')),
      '7b5c04aa91ae77b8dbf3a1d77d746d5280c2bef86ba6f11dcfcf23afbde8776f'
    )
  })

  test('ships the favicon files at their declared sizes', () => {
    const favicon = resolve(webRoot, 'public/molii-favicon-32.png')
    const appleIcon = resolve(webRoot, 'public/apple-touch-icon.png')

    assert.deepEqual(readPngSize(favicon), { width: 32, height: 32 })
    assert.deepEqual(readPngSize(appleIcon), { width: 180, height: 180 })
  })

  test('keeps the legacy favicon.ico fallback on the Molii mark', () => {
    const faviconIco = readFileSync(resolve(webRoot, 'public/favicon.ico'))

    assert.equal(faviconIco.readUInt16LE(0), 0)
    assert.equal(faviconIco.readUInt16LE(2), 1)
    assert.equal(faviconIco.readUInt16LE(4), 1)
    assert.equal(faviconIco.readUInt8(6), 32)
    assert.equal(faviconIco.readUInt8(7), 32)
    assert.equal(faviconIco.readUInt32LE(18), 22)
    assert.equal(faviconIco.readUInt32LE(22), 40)
    assert.equal(faviconIco.readInt32LE(26), 32)
    assert.equal(faviconIco.readInt32LE(30), 64)
  })
})
