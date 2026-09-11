import { expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

import siteConfig from '../docusaurus.config';
import { resolvePublicConfig } from './config';

const validEnvironment = {
  DOCS_ENV: 'development',
  DOCS_SITE_URL: 'http://127.0.0.1:3100',
  DOCS_BASE_URL: '/',
  DOCS_API_BASE_URL: 'http://127.0.0.1:3000',
  DOCS_BRAND_ID: 'molii',
};

const ixiaozuEnvironment = {
  DOCS_ENV: 'production',
  DOCS_SITE_URL: 'https://aigc.ixiaozu.cn',
  DOCS_BASE_URL: '/docs/',
  DOCS_API_BASE_URL: 'https://aigc.ixiaozu.cn',
  DOCS_BRAND_ID: 'ixiaozu',
  DOCS_SITE_TITLE: 'iXiaozu 开发者文档',
  DOCS_TAGLINE: 'iXiaozu AI 创作平台开发指南',
  DOCS_NAVBAR_TITLE: 'iXiaozu',
  DOCS_LOGO_PATH: 'img/brand/logo.svg',
  DOCS_FAVICON_PATH: 'img/brand/favicon.png',
  DOCS_SOCIAL_IMAGE_PATH: 'img/brand/social.png',
  DOCS_DEFAULT_FONT: 'sans',
};

test('rejects a site URL with a path', () => {
  expect(() =>
    resolvePublicConfig({
      ...validEnvironment,
      DOCS_SITE_URL: 'https://docs.example.com/path',
    }),
  ).toThrow();
});

test('normalizes a base URL with leading and trailing slashes', () => {
  expect(
    resolvePublicConfig({
      ...validEnvironment,
      DOCS_BASE_URL: 'guides',
    }).baseUrl,
  ).toBe('/guides/');
});

test('rejects secret-like values in the public environment', () => {
  expect(() =>
    resolvePublicConfig({
      ...validEnvironment,
      DOCS_API_TOKEN: 'not-safe-for-a-static-site',
    }),
  ).toThrow();
});

test('prevents search indexing during development', () => {
  expect(resolvePublicConfig(validEnvironment).noIndex).toBe(true);
  expect(
    resolvePublicConfig({ ...validEnvironment, DOCS_ENV: 'production' }).noIndex,
  ).toBe(false);
});

test('resolves a complete iXiaozu documentation brand', () => {
  expect(resolvePublicConfig(ixiaozuEnvironment).brand).toEqual({
    id: 'ixiaozu',
    siteTitle: 'iXiaozu 开发者文档',
    tagline: 'iXiaozu AI 创作平台开发指南',
    navbarTitle: 'iXiaozu',
    logoPath: 'img/brand/logo.svg',
    faviconPath: 'img/brand/favicon.png',
    socialImagePath: 'img/brand/social.png',
    defaultFont: 'sans',
  });
});

test('rejects incomplete or unknown documentation brands', () => {
  expect(() =>
    resolvePublicConfig({
      ...ixiaozuEnvironment,
      DOCS_SITE_TITLE: '',
    }),
  ).toThrow('DOCS_SITE_TITLE must be set for ixiaozu');
  expect(() =>
    resolvePublicConfig({
      ...validEnvironment,
      DOCS_BRAND_ID: 'unknown',
    }),
  ).toThrow('DOCS_BRAND_ID');
});

test('rejects unsafe brand asset paths and unsupported fonts', () => {
  for (const logoPath of [
    'https://assets.example/logo.svg',
    '../img/brand/logo.svg',
    '/img/brand/logo.svg',
  ]) {
    expect(() =>
      resolvePublicConfig({
        ...ixiaozuEnvironment,
        DOCS_LOGO_PATH: logoPath,
      }),
    ).toThrow('DOCS_LOGO_PATH');
  }
  expect(() =>
    resolvePublicConfig({
      ...ixiaozuEnvironment,
      DOCS_DEFAULT_FONT: 'comic-sans',
    }),
  ).toThrow('DOCS_DEFAULT_FONT');
});

test('resolves the public /docs/ deployment contract for both environments', () => {
  expect(
    resolvePublicConfig({
      DOCS_ENV: 'development',
      DOCS_SITE_URL: 'https://dev.molii.co',
      DOCS_BASE_URL: '/docs/',
      DOCS_API_BASE_URL: 'https://dev.molii.co',
      DOCS_BRAND_ID: 'molii',
    }),
  ).toEqual({
    siteUrl: 'https://dev.molii.co',
    baseUrl: '/docs/',
    apiBaseUrl: 'https://dev.molii.co',
    noIndex: true,
    brand: {
      id: 'molii',
      siteTitle: 'Molii 开发者文档',
      tagline: '构建可靠、可扩展的 AI 创作体验',
      navbarTitle: 'Molii',
      logoPath: 'img/molii-wordmark.png',
      faviconPath: 'img/molii-favicon-32.png?v=4',
      socialImagePath: 'img/molii-mark.svg',
      defaultFont: 'serif',
    },
  });

  expect(
    resolvePublicConfig({
      DOCS_ENV: 'production',
      DOCS_SITE_URL: 'https://molii.co',
      DOCS_BASE_URL: '/docs/',
      DOCS_API_BASE_URL: 'https://molii.co',
      DOCS_BRAND_ID: 'molii',
    }),
  ).toEqual({
    siteUrl: 'https://molii.co',
    baseUrl: '/docs/',
    apiBaseUrl: 'https://molii.co',
    noIndex: false,
    brand: {
      id: 'molii',
      siteTitle: 'Molii 开发者文档',
      tagline: '构建可靠、可扩展的 AI 创作体验',
      navbarTitle: 'Molii',
      logoPath: 'img/molii-wordmark.png',
      faviconPath: 'img/molii-favicon-32.png?v=4',
      socialImagePath: 'img/molii-mark.svg',
      defaultFont: 'serif',
    },
  });
});

test('enables Algolia only for a fully configured development build', () => {
  const algolia = {
    DOCS_ALGOLIA_APP_ID: 'development-app',
    DOCS_ALGOLIA_SEARCH_API_KEY: 'public-search-only-key',
    DOCS_ALGOLIA_INDEX_NAME: 'molii-development',
  };

  expect(resolvePublicConfig({ ...validEnvironment, ...algolia }).algolia).toEqual({
    appId: 'development-app',
    apiKey: 'public-search-only-key',
    contextualSearch: false,
    indexName: 'molii-development',
  });
  expect(
    resolvePublicConfig({
      ...validEnvironment,
      ...algolia,
      DOCS_ENV: 'production',
      DOCS_SITE_URL: 'https://molii.co',
      DOCS_BASE_URL: '/docs/',
      DOCS_API_BASE_URL: 'https://molii.co',
      DOCS_BRAND_ID: 'molii',
    }).algolia,
  ).toBeUndefined();
});

test('rejects a partially configured development Algolia search', () => {
  expect(() =>
    resolvePublicConfig({
      ...validEnvironment,
      DOCS_ALGOLIA_APP_ID: 'development-app',
      DOCS_ALGOLIA_SEARCH_API_KEY: 'public-search-only-key',
    }),
  ).toThrow('Development Algolia search requires');
});

test('keeps New API and QuantumNous attribution visible in the custom footer', async () => {
  const footer = await readFile(join(import.meta.dir, 'theme/Footer/index.tsx'), 'utf8');

  expect(footer).toContain('New API');
  expect(footer).toContain('QuantumNous');
});

test('links the documentation wordmark to the configured site origin', () => {
  const navbar = siteConfig.themeConfig?.navbar as {
    logo?: { href?: string; target?: string };
  };

  expect(navbar.logo).toMatchObject({
    href: siteConfig.url,
    target: '_self',
  });
});
