import type { PublicBrandConfig } from './config';

const CLAUDEYE_WORDMARK_FALLBACK = 'img/claudeye-wordmark-neutral.png';

export interface DocsBrandAssets {
  navbarLogo: string;
  footerLogo: string;
  favicon: string;
  fallbackLogo: string;
  dynamic: boolean;
}

function withBaseUrl(baseUrl: string, assetPath: string): string {
  const normalizedBaseUrl = baseUrl.endsWith('/') ? baseUrl : `${baseUrl}/`;
  return `${normalizedBaseUrl}${assetPath.replace(/^\/+/, '')}`;
}

export function resolveDocsBrandAssets(
  brand: PublicBrandConfig,
  apiBaseUrl: string,
  baseUrl: string,
): DocsBrandAssets {
  if (brand.id !== 'claudeye') {
    const logo = withBaseUrl(baseUrl, brand.logoPath);
    return {
      navbarLogo: logo,
      footerLogo: logo,
      favicon: withBaseUrl(baseUrl, brand.faviconPath),
      fallbackLogo: logo,
      dynamic: false,
    };
  }

  return {
    navbarLogo: new URL(
      '/api/branding/claudeye/wordmark.svg?surface=light',
      apiBaseUrl,
    ).href,
    footerLogo: new URL(
      '/api/branding/claudeye/wordmark.svg?surface=dark',
      apiBaseUrl,
    ).href,
    favicon: new URL('/api/branding/claudeye/favicon.svg', apiBaseUrl).href,
    fallbackLogo: withBaseUrl(baseUrl, CLAUDEYE_WORDMARK_FALLBACK),
    dynamic: true,
  };
}
