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

const CLAUDEYE_FIXTURE = {
  ...IXIAOZU_FIXTURE,
  VITE_SITE_PROFILE: 'claudeye',
  VITE_SITE_TITLE: 'Claudeye',
  VITE_SITE_DESCRIPTION: 'Claudeye model gateway.',
  VITE_SITE_LOGO: '/claudeye-logo.svg',
  VITE_SITE_FAVICON: '/claudeye-static-favicon.png',
  VITE_SITE_APPLE_TOUCH_ICON: '/claudeye-apple-touch-icon.png',
  VITE_SITE_BANNER_BRAND: 'Claudeye',
}

test('preserves Molii profile defaults', () => {
  expect(resolveSiteBrand({ VITE_SITE_PROFILE: 'molii' })).toEqual({
    id: 'molii',
    title: 'Molii Gateway',
    description: 'Unified AI API gateway and admin dashboard.',
    logo: '/logo.png',
    favicon: '/molii-favicon-32.png?v=4',
    faviconFallback: '/molii-favicon-32.png?v=4',
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
    faviconFallback: '/ixiaozu-favicon.png',
    appleTouchIcon: '/ixiaozu-apple-touch-icon.png',
    bannerBrand: 'iXiaozu',
    defaultFont: 'sans',
  })
})

test('rejects unknown profiles and fonts', () => {
  expect(() =>
    resolveSiteBrand({ VITE_SITE_PROFILE: 'unknown' })
  ).toThrow('VITE_SITE_PROFILE must be molii, ixiaozu or claudeye')
  expect(() =>
    resolveSiteBrand({
      ...IXIAOZU_FIXTURE,
      VITE_SITE_DEFAULT_FONT: 'comic-sans',
    })
  ).toThrow('VITE_SITE_DEFAULT_FONT must be sans or serif')
})

test('claudeye requires explicit independent branding', () => {
  expect(() => resolveSiteBrand({ VITE_SITE_PROFILE: 'claudeye' }))
    .toThrow('VITE_SITE_TITLE must be set for claudeye')
  const result = resolveSiteBrand({
    ...IXIAOZU_FIXTURE,
    VITE_SITE_PROFILE: 'claudeye',
    VITE_SITE_TITLE: 'Claudeye',
    VITE_SITE_BANNER_BRAND: 'Claudeye',
    VITE_SITE_LOGO: '/claudeye-logo.svg',
  })
  expect(result.id).toBe('claudeye')
  expect(result.title).toBe('Claudeye')
  expect(result.logo).toBe('/claudeye-logo.svg')
  expect(result.bannerBrand).toBe('Claudeye')
})

test('uses the dynamic favicon only for a complete claudeye profile', () => {
  const claudeye = resolveSiteBrand(CLAUDEYE_FIXTURE)

  expect(claudeye.logo).toBe('/claudeye-logo.svg')
  expect(claudeye.favicon).toBe('/api/branding/claudeye/favicon.svg')
  expect(claudeye.faviconFallback).toBe('/claudeye-static-favicon.png')
  expect(claudeye.appleTouchIcon).toBe('/claudeye-apple-touch-icon.png')
  expect(resolveSiteBrand(IXIAOZU_FIXTURE).favicon).toBe(
    '/ixiaozu-favicon.png'
  )
  expect(resolveSiteBrand({ VITE_SITE_PROFILE: 'molii' }).favicon).toBe(
    '/molii-favicon-32.png?v=4'
  )
})
