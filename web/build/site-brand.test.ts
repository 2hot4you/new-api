import { expect, test } from 'bun:test'

import { resolveSiteBrand } from './site-brand'

const IXIAOZU_FIXTURE = {
  VITE_SITE_PROFILE: 'ixiaozu',
  VITE_SITE_TITLE: 'iXiaozu AIGC',
  VITE_SITE_DESCRIPTION: 'Unified AI creation platform.',
  VITE_SITE_LOGO: '/ixiaozu-logo.png',
  VITE_SITE_FAVICON: '/ixiaozu-favicon.png',
  VITE_SITE_APPLE_TOUCH_ICON: '/ixiaozu-apple-touch-icon.png',
  VITE_SITE_BANNER_BRAND: 'iXiaozu',
  VITE_SITE_DEFAULT_FONT: 'sans',
}

test('preserves Molii profile defaults', () => {
  expect(resolveSiteBrand({ VITE_SITE_PROFILE: 'molii' })).toEqual({
    id: 'molii',
    title: 'Molii Gateway',
    description: 'Unified AI API gateway and admin dashboard.',
    logo: '/logo.png',
    favicon: '/molii-favicon-32.png?v=4',
    appleTouchIcon: '/apple-touch-icon.png?v=4',
    bannerBrand: 'Molii',
    defaultFont: 'serif',
  })
})

test('uses Molii when no profile is configured', () => {
  expect(resolveSiteBrand({}).id).toBe('molii')
})

test('requires every public field for ixiaozu', () => {
  expect(() =>
    resolveSiteBrand({ VITE_SITE_PROFILE: 'ixiaozu' })
  ).toThrow('VITE_SITE_TITLE must be set for ixiaozu')
})

test('accepts a complete non-Molii profile', () => {
  expect(resolveSiteBrand(IXIAOZU_FIXTURE)).toEqual({
    id: 'ixiaozu',
    title: 'iXiaozu AIGC',
    description: 'Unified AI creation platform.',
    logo: '/ixiaozu-logo.png',
    favicon: '/ixiaozu-favicon.png',
    appleTouchIcon: '/ixiaozu-apple-touch-icon.png',
    bannerBrand: 'iXiaozu',
    defaultFont: 'sans',
  })
})

test('rejects unknown profiles and fonts', () => {
  expect(() =>
    resolveSiteBrand({ VITE_SITE_PROFILE: 'unknown' })
  ).toThrow('VITE_SITE_PROFILE must be molii or ixiaozu')
  expect(() =>
    resolveSiteBrand({
      ...IXIAOZU_FIXTURE,
      VITE_SITE_DEFAULT_FONT: 'comic-sans',
    })
  ).toThrow('VITE_SITE_DEFAULT_FONT must be sans or serif')
})
