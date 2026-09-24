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

import { api } from '@/lib/api'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLInputElement',
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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { CreateAssetCard } = await import('../create-asset-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('temporary asset source options', () => {
  after(() => domWindow.close())

  test('keeps URL creation available when COS local upload is disabled', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <CreateAssetCard
            uploadConfig={{
              enabled: false,
              limits: { image: 30, video: 200, audio: 15 },
            }}
            onCreated={async () => {}}
          />
        </I18nextProvider>
      )
    })

    const localFileInput =
      container.querySelector<HTMLInputElement>('input[type="file"]')
    assert.ok(localFileInput)
    assert.equal(localFileInput.disabled, true)
    assert.match(container.textContent ?? '', /Configure Tencent COS/)

    const urlTab = [...container.querySelectorAll('button')].find(
      (button) => button.textContent === 'Add by URL'
    )
    assert.ok(urlTab)
    await act(async () => urlTab.click())

    assert.ok(container.querySelector('input[type="url"]'))
    assert.match(container.textContent ?? '', /Public media URL/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('uses provider-neutral wording while submitting a local asset', async () => {
    const originalXHR = globalThis.XMLHttpRequest
    const originalObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    let completeUpload: (() => void) | undefined
    const completion = new Promise<void>((resolve) => {
      completeUpload = resolve
    })
    const post = vi.spyOn(api, 'post')
    post
      .mockResolvedValueOnce({
        data: {
          data: {
            upload_id: 'upload-1',
            upload_url: 'https://upload.example',
            headers: {},
          },
        },
      })
      .mockImplementationOnce(async () => {
        await completion
        return { data: { success: true } }
      })

    class SuccessfulUploadRequest extends domWindow.EventTarget {
      status = 200
      upload = new domWindow.EventTarget()
      open() {}
      setRequestHeader() {}
      send() {
        this.dispatchEvent(new domWindow.Event('load'))
      }
    }
    Object.defineProperty(globalThis, 'XMLHttpRequest', {
      configurable: true,
      value: SuccessfulUploadRequest,
    })
    URL.createObjectURL = () => 'blob:preview'
    URL.revokeObjectURL = () => {}

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    try {
      await act(async () => {
        root.render(
          <I18nextProvider i18n={i18n}>
            <CreateAssetCard
              uploadConfig={{
                enabled: true,
                limits: {
                  image: 30_000_000,
                  video: 200_000_000,
                  audio: 15_000_000,
                },
              }}
              onCreated={async () => {}}
            />
          </I18nextProvider>
        )
      })

      const dropZone = container.querySelector('button.border-dashed')
      assert.ok(dropZone)
      const drop = new Event('drop', {
        bubbles: true,
        cancelable: true,
      })
      Object.defineProperty(drop, 'dataTransfer', {
        value: {
          files: [
            new domWindow.File(['image'], 'sample.png', { type: 'image/png' }),
          ],
        },
      })
      await act(async () => dropZone.dispatchEvent(drop))

      const form = container.querySelector('form')
      assert.ok(form)
      await act(async () =>
        form.dispatchEvent(
          new Event('submit', { bubbles: true, cancelable: true })
        )
      )

      assert.match(
        container.textContent ?? '',
        /Submitting temporary asset\.\.\./
      )
      assert.doesNotMatch(
        container.textContent ?? '',
        /Molii Volcengine Imagine API/
      )
    } finally {
      await act(async () => {
        completeUpload?.()
      })
      await act(async () => root.unmount())
      container.remove()
      post.mockRestore()
      Object.defineProperty(globalThis, 'XMLHttpRequest', {
        configurable: true,
        value: originalXHR,
      })
      URL.createObjectURL = originalObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })
})
