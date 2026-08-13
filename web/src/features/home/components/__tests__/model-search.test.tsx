import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { PricingModel } from '@/features/pricing/types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
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
  'KeyboardEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'matchMedia',
  'customElements',
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
const { ModelSearch } = await import('../model-search')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(
    domWindow.HTMLInputElement.prototype,
    'value'
  )?.set
  assert.ok(setter)
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

function model(modelName: string, vendorName: string): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    vendor_name: vendorName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
  }
}

describe('home model search', () => {
  after(() => domWindow.close())

  test('uses catalog IDs while idle and restores the original copy on focus', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelSearch models={[model('deepseek-v4', 'DeepSeek')]} />
        </I18nextProvider>
      )
    })

    const input = container.querySelector<HTMLInputElement>('input')
    assert.ok(input)
    assert.notEqual(
      input.placeholder,
      'Search models, providers, or capabilities'
    )

    await act(async () => input.focus())
    assert.equal(input.placeholder, 'Search models, providers, or capabilities')

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps the search text and submit action in separate layout regions', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelSearch models={[model('deepseek-v4', 'DeepSeek')]} />
        </I18nextProvider>
      )
    })

    const form = container.querySelector('form[role="search"]')
    const textRegion = form?.querySelector('[data-home-search-text]')
    const actionRegion = form?.querySelector('[data-home-search-action]')
    assert.ok(textRegion)
    assert.ok(actionRegion)
    assert.equal(actionRegion.closest('[data-home-search-text]'), null)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows matched models and submits a normalized marketplace query', async () => {
    const queries: string[] = []
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelSearch
            models={[
              model('deepseek-v4', 'DeepSeek'),
              model('qwen3.5-plus', 'Qwen'),
            ]}
            onSearch={(query) => queries.push(query)}
          />
        </I18nextProvider>
      )
    })

    const input = container.querySelector<HTMLInputElement>('input')
    assert.ok(input)
    await act(async () => {
      input.focus()
      setInputValue(input, '  deepseek  ')
    })

    assert.match(container.textContent ?? '', /deepseek-v4/)

    const form = container.querySelector('form')
    assert.ok(form)
    await act(async () => {
      form.dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true })
      )
    })

    assert.deepEqual(queries, ['deepseek'])
    await act(async () => root.unmount())
    container.remove()
  })

  test('links a suggestion directly to its model detail page', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelSearch models={[model('qwen3.5-plus', 'Qwen')]} />
        </I18nextProvider>
      )
    })

    const input = container.querySelector<HTMLInputElement>('input')
    assert.ok(input)
    await act(async () => {
      input.focus()
      setInputValue(input, 'qwen')
    })

    assert.ok(container.querySelector('a[href="/pricing/qwen3.5-plus"]'))
    await act(async () => root.unmount())
    container.remove()
  })
})
