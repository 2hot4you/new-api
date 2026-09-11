import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { SiteBrand } from '../../../build/site-brand'
import { resolveRuntimeSiteBrand } from '../../config/site-brand'
import { resolveSystemName } from '../constants'
import { resolveFaviconUrl } from '../dom-utils'
import { resolveDefaultThemeCustomization } from '../theme-customization'

const IXIAOZU_BRAND: SiteBrand = {
  id: 'ixiaozu',
  title: 'iXiaozu AIGC',
  description: 'iXiaozu description',
  logo: '/ixiaozu-logo.png',
  favicon: '/ixiaozu-favicon.png',
  appleTouchIcon: '/ixiaozu-apple.png',
  bannerBrand: 'iXiaozu',
  defaultFont: 'sans',
}

describe('site brand runtime fallbacks', () => {
  test('uses the injected site brand when present', () => {
    assert.deepEqual(resolveRuntimeSiteBrand(IXIAOZU_BRAND), IXIAOZU_BRAND)
  })

  test('resolves empty and legacy New API names to the active brand', () => {
    assert.equal(resolveSystemName('', IXIAOZU_BRAND), 'iXiaozu AIGC')
    assert.equal(resolveSystemName('New API', IXIAOZU_BRAND), 'iXiaozu AIGC')
    assert.equal(resolveSystemName('Custom Name', IXIAOZU_BRAND), 'Custom Name')
  })

  test('resolves the active brand logo to its dedicated favicon', () => {
    assert.equal(
      resolveFaviconUrl('/ixiaozu-logo.png', IXIAOZU_BRAND),
      '/ixiaozu-favicon.png'
    )
  })

  test('uses each site profile font as the theme default', () => {
    assert.equal(resolveDefaultThemeCustomization('serif').font, 'serif')
    assert.equal(
      resolveDefaultThemeCustomization(IXIAOZU_BRAND.defaultFont).font,
      'sans'
    )
  })
})
