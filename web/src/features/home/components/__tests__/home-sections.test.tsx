import assert from 'node:assert/strict'
import { afterAll as after, describe, test } from 'vitest'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
const { MoliiBrandSentence } = await import('../molii-brand-sentence')
const { LatestModels } = await import('../sections/latest-models')
const { Features } = await import('../sections/features')
const { HowItWorks } = await import('../sections/how-it-works')
const { CTA } = await import('../sections/cta')
const { Hero } = await import('../sections/hero')
const { HomeFooterContent } = await import('../sections/home-footer')

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
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  container.innerHTML = renderToStaticMarkup(
    <QueryClientProvider client={queryClient}>
      <I18nextProvider i18n={i18n}>{node}</I18nextProvider>
    </QueryClientProvider>
  )
  return container
}

describe('Molii home sections', () => {
  after(() => domWindow.close())

  test('renders Molii with the approved per-letter brand color order', () => {
    const container = render(
      <MoliiBrandSentence sentence='Create with Molii.' />
    )
    const letters = [...container.querySelectorAll('[data-home-molii-letter]')]

    assert.equal(container.textContent, 'Create with Molii.')
    assert.deepEqual(
      letters.map((letter) => [
        letter.textContent,
        letter.getAttribute('data-color'),
      ]),
      [
        ['M', 'pink'],
        ['o', 'blue'],
        ['l', 'pink'],
        ['i', 'blue'],
        ['i', 'pink'],
      ]
    )
    assert.match(letters[0].className, /from-\[#ffb3c7\]/)
    assert.match(letters[1].className, /from-\[#62cdf6\]/)
  })

  test('preserves translated sentence order when Molii appears first', () => {
    const container = render(
      <MoliiBrandSentence sentence='Molii で作成します。' />
    )

    assert.equal(container.textContent, 'Molii で作成します。')
    assert.equal(
      container.querySelectorAll('[data-home-molii-letter]').length,
      5
    )
  })

  test('leaves translations without Molii unchanged', () => {
    const container = render(<MoliiBrandSentence sentence='开始创作。' />)

    assert.equal(container.textContent, '开始创作。')
    assert.equal(
      container.querySelectorAll('[data-home-molii-letter]').length,
      0
    )
  })

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

    for (const row of rows) {
      const sequences = row.querySelectorAll('[data-vendor-marquee-sequence]')
      assert.equal(sequences.length, 2)
      assert.equal(sequences[0].querySelectorAll('a').length, 8)
      assert.equal(sequences[1].querySelectorAll('a').length, 8)
      assert.match(sequences[0].textContent ?? '', /DeepSeek/)
      assert.match(sequences[0].textContent ?? '', /Qwen/)
      assert.deepEqual(
        [...sequences[0].querySelectorAll('a')]
          .slice(0, 2)
          .map((link) => link.getAttribute('href')),
        ['/pricing?vendor=DeepSeek', '/pricing?vendor=Qwen']
      )
    }

    assert.equal(
      container.querySelectorAll('a[href="/pricing?vendor=DeepSeek"]').length,
      16
    )
  })

  test('keeps search suggestions in a higher layer than the hero actions', () => {
    const container = render(<Hero models={[]} />)
    const hero = container.querySelector('[data-home-hero]')
    const searchLayer = container.querySelector('[data-home-search-layer]')
    const actionLayer = container.querySelector('[data-home-cta-layer]')

    assert.ok(hero)
    assert.ok(searchLayer)
    assert.ok(actionLayer)
    assert.match(hero.className, /z-20/)
    assert.match(hero.className, /overflow-visible/)
    assert.match(searchLayer.className, /z-20/)
    assert.doesNotMatch(actionLayer.className, /z-20/)
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
    const apiExample = container.querySelector('[data-home-api-example]')
    assert.ok(apiExample)
    assert.match(apiExample.className, /bg-white/)
    assert.match(apiExample.className, /text-slate-950/)
    assert.doesNotMatch(apiExample.className, /bg-\[#111\]/)
    assert.doesNotMatch(apiExample.className, /dark:/)
    assert.match(container.textContent ?? '', /Authorization: Bearer/)
    assert.ok(container.querySelector('a[href="/pricing"]'))
    assert.equal(container.querySelector('form'), null)
  })

  test('ends the default homepage with the dedicated Molii footer', () => {
    const container = render(
      <HomeFooterContent
        displayName='Molii'
        displayLogo='/logo.png'
        userAgreementEnabled={false}
        privacyPolicyEnabled={false}
        vendors={[]}
      />
    )

    assert.ok(container.querySelector('[data-home-footer]'))
  })
})
