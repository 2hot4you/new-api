/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, describe, test, vi } from 'vitest'

import type { TemporaryAsset } from '../../lib/asset-utils'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLImageElement',
  'HTMLMediaElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { AssetAudioPreview, AssetPreview, AssetPreviewLink } =
  await import('../asset-preview')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const cosImage: TemporaryAsset = {
  id: 'asset-cos-image',
  asset_type: 'image',
  source_kind: 'cos',
  preview_url: 'https://cos.example/expired-signature',
  status: 'SUCCESS',
  created_at: 100,
  expires_at: 1_000,
  verified_at: 100,
}

describe('temporary asset signed preview recovery', () => {
  after(() => domWindow.close())

  test('refreshes an expired COS image URL once before showing failure', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let refreshCount = 0

    await act(async () => {
      root.render(
        <AssetPreview
          item={cosImage}
          alt='Reference image'
          missingLabel='Missing preview'
          failedLabel='Preview failed'
          onRefreshPreview={async () => {
            refreshCount += 1
            return 'https://cos.example/fresh-signature'
          }}
        />
      )
    })

    const expiredImage = container.querySelector('img')
    assert.ok(expiredImage)
    await act(async () => expiredImage.dispatchEvent(new Event('error')))

    const refreshedImage = container.querySelector('img')
    assert.ok(refreshedImage)
    assert.equal(
      refreshedImage.getAttribute('src'),
      'https://cos.example/fresh-signature'
    )
    assert.equal(refreshCount, 1)

    await act(async () => refreshedImage.dispatchEvent(new Event('error')))
    assert.equal(refreshCount, 1)
    assert.match(container.textContent ?? '', /Preview failed/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not refresh third-party URL assets', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let refreshCount = 0

    await act(async () => {
      root.render(
        <AssetPreview
          item={{
            ...cosImage,
            id: 'asset-url-image',
            source_kind: 'url',
            source_url: 'https://third-party.example/image.png',
            preview_url: undefined,
          }}
          alt='Reference image'
          missingLabel='Missing preview'
          failedLabel='Preview failed'
          onRefreshPreview={async () => {
            refreshCount += 1
            return 'https://third-party.example/new.png'
          }}
        />
      )
    })

    const image = container.querySelector('img')
    assert.ok(image)
    await act(async () => image.dispatchEvent(new Event('error')))

    assert.equal(refreshCount, 0)
    assert.match(container.textContent ?? '', /Preview failed/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('refreshes an expired COS video URL once', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let refreshCount = 0

    await act(async () => {
      root.render(
        <AssetPreview
          item={{
            ...cosImage,
            id: 'asset-cos-video',
            asset_type: 'video',
          }}
          alt='Reference video'
          missingLabel='Missing preview'
          failedLabel='Preview failed'
          onRefreshPreview={async () => {
            refreshCount += 1
            return 'https://cos.example/fresh-video-signature'
          }}
        />
      )
    })

    const expiredVideo = container.querySelector('video')
    assert.ok(expiredVideo)
    await act(async () => expiredVideo.dispatchEvent(new Event('error')))

    const refreshedVideo = container.querySelector('video')
    assert.ok(refreshedVideo)
    assert.equal(
      refreshedVideo.getAttribute('src'),
      'https://cos.example/fresh-video-signature'
    )
    assert.equal(refreshCount, 1)

    await act(async () => refreshedVideo.dispatchEvent(new Event('error')))
    assert.equal(refreshCount, 1)
    assert.match(container.textContent ?? '', /Preview failed/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('refreshes an expired COS audio URL once', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let refreshCount = 0

    await act(async () => {
      root.render(
        <AssetAudioPreview
          item={{
            ...cosImage,
            id: 'asset-cos-audio',
            asset_type: 'audio',
          }}
          onRefreshPreview={async () => {
            refreshCount += 1
            return 'https://cos.example/fresh-audio-signature'
          }}
        />
      )
    })

    const expiredAudio = container.querySelector('audio')
    assert.ok(expiredAudio)
    await act(async () => expiredAudio.dispatchEvent(new Event('error')))

    const refreshedAudio = container.querySelector('audio')
    assert.ok(refreshedAudio)
    assert.equal(
      refreshedAudio.getAttribute('src'),
      'https://cos.example/fresh-audio-signature'
    )
    assert.equal(refreshCount, 1)

    await act(async () => refreshedAudio.dispatchEvent(new Event('error')))
    assert.equal(refreshCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })

  test('refreshes the COS URL before opening a preview tab', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let refreshCount = 0
    let openedURL = ''
    let closed = false
    const previewWindow = {
      opener: domWindow,
      close: () => {
        closed = true
      },
      location: {
        replace: (url: string) => {
          openedURL = url
        },
      },
    }
    const openSpy = vi
      .spyOn(domWindow, 'open')
      .mockReturnValue(previewWindow as unknown as Window)

    await act(async () => {
      root.render(
        <AssetPreviewLink
          item={cosImage}
          label='Open COS preview'
          onRefreshPreview={async () => {
            refreshCount += 1
            return 'https://cos.example/fresh-open-signature'
          }}
        />
      )
    })

    const link = container.querySelector('a')
    assert.ok(link)
    await act(async () => link.click())

    assert.equal(refreshCount, 1)
    assert.equal(openedURL, 'https://cos.example/fresh-open-signature')
    assert.equal(closed, false)
    assert.equal(previewWindow.opener, null)
    assert.equal(openSpy.mock.calls[0]?.[0], 'about:blank')

    openSpy.mockRestore()
    await act(async () => root.unmount())
    container.remove()
  })
})
