import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import { buildHomeDocsUrl, getHomeFooterVariant } from '../../lib/home-footer'
import { HomeFooterContent } from '../sections/home-footer'

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

function renderFooter(
  overrides: Partial<React.ComponentProps<typeof HomeFooterContent>> = {}
) {
  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <HomeFooterContent
        displayName='Molii'
        displayLogo='/logo.png'
        docsLink='https://docs.molii.example/'
        userAgreementEnabled={false}
        privacyPolicyEnabled={false}
        {...overrides}
      />
    </I18nextProvider>
  )
}

describe('Molii homepage footer', () => {
  test('uses the shared large Molii wordmark in the default footer', () => {
    const markup = renderFooter()

    assert.match(markup, /data-molii-wordmark="true"/)
    assert.match(markup, /data-home-footer-wordmark="true"/)
    assert.match(markup, /class="[^"]*h-12[^"]*"/)
    assert.doesNotMatch(markup, /src="\/logo\.png"/)
    assert.doesNotMatch(markup, /data-home-molii-letter/)
  })

  test('preserves configured footer branding', () => {
    const markup = renderFooter({
      displayName: 'Custom Brand',
      displayLogo: '/custom-brand.png',
    })

    assert.match(markup, /src="\/custom-brand\.png"/)
    assert.match(markup, />Custom Brand<\/span>/)
    assert.doesNotMatch(markup, /data-molii-wordmark/)
  })

  test('builds documentation child links without accepting unsafe URLs', () => {
    assert.equal(
      buildHomeDocsUrl(
        'https://docs.molii.example/base/',
        '/getting-started/quickstart'
      ),
      'https://docs.molii.example/base/getting-started/quickstart'
    )
    assert.equal(
      buildHomeDocsUrl('/docs/', '/api-reference'),
      '/docs/api-reference'
    )
    assert.equal(
      buildHomeDocsUrl(
        'https://docs.molii.example/base?source=home#start',
        '/api-reference'
      ),
      'https://docs.molii.example/base/api-reference'
    )
    assert.equal(
      buildHomeDocsUrl('javascript:alert(1)', '/api-reference'),
      undefined
    )
    assert.equal(buildHomeDocsUrl(undefined, '/api-reference'), undefined)
  })

  test('renders real product and documentation destinations in a charcoal layout', () => {
    const markup = renderFooter()

    assert.match(markup, /data-home-footer="true"/)
    assert.match(markup, /bg-\[#171717\]/)
    assert.match(markup, /lg:grid-cols-\[1\.4fr_repeat\(3,1fr\)\]/)
    for (const href of [
      '/pricing',
      '/keys',
      '/usage-logs/task',
      '/usage-logs/common',
      'https://docs.molii.example/getting-started/quickstart',
      'https://docs.molii.example/api-reference',
      '/about',
      'https://github.com/QuantumNous/new-api',
    ]) {
      assert.match(markup, new RegExp(`href="${href}"`))
    }
  })

  test('shows only the legal links enabled by system status', () => {
    const privacyOnly = renderFooter({ privacyPolicyEnabled: true })
    assert.match(privacyOnly, /href="\/privacy-policy"/)
    assert.doesNotMatch(privacyOnly, /href="\/user-agreement"/)

    const both = renderFooter({
      userAgreementEnabled: true,
      privacyPolicyEnabled: true,
    })
    assert.match(both, /href="\/privacy-policy"/)
    assert.match(both, /href="\/user-agreement"/)
  })

  test('omits documentation links when no documentation site is configured', () => {
    const markup = renderFooter({ docsLink: undefined })

    assert.doesNotMatch(markup, /getting-started\/quickstart/)
    assert.doesNotMatch(markup, /api-reference/)
  })

  test('preserves the existing custom Footer HTML path', () => {
    assert.equal(getHomeFooterVariant('<span>Custom Footer</span>'), 'custom')
    assert.equal(getHomeFooterVariant('  '), 'molii')
    assert.equal(getHomeFooterVariant(undefined), 'molii')
  })
})
