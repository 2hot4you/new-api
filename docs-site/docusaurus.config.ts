import type { Config, Plugin } from '@docusaurus/types';

import { resolvePublicConfig } from './src/config';

const publicConfig = resolvePublicConfig({
  ...process.env,
  DOCS_ENV: process.env.DOCS_ENV ?? 'development',
  DOCS_SITE_URL: process.env.DOCS_SITE_URL ?? 'http://127.0.0.1:3100',
  DOCS_BASE_URL: process.env.DOCS_BASE_URL ?? '/',
  DOCS_API_BASE_URL: process.env.DOCS_API_BASE_URL ?? 'http://127.0.0.1:3000',
});

type WebpackRule = {
  test?: RegExp;
  exclude?: unknown;
};

const openApiCommonJsPattern =
  /node_modules[\\/]docusaurus-(?:plugin|theme)-openapi-docs[\\/]lib[\\/]/;

function preserveOpenApiPackagesCommonJs(): Plugin<void> {
  return {
    name: 'preserve-openapi-packages-commonjs',
    configureWebpack(config, isServer) {
      const javaScriptRule = config.module?.rules?.find((candidate) => {
        if (!candidate || typeof candidate !== 'object' || !('test' in candidate)) return false;
        return String(candidate.test) === String(/\.[jt]sx?$/i);
      }) as WebpackRule | undefined;

      if (!javaScriptRule || typeof javaScriptRule.exclude !== 'function') {
        throw new Error('Unable to locate the Docusaurus JavaScript transpilation rule');
      }

      const defaultExclude = javaScriptRule.exclude as (modulePath: string) => boolean;
      // OpenAPI Docs 5.1.3 mixes TypeScript-compiled CommonJS with copied JSX/ESM.
      // Docusaurus's default transform-runtime pass injects ESM helpers into those
      // CommonJS files, so transpile both packages without transform-runtime.
      javaScriptRule.exclude = (modulePath: string) =>
        openApiCommonJsPattern.test(modulePath) || defaultExclude(modulePath);

      return {
        module: {
          rules: [
            {
              test: /\.jsx?$/i,
              include: openApiCommonJsPattern,
              use: {
                loader: 'babel-loader',
                options: {
                  babelrc: false,
                  configFile: false,
                  presets: [
                    [
                      '@babel/preset-env',
                      isServer
                        ? { targets: { node: 'current' } }
                        : {
                            corejs: '3',
                            exclude: ['transform-typeof-symbol'],
                            loose: true,
                            modules: false,
                            useBuiltIns: 'entry',
                          },
                    ],
                    ['@babel/preset-react', { runtime: 'automatic' }],
                  ],
                },
              },
            },
          ],
        },
      };
    },
  };
}

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
      },
    ],
  ],
  plugins: [
    preserveOpenApiPackagesCommonJs,
    [
      '@docusaurus/plugin-content-docs',
      {
        id: 'api',
        path: 'generated/api',
        routeBasePath: 'api-reference',
        docItemComponent: '@theme/ApiItem',
      },
    ],
    [
      'docusaurus-plugin-openapi-docs',
      {
        id: 'api',
        docsPluginId: 'api',
        config: {
          relay: {
            specPath: 'generated/openapi/relay.public.json',
            outputDir: 'generated/api',
            sidebarOptions: {
              groupPathsBy: 'tag',
            },
          },
        },
      },
    ],
  ],
  themes: [
    'docusaurus-theme-openapi-docs',
    ['@cmfcmf/docusaurus-search-local', { indexBlog: false }],
  ],
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
        { label: 'API 参考', to: '/api-reference/molii-public-api' },
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
            { label: 'API 参考', to: '/api-reference/molii-public-api' },
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
