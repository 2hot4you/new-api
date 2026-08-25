import type { Config } from '@docusaurus/types';

import { resolvePublicConfig } from './src/config';

const publicConfig = resolvePublicConfig({
  ...process.env,
  DOCS_ENV: process.env.DOCS_ENV ?? 'development',
  DOCS_SITE_URL: process.env.DOCS_SITE_URL ?? 'http://127.0.0.1:3100',
  DOCS_BASE_URL: process.env.DOCS_BASE_URL ?? '/',
  DOCS_API_BASE_URL: process.env.DOCS_API_BASE_URL ?? 'http://127.0.0.1:3000',
});

const config: Config = {
  title: 'Molii 开发者文档',
  tagline: '构建可靠、可扩展的 AI 创作体验',
  favicon: 'img/molii-favicon-32.png?v=4',
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
    image: 'img/molii-mark.svg',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: true,
      respectPrefersColorScheme: false,
    },
    navbar: {
      logo: {
        alt: 'Molii',
        src: 'img/molii-wordmark.png',
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
    footer: {
      style: 'dark',
      links: [
        {
          title: '开发者资源',
          items: [
            { label: '快速开始', to: '/quick-start' },
            { label: 'API 参考', to: '/api-reference' },
            { label: '帮助', to: '/help' },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Molii. 保留所有权利。基于 New API（QuantumNous）构建。`,
    },
  },
  customFields: {
    apiBaseUrl: publicConfig.apiBaseUrl,
    noIndex: publicConfig.noIndex,
  },
};

export default config;
