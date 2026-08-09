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
  favicon: 'img/molii-mark.svg',
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
          customCss: './src/css/custom.css',
        },
      },
    ],
  ],
  themes: [['@cmfcmf/docusaurus-search-local', { indexBlog: false }]],
  themeConfig: {
    image: 'img/molii-mark.svg',
    navbar: {
      title: 'Molii',
      logo: {
        alt: 'Molii',
        src: 'img/molii-mark.svg',
      },
      items: [
        { label: '快速开始', to: '/quick-start' },
        { label: '平台', to: '/platform' },
        { label: 'API 基础', to: '/api-basics' },
        { label: '模型', to: '/models' },
        { label: 'API 参考', to: '/api-reference' },
        { label: '示例', to: '/examples' },
        { label: '帮助', to: '/help' },
        { label: '更新日志', to: '/changelog' },
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
      copyright: `Copyright © ${new Date().getFullYear()} Molii. 保留所有权利。`,
    },
  },
  customFields: {
    apiBaseUrl: publicConfig.apiBaseUrl,
    noIndex: publicConfig.noIndex,
  },
};

export default config;
