import assert from 'node:assert/strict'

import { renderToStaticMarkup } from 'react-dom/server'
import { describe, test } from 'vitest'

import { HeaderBrand } from '../header-brand'
import { SystemBrandInlineContent } from '../system-brand'

const IXIAOZU_LOGO = '/logo.png'

describe('iXiaozu site brand', () => {
  test('renders the configured default image and name in the public header', () => {
    const markup = renderToStaticMarkup(
      <HeaderBrand
        brandId='ixiaozu'
        systemLogo={IXIAOZU_LOGO}
        siteName='运通源益'
        loading={false}
        logoLoaded
      />
    )

    assert.match(markup, /src="\/logo\.png"/)
    assert.match(markup, /data-header-site-name="true"/)
    assert.match(markup, />运通源益<\/span>/)
    assert.doesNotMatch(markup, /data-molii-wordmark/)
  })

  test('renders the configured default image and name in the console header', () => {
    const markup = renderToStaticMarkup(
      <SystemBrandInlineContent
        brandId='ixiaozu'
        logo={IXIAOZU_LOGO}
        name='运通源益'
        logoAlt='Logo'
      />
    )

    assert.match(markup, /src="\/logo\.png"/)
    assert.match(markup, /data-system-brand-name="true"/)
    assert.match(markup, />运通源益<\/span>/)
    assert.doesNotMatch(markup, /data-molii-wordmark/)
  })
})
