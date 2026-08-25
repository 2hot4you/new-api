import { expect, test } from 'bun:test';

import siteConfig from '../docusaurus.config';
import { resolvePublicConfig } from './config';

const validEnvironment = {
  DOCS_ENV: 'development',
  DOCS_SITE_URL: 'http://127.0.0.1:3100',
  DOCS_BASE_URL: '/',
  DOCS_API_BASE_URL: 'http://127.0.0.1:3000',
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

test('resolves the public /docs/ deployment contract for both environments', () => {
  expect(
    resolvePublicConfig({
      DOCS_ENV: 'development',
      DOCS_SITE_URL: 'https://dev.molii.co',
      DOCS_BASE_URL: '/docs/',
      DOCS_API_BASE_URL: 'https://dev.molii.co',
    }),
  ).toEqual({
    siteUrl: 'https://dev.molii.co',
    baseUrl: '/docs/',
    apiBaseUrl: 'https://dev.molii.co',
    noIndex: true,
  });

  expect(
    resolvePublicConfig({
      DOCS_ENV: 'production',
      DOCS_SITE_URL: 'https://molii.co',
      DOCS_BASE_URL: '/docs/',
      DOCS_API_BASE_URL: 'https://molii.co',
    }),
  ).toEqual({
    siteUrl: 'https://molii.co',
    baseUrl: '/docs/',
    apiBaseUrl: 'https://molii.co',
    noIndex: false,
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

test('keeps New API and QuantumNous attribution visible in the footer', () => {
  const footer = siteConfig.themeConfig?.footer as { copyright?: string };

  expect(footer.copyright).toContain('New API');
  expect(footer.copyright).toContain('QuantumNous');
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
