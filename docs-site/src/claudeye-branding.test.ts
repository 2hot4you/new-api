import { describe, expect, test } from 'bun:test';

import type { PublicBrandConfig } from './config';
import { resolveDocsBrandAssets } from './claudeye-branding';

const CLAUDEYE_BRAND: PublicBrandConfig = {
  id: 'claudeye',
  siteTitle: 'claudeye 开发者文档',
  tagline: 'claudeye AI API 开发指南',
  navbarTitle: 'claudeye',
  logoPath: 'img/brand/logo.svg',
  faviconPath: 'img/brand/favicon.png',
  socialImagePath: 'img/brand/social.png',
  defaultFont: 'sans',
};

const MOLII_BRAND: PublicBrandConfig = {
  id: 'molii',
  siteTitle: 'Molii 开发者文档',
  tagline: '构建可靠、可扩展的 AI 创作体验',
  navbarTitle: 'Molii',
  logoPath: 'img/molii-wordmark.png',
  faviconPath: 'img/molii-favicon-32.png?v=4',
  socialImagePath: 'img/molii-mark.svg',
  defaultFont: 'serif',
};

describe('resolveDocsBrandAssets', () => {
  test('uses absolute runtime endpoints and a base-path-aware neutral fallback for claudeye', () => {
    expect(resolveDocsBrandAssets(CLAUDEYE_BRAND, 'https://claudeye.com', '/docs/')).toEqual({
      navbarLogo: 'https://claudeye.com/api/branding/claudeye/wordmark.svg?surface=light',
      footerLogo: 'https://claudeye.com/api/branding/claudeye/wordmark.svg?surface=dark',
      favicon: 'https://claudeye.com/api/branding/claudeye/favicon.svg',
      fallbackLogo: '/docs/img/claudeye-wordmark-neutral.png',
      dynamic: true,
    });
  });

  test('derives dynamic endpoints from each active API origin', () => {
    expect(
      resolveDocsBrandAssets(CLAUDEYE_BRAND, 'https://model.claudeye.com', '/docs/'),
    ).toMatchObject({
      navbarLogo:
        'https://model.claudeye.com/api/branding/claudeye/wordmark.svg?surface=light',
      footerLogo:
        'https://model.claudeye.com/api/branding/claudeye/wordmark.svg?surface=dark',
      favicon: 'https://model.claudeye.com/api/branding/claudeye/favicon.svg',
    });
  });

  test('keeps configured static assets for non-claudeye brands', () => {
    expect(resolveDocsBrandAssets(MOLII_BRAND, 'https://molii.co', '/docs/')).toEqual({
      navbarLogo: '/docs/img/molii-wordmark.png',
      footerLogo: '/docs/img/molii-wordmark.png',
      favicon: '/docs/img/molii-favicon-32.png?v=4',
      fallbackLogo: '/docs/img/molii-wordmark.png',
      dynamic: false,
    });
  });
});
