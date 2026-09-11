import { resolvePublicConfig } from './src/config';
import { createSiteConfig } from './src/site-config';

const publicConfig = resolvePublicConfig({
  ...process.env,
  DOCS_ENV: process.env.DOCS_ENV ?? 'development',
  DOCS_SITE_URL: process.env.DOCS_SITE_URL ?? 'http://127.0.0.1:3100',
  DOCS_BASE_URL: process.env.DOCS_BASE_URL ?? '/',
  DOCS_API_BASE_URL: process.env.DOCS_API_BASE_URL ?? 'http://127.0.0.1:3000',
  DOCS_BRAND_ID: process.env.DOCS_BRAND_ID ?? 'molii',
});

export default createSiteConfig(publicConfig);
