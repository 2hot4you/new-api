import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { describe, test } from 'node:test'
import { fileURLToPath } from 'node:url'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

function readPngSize(path: string) {
  const buffer = readFileSync(path)
  assert.equal(buffer.subarray(1, 4).toString('ascii'), 'PNG')
  return {
    width: buffer.readUInt32BE(16),
    height: buffer.readUInt32BE(20),
  }
}

describe('Molii favicon assets', () => {
  test('declares the versioned browser and Apple icons', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(html, /href="\/molii-favicon-32\.png\?v=3"/)
    assert.match(html, /href="\/apple-touch-icon\.png\?v=3"/)
    assert.doesNotMatch(html, /molii-favicon\.svg/)
    assert.doesNotMatch(html, /rel="icon"[^>]+href="\/logo\.png"/)
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
