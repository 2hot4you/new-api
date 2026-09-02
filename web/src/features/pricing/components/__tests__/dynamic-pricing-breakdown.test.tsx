import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import { DynamicPricingBreakdown } from '../dynamic-pricing-breakdown'

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

function render(expression: string): string {
  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <DynamicPricingBreakdown billingExpr={expression} />
    </I18nextProvider>
  )
}

describe('dynamic pricing breakdown language', () => {
  test('explains that len tiers use this request complete input Tokens', () => {
    const html = render(
      'len <= 272000 ? tier("short", p * 1 + c * 2) : tier("long", p * 2 + c * 3)'
    )

    assert.match(html, /Tiered by per-request input Tokens/)
    assert.match(html, /not cumulative usage/)
    assert.match(html, /≤ 272K/)
    assert.match(html, /&gt; 272K/)
  })

  test('presents time rules as readable windows with a base-price fallback', () => {
    const html = render(
      '(tier("base", p * 1.5 + c * 4.5)) * (hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12 ? 2 : 1) * (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18 ? 2 : 1)'
    )

    assert.match(html, /Priced by request time/)
    assert.match(html, /09:00–12:00 \(Asia\/Shanghai\)/)
    assert.match(html, /14:00–18:00 \(Asia\/Shanghai\)/)
    assert.match(html, /Other times use the base price/)
    assert.match(html, /Base-period price/)
    assert.doesNotMatch(html, />base</)
    assert.doesNotMatch(html, /Conditional multipliers/)
  })

  test('shows input ranges instead of internal context tier identifiers', () => {
    const html = render(
      'len <= 128000 ? tier("short_context", p * 1 + c * 2) : tier("long_context", p * 2 + c * 4)'
    )

    assert.match(html, /Single-request input ≤ 128K Tokens/)
    assert.match(html, /Single-request input &gt; 128K Tokens/)
    assert.doesNotMatch(html, /short_context|long_context/)
  })
})
