import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import { HeaderBrand } from '../header-brand'

function renderBrand(
  overrides: Partial<React.ComponentProps<typeof HeaderBrand>> = {}
) {
  return renderToStaticMarkup(
    <HeaderBrand
      systemLogo='/logo.png'
      siteName='Molii'
      loading={false}
      logoLoaded
      {...overrides}
    />
  )
}

describe('public header brand', () => {
  test('uses the Molii wordmark without repeating the site name by default', () => {
    const markup = renderBrand()

    assert.match(markup, /data-header-wordmark="true"/)
    assert.match(markup, /src="\/molii-wordmark\.png"/)
    assert.match(markup, /alt="Molii"/)
    assert.doesNotMatch(markup, /data-header-site-name/)
  })

  test('keeps the configured logo and site name for custom system branding', () => {
    const markup = renderBrand({ systemLogo: '/custom-brand.png' })

    assert.match(markup, /src="\/custom-brand\.png"/)
    assert.match(markup, /data-header-site-name="true"/)
    assert.match(markup, />Molii<\/span>/)
    assert.doesNotMatch(markup, /data-header-wordmark/)
  })

  test('keeps a custom React logo together with the site name', () => {
    const markup = renderBrand({
      customLogo: <svg data-custom-logo='true' />,
    })

    assert.match(markup, /data-custom-logo="true"/)
    assert.match(markup, /data-header-site-name="true"/)
    assert.doesNotMatch(markup, /data-header-wordmark/)
  })

  test('uses a wordmark-sized skeleton for the default loading state', () => {
    const markup = renderBrand({ loading: true, logoLoaded: false })

    assert.match(markup, /data-header-wordmark-skeleton="true"/)
    assert.doesNotMatch(markup, /data-header-site-name/)
  })
})
