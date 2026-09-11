export type PublicEnvironment = Record<string, string | undefined>;

export interface PublicAlgoliaConfig {
  apiKey: string;
  appId: string;
  contextualSearch: false;
  indexName: string;
}

export interface PublicBrandConfig {
  id: 'molii' | 'ixiaozu';
  siteTitle: string;
  tagline: string;
  navbarTitle: string;
  logoPath: string;
  faviconPath: string;
  socialImagePath: string;
  defaultFont: 'sans' | 'serif';
}

export interface PublicConfig {
  algolia?: PublicAlgoliaConfig;
  apiBaseUrl: string;
  baseUrl: string;
  brand: PublicBrandConfig;
  noIndex: boolean;
  siteUrl: string;
}

const publicVariables = new Set([
  'DOCS_ENV',
  'DOCS_SITE_URL',
  'DOCS_BASE_URL',
  'DOCS_API_BASE_URL',
  'DOCS_BRAND_ID',
  'DOCS_SITE_TITLE',
  'DOCS_TAGLINE',
  'DOCS_NAVBAR_TITLE',
  'DOCS_LOGO_PATH',
  'DOCS_FAVICON_PATH',
  'DOCS_SOCIAL_IMAGE_PATH',
  'DOCS_DEFAULT_FONT',
  'DOCS_LOGO_SOURCE_URL',
  'DOCS_FAVICON_SOURCE_URL',
  'DOCS_SOCIAL_IMAGE_SOURCE_URL',
  'DOCS_ALGOLIA_APP_ID',
  'DOCS_ALGOLIA_SEARCH_API_KEY',
  'DOCS_ALGOLIA_INDEX_NAME',
]);

const secretName = /(?:key|secret|token|password|private|credential)/i;

function required(environment: PublicEnvironment, name: string): string {
  const value = environment[name]?.trim();

  if (!value) {
    throw new Error(`${name} must be set.`);
  }

  return value;
}

function originUrl(value: string, name: string): string {
  let url: URL;

  try {
    url = new URL(value);
  } catch {
    throw new Error(`${name} must be a valid HTTP(S) origin.`);
  }

  if (
    !['http:', 'https:'].includes(url.protocol) ||
    url.pathname !== '/' ||
    url.search ||
    url.hash ||
    url.username ||
    url.password
  ) {
    throw new Error(`${name} must be an HTTP(S) origin without a path.`);
  }

  return url.origin;
}

function normalizeBaseUrl(value: string): string {
  if (value.includes('://') || value.includes('?') || value.includes('#')) {
    throw new Error('DOCS_BASE_URL must be a path.');
  }

  const normalizedPath = value.replace(/^\/+|\/+$/g, '');

  return normalizedPath ? `/${normalizedPath}/` : '/';
}

const moliiBrand: PublicBrandConfig = {
  id: 'molii',
  siteTitle: 'Molii 开发者文档',
  tagline: '构建可靠、可扩展的 AI 创作体验',
  navbarTitle: 'Molii',
  logoPath: 'img/molii-wordmark.png',
  faviconPath: 'img/molii-favicon-32.png?v=4',
  socialImagePath: 'img/molii-mark.svg',
  defaultFont: 'serif',
};

function requiredForBrand(
  environment: PublicEnvironment,
  name: string,
  brandId: PublicBrandConfig['id'],
): string {
  const value = environment[name]?.trim();
  if (!value) {
    throw new Error(`${name} must be set for ${brandId}.`);
  }
  return value;
}

function brandAssetPath(value: string, name: string): string {
  if (
    !value.startsWith('img/brand/') ||
    value.includes('..') ||
    value.includes('\\') ||
    value.includes('://') ||
    value.includes('?') ||
    value.includes('#') ||
    !/\.(?:png|jpe?g|webp|svg|ico)$/i.test(value)
  ) {
    throw new Error(`${name} must be a relative file under img/brand/.`);
  }
  return value;
}

function resolveBrand(environment: PublicEnvironment): PublicBrandConfig {
  const id = required(environment, 'DOCS_BRAND_ID');
  if (id !== 'molii' && id !== 'ixiaozu') {
    throw new Error('DOCS_BRAND_ID must be either molii or ixiaozu.');
  }

  const configuredFields = [
    'DOCS_SITE_TITLE',
    'DOCS_TAGLINE',
    'DOCS_NAVBAR_TITLE',
    'DOCS_LOGO_PATH',
    'DOCS_FAVICON_PATH',
    'DOCS_SOCIAL_IMAGE_PATH',
    'DOCS_DEFAULT_FONT',
  ].filter((name) => environment[name]?.trim());

  if (id === 'molii' && configuredFields.length === 0) {
    return moliiBrand;
  }

  const defaultFont = requiredForBrand(environment, 'DOCS_DEFAULT_FONT', id);
  if (defaultFont !== 'sans' && defaultFont !== 'serif') {
    throw new Error('DOCS_DEFAULT_FONT must be either sans or serif.');
  }

  return {
    id,
    siteTitle: requiredForBrand(environment, 'DOCS_SITE_TITLE', id),
    tagline: requiredForBrand(environment, 'DOCS_TAGLINE', id),
    navbarTitle: requiredForBrand(environment, 'DOCS_NAVBAR_TITLE', id),
    logoPath: brandAssetPath(
      requiredForBrand(environment, 'DOCS_LOGO_PATH', id),
      'DOCS_LOGO_PATH',
    ),
    faviconPath: brandAssetPath(
      requiredForBrand(environment, 'DOCS_FAVICON_PATH', id),
      'DOCS_FAVICON_PATH',
    ),
    socialImagePath: brandAssetPath(
      requiredForBrand(environment, 'DOCS_SOCIAL_IMAGE_PATH', id),
      'DOCS_SOCIAL_IMAGE_PATH',
    ),
    defaultFont,
  };
}

function assertNoSecrets(environment: PublicEnvironment): void {
  for (const [name, value] of Object.entries(environment)) {
    if (name.startsWith('DOCS_') && !publicVariables.has(name) && secretName.test(name) && value) {
      throw new Error(`${name} cannot be exposed to the public documentation site.`);
    }
  }
}

export function resolvePublicConfig(environment: PublicEnvironment): PublicConfig {
  assertNoSecrets(environment);

  const docsEnvironment = required(environment, 'DOCS_ENV');
  if (!['development', 'production'].includes(docsEnvironment)) {
    throw new Error('DOCS_ENV must be either development or production.');
  }

  let algolia: PublicAlgoliaConfig | undefined;
  if (docsEnvironment === 'development') {
    const appId = environment.DOCS_ALGOLIA_APP_ID?.trim();
    const apiKey = environment.DOCS_ALGOLIA_SEARCH_API_KEY?.trim();
    const indexName = environment.DOCS_ALGOLIA_INDEX_NAME?.trim();
    const configuredValueCount = [appId, apiKey, indexName].filter(Boolean).length;

    if (configuredValueCount > 0 && configuredValueCount < 3) {
      throw new Error(
        'Development Algolia search requires DOCS_ALGOLIA_APP_ID, DOCS_ALGOLIA_SEARCH_API_KEY, and DOCS_ALGOLIA_INDEX_NAME.',
      );
    }
    if (configuredValueCount === 3) {
      algolia = {
        appId: appId!,
        apiKey: apiKey!,
        contextualSearch: false,
        indexName: indexName!,
      };
    }
  }

  return {
    ...(algolia ? { algolia } : {}),
    siteUrl: originUrl(required(environment, 'DOCS_SITE_URL'), 'DOCS_SITE_URL'),
    baseUrl: normalizeBaseUrl(required(environment, 'DOCS_BASE_URL')),
    apiBaseUrl: originUrl(required(environment, 'DOCS_API_BASE_URL'), 'DOCS_API_BASE_URL'),
    brand: resolveBrand(environment),
    noIndex: docsEnvironment !== 'production',
  };
}
