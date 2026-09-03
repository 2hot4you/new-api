import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { afterAll as after, describe, test } from 'vitest'

import type { PricingModel } from '../../types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
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
const { ModelDetailsCapabilities } =
  await import('../model-details-capabilities')
const { ModelBackendDetailsSection } = await import('../model-details')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function model(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'deepseek-v4-flash-202605',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    enable_groups: ['default'],
    context_length: 1_000_000,
    max_output_tokens: 384_000,
    release_date: '2026-05-01',
    input_modalities: ['text'],
    output_modalities: ['text'],
    capabilities: [
      'streaming',
      'system_prompt',
      'reasoning',
      'tools',
      'structured_output',
    ],
    supported_parameters: ['stream', 'tools'],
    supported_resolutions: ['720p', '1080p'],
    supported_aspect_ratios: ['16:9', '9:16'],
    max_input_images: 3,
    output_formats: ['url'],
    min_duration: 4,
    max_duration: 15,
    reference_modalities: ['image', 'video'],
    metadata_updated_time: 1_786_569_600,
    ...overrides,
  }
}

async function renderCapabilities(pricingModel: PricingModel) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ModelDetailsCapabilities model={pricingModel} />
      </I18nextProvider>
    )
  })

  return { container, root }
}

async function renderDetails(pricingModel: PricingModel) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ModelBackendDetailsSection model={pricingModel} />
      </I18nextProvider>
    )
  })

  return { container, root }
}

describe('LLM model capability overview card', () => {
  after(() => domWindow.close())

  test('combines specifications, modalities, capabilities, and update time', async () => {
    const { container, root } = await renderCapabilities(model())

    const card = container.querySelector('[data-model-capabilities-card]')
    assert.ok(card)
    assert.match(card.textContent ?? '', /1M/)
    assert.match(card.textContent ?? '', /384K/)
    assert.equal(
      container.querySelectorAll('[data-model-modalities]').length,
      1
    )
    assert.equal(
      container.querySelectorAll('[data-model-capability]').length,
      5
    )
    assert.equal(
      container.querySelectorAll('[data-model-capability-metadata]').length,
      6
    )
    assert.equal(
      container.querySelectorAll('[data-model-compact-metadata-cell]').length,
      1
    )
    assert.match(container.textContent ?? '', /720p/)
    assert.match(container.textContent ?? '', /16:9/)
    assert.match(container.textContent ?? '', /4–15/)
    assert.ok(container.querySelector('[data-model-metadata-note]'))
    assert.doesNotMatch(container.textContent ?? '', /models\.dev|Source/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('omits empty optional sections instead of rendering placeholders', async () => {
    const { container, root } = await renderCapabilities(
      model({
        release_date: undefined,
        knowledge_cutoff: undefined,
        input_modalities: undefined,
        output_modalities: undefined,
        capabilities: undefined,
        metadata_updated_time: undefined,
      })
    )

    assert.ok(container.querySelector('[data-model-capabilities-card]'))
    assert.equal(container.querySelector('[data-model-modalities]'), null)
    assert.equal(container.querySelector('[data-model-capability]'), null)
    assert.equal(container.querySelector('[data-model-metadata-note]'), null)
    assert.doesNotMatch(
      container.textContent ?? '',
      /Released|Knowledge cutoff/
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not reserve an empty third specification column', async () => {
    const { container, root } = await renderCapabilities(
      model({ release_date: undefined, knowledge_cutoff: undefined })
    )

    const specs = container.querySelector('[data-model-core-specs]')
    assert.ok(specs)
    assert.equal(specs.children.length, 2)
    assert.match(specs.className, /grid-cols-2/)
    assert.doesNotMatch(specs.className, /@2xl\/details:grid-cols-3/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('integrates one capability card and keeps internal provenance out of public details', async () => {
    const { container, root } = await renderDetails(
      model({
        vendor_name: 'DeepSeek',
        supported_endpoint_types: ['openai'],
      })
    )

    assert.equal(
      container.querySelectorAll('[data-model-capabilities-card]').length,
      1
    )
    assert.equal(
      container.querySelectorAll('[data-model-modalities]').length,
      1
    )
    assert.equal((container.textContent?.match(/models\.dev/g) ?? []).length, 0)
    assert.equal(
      container
        .querySelector('[data-model-provider-info]')
        ?.textContent?.includes('models.dev'),
      false
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('places supported parameters and model tags in one compact row', async () => {
    const { container, root } = await renderDetails(
      model({
        tags: 'fast,reasoning',
        vendor_name: 'DeepSeek',
        supported_endpoint_types: ['openai'],
      })
    )

    const compactRow = container.querySelector(
      '[data-model-compact-metadata-row]'
    )
    assert.ok(compactRow)
    assert.equal(
      compactRow.querySelectorAll('[data-model-compact-metadata-cell]').length,
      2
    )
    assert.match(compactRow.textContent ?? '', /Supported parameters/)
    assert.match(compactRow.textContent ?? '', /Tags/)

    const providerInfo = container.querySelector('[data-model-provider-info]')
    assert.ok(providerInfo)
    assert.doesNotMatch(providerInfo.textContent ?? '', /fast|reasoning/)

    await act(async () => root.unmount())
    container.remove()
  })
})
