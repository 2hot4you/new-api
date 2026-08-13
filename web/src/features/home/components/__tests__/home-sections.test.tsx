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
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'matchMedia',
  'customElements',
  'ResizeObserver',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { renderToStaticMarkup } = await import('react-dom/server')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { VendorMarquee } = await import('../vendor-marquee')
const { LatestModels } = await import('../sections/latest-models')
const { Features } = await import('../sections/features')
const { HowItWorks } = await import('../sections/how-it-works')
const { CTA } = await import('../sections/cta')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

function model(
  modelName: string,
  overrides: Partial<PricingModel> = {}
): PricingModel {
  return {
    id: overrides.id ?? 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

function render(node: React.ReactNode): HTMLDivElement {
  const container = document.createElement('div')
  container.innerHTML = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>{node}</I18nextProvider>
  )
  return container
}

describe('Molii home sections', () => {
  after(() => domWindow.close())

  test('renders two opposite vendor marquee rows from the public catalog', () => {
    const container = render(
      <VendorMarquee
        vendors={[
          { id: 1, name: 'DeepSeek', icon: 'DeepSeek', modelCount: 2 },
          { id: 2, name: 'Qwen', icon: 'Qwen', modelCount: 3 },
        ]}
      />
    )

    const rows = container.querySelectorAll('[data-vendor-marquee-row]')
    assert.equal(rows.length, 2)
    assert.equal(rows[0].getAttribute('data-direction'), 'forward')
    assert.equal(rows[1].getAttribute('data-direction'), 'reverse')
    assert.ok(rows[0].querySelector('.home-marquee-forward'))
    assert.ok(rows[1].querySelector('.home-marquee-reverse'))
    assert.equal(
      container.querySelectorAll('a[href="/pricing?vendor=DeepSeek"]').length,
      2
    )
  })

  test('renders latest model metadata and model-square detail links', () => {
    const container = render(
      <LatestModels
        models={[
          model('deepseek-v4', {
            vendor_name: 'DeepSeek',
            description: 'Long-context reasoning model',
            release_date: '2026-08-13',
            input_modalities: ['text'],
            output_modalities: ['text'],
          }),
        ]}
      />
    )

    assert.match(container.textContent ?? '', /deepseek-v4/)
    assert.match(container.textContent ?? '', /DeepSeek/)
    assert.match(container.textContent ?? '', /Long-context reasoning model/)
    assert.match(container.textContent ?? '', /2026-08-13/)
    assert.ok(container.querySelector('a[href="/pricing/deepseek-v4"]'))
  })

  test('does not render an empty latest-model section', () => {
    assert.equal(render(<LatestModels models={[]} />).innerHTML, '')
  })

  test('presents the five real Molii product capabilities', () => {
    const container = render(<Features />)

    assert.equal(container.querySelectorAll('[data-home-capability]').length, 5)
    for (const copy of [
      'LLM',
      'Seedance',
      'API Key',
      'Asynchronous',
      'billing',
    ]) {
      assert.match(container.textContent ?? '', new RegExp(copy, 'i'))
    }
  })

  test('renders four Molii product principles instead of setup steps', () => {
    const container = render(<HowItWorks />)
    assert.equal(container.querySelectorAll('[data-home-principle]').length, 4)
  })

  test('shows display-only API examples beside the model marketplace entry', () => {
    const container = render(<CTA isAuthenticated={false} />)
    assert.ok(container.querySelector('[data-home-api-example]'))
    assert.match(container.textContent ?? '', /Authorization: Bearer/)
    assert.ok(container.querySelector('a[href="/pricing"]'))
    assert.equal(container.querySelector('form'), null)
  })
})
