export type PublicEnvironment = Record<string, string | undefined>;

export interface PublicAlgoliaConfig {
  apiKey: string;
  appId: string;
  contextualSearch: false;
  indexName: string;
}

export interface PublicConfig {
  algolia?: PublicAlgoliaConfig;
  apiBaseUrl: string;
  baseUrl: string;
  noIndex: boolean;
  siteUrl: string;
}

const publicVariables = new Set([
  'DOCS_ENV',
  'DOCS_SITE_URL',
  'DOCS_BASE_URL',
  'DOCS_API_BASE_URL',
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
    noIndex: docsEnvironment !== 'production',
  };
}
