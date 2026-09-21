import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, test } from 'vitest'

import type { SiteBrand } from '../../../build/site-brand'
import { resolveFaviconUrl } from '../dom-utils'

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
    const brand: SiteBrand = {
      id: 'claudeye',
      title: 'Claudeye',
      description: 'Claudeye model gateway.',
      logo: '/logo.png',
      favicon: '/api/branding/claudeye/favicon.svg',
      appleTouchIcon: '/claudeye-apple-touch-icon.png',
      bannerBrand: 'Claudeye',
      defaultFont: 'sans',
    }

    assert.equal(
      resolveFaviconUrl('/logo.png', brand),
      '/api/branding/claudeye/favicon.svg'
    )
    assert.equal(
      resolveFaviconUrl('https://cdn.example/logo.png', brand),
      'https://cdn.example/logo.png'
    )
    assert.equal(
      resolveFaviconUrl('https://cdn.example/custom.png', brand),
      'https://cdn.example/custom.png'
    )
  })

  test('templates browser and Apple icons from the active site profile', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(html, /href="<%= siteBrand\.favicon %>"/)
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
