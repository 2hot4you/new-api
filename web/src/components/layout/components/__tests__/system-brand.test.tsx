import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { renderToStaticMarkup } from 'react-dom/server'

import { SystemBrandInlineContent } from '../system-brand'

describe('console inline brand', () => {
  test('uses the shared Molii wordmark without the legacy logo and name', () => {
    const markup = renderToStaticMarkup(
      <SystemBrandInlineContent logo='/logo.png' name='Molii' logoAlt='Logo' />
    )

    assert.match(markup, /data-molii-wordmark="true"/)
    assert.match(markup, /data-console-wordmark="true"/)
    assert.match(markup, /class="[^"]*h-6[^"]*"/)
    assert.doesNotMatch(markup, /src="\/logo\.png"/)
    assert.doesNotMatch(markup, /data-system-brand-name/)
  })

  test('preserves the configured console logo and name', () => {
    const markup = renderToStaticMarkup(
      <SystemBrandInlineContent
        logo='/custom-brand.png'
        name='Custom Brand'
        logoAlt='Logo'
      />
    )

    assert.match(markup, /src="\/custom-brand\.png"/)
    assert.match(markup, /data-system-brand-name="true"/)
    assert.match(markup, />Custom Brand<\/span>/)
    assert.doesNotMatch(markup, /data-molii-wordmark/)
  })
})
