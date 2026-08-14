import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
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

function sha256(path: string) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

describe('Molii favicon assets', () => {
  test('declares the versioned browser and Apple icons', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(html, /href="\/molii-favicon-32\.png\?v=4"/)
    assert.match(html, /href="\/apple-touch-icon\.png\?v=4"/)
    assert.doesNotMatch(html, /molii-favicon\.svg/)
    assert.doesNotMatch(html, /rel="icon"[^>]+href="\/logo\.png"/)
  })

  test('uses Molii Gateway before the application starts', () => {
    const html = readFileSync(resolve(webRoot, 'index.html'), 'utf8')

    assert.match(html, /<title>Molii Gateway<\/title>/)
    assert.match(html, /<meta name="title" content="Molii Gateway" \/>/)
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
