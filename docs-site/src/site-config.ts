import type { Config } from '@docusaurus/types';

import type { PublicConfig } from './config';

export function createSiteConfig(publicConfig: PublicConfig): Config {
  const brand = publicConfig.brand;

  return {
    title: brand.siteTitle,
    tagline: brand.tagline,
    favicon: brand.faviconPath,
    url: publicConfig.siteUrl,
    baseUrl: publicConfig.baseUrl,
    organizationName: 'molii',
    projectName: 'developer-docs',
    onBrokenLinks: 'throw',
    onBrokenAnchors: 'throw',
    onDuplicateRoutes: 'throw',
    noIndex: publicConfig.noIndex,
    markdown: {
      hooks: {
        onBrokenMarkdownImages: 'throw',
        onBrokenMarkdownLinks: 'throw',
      },
    },
    i18n: {
      defaultLocale: 'zh-Hans',
      locales: ['zh-Hans'],
    },
    plugins: [
      function docsBrandFontPlugin() {
        return {
          name: 'docs-brand-font',
          injectHtmlTags() {
            return {
              headTags: [
                {
                  tagName: 'script',
                  innerHTML: `document.documentElement.dataset.docsFont = '${brand.defaultFont}';`,
                },
              ],
            };
          },
        };
      },
    ],
    presets: [
      [
        'classic',
        {
          docs: {
            routeBasePath: '/',
            sidebarPath: './sidebars.ts',
          },
          blog: false,
          theme: {
            customCss: './src/css/fonts.css',
          },
        },
      ],
    ],
    themes: publicConfig.algolia
      ? []
      : [['@cmfcmf/docusaurus-search-local', { indexBlog: false, language: ['zh'] }]],
    themeConfig: {
      ...(publicConfig.algolia
        ? {
            algolia: publicConfig.algolia,
          }
        : {}),
      image: brand.socialImagePath,
      colorMode: {
        defaultMode: 'light',
        disableSwitch: true,
        respectPrefersColorScheme: false,
      },
      navbar: {
        title: brand.id === 'molii' ? undefined : brand.navbarTitle,
        logo: {
          alt: brand.navbarTitle,
          src: brand.logoPath,
          href: publicConfig.siteUrl,
          target: '_self',
        },
        items: [
          { label: '开始使用', to: '/quick-start' },
          { label: '平台与账户', to: '/platform' },
          { label: '开发指南', to: '/api-basics' },
          { label: '模型与能力', to: '/models' },
          { label: 'API 参考', to: '/api-reference' },
          { label: '帮助与更新', to: '/help' },
        ],
      },
    },
    customFields: {
      apiBaseUrl: publicConfig.apiBaseUrl,
      docsBrand: brand,
      noIndex: publicConfig.noIndex,
    },
  };
}
