import { describe, expect, test } from 'bun:test';
import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';

const siteRoot = join(import.meta.dir, '..');

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

describe('Docusaurus default-theme contract', () => {
  test('uses the official Docusaurus shell with a scoped footer replacement', async () => {
    const config = await source('src/site-config.ts');
    const fonts = await source('src/css/fonts.css');

    expect(config).toContain("customCss: './src/css/fonts.css'");
    expect(fonts).toContain("@import '@fontsource-variable/lora';");
    await expect(access(join(siteRoot, 'src/css/shell.css'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/theme/Footer/index.tsx'), constants.F_OK)).resolves.toBeNull();
    await expect(access(join(siteRoot, 'src/theme/Footer/styles.module.css'), constants.F_OK)).resolves.toBeNull();
    await expect(access(join(siteRoot, 'src/css/custom.css'), constants.F_OK)).rejects.toThrow();
  });

  test('clears cached base-path assets before every production build', async () => {
    const packageJson = JSON.parse(await source('package.json')) as {
      scripts?: Record<string, string>;
    };

    expect(packageJson.scripts?.prebuild).toContain('docusaurus clear');
    expect(packageJson.scripts?.prebuild).toContain('catalog:generate');
  });

  test('uses only the default Docs renderer for guides and API reference pages', async () => {
    const config = await source('src/site-config.ts');

    expect(config).toContain("'@cmfcmf/docusaurus-search-local'");
    expect(config).not.toContain('docusaurus-plugin-openapi-docs');
    expect(config).not.toContain('docusaurus-theme-openapi-docs');
    expect(config).not.toContain("docItemComponent: '@theme/ApiItem'");
    expect(config).not.toContain('preserveOpenApiPackagesCommonJs');
    expect(config).toContain("defaultMode: 'light'");
    expect(config).toContain('disableSwitch: true');
    expect(config).toContain('respectPrefersColorScheme: false');
    expect(config).not.toContain("title: 'Molii'");
    expect(config).toContain('src: brand.logoPath');
    expect(config).toContain("title: brand.id === 'molii' ? undefined : brand.navbarTitle");
    for (const label of ['开始使用', '平台与账户', '开发指南', '模型与能力', 'API 参考', '帮助与更新']) {
      expect(config).toContain(`label: '${label}'`);
    }
    for (const removed of ['主页', '控制台', '模型广场', '排行榜', '文档', '关于']) {
      expect(config).not.toContain(`label: '${removed}'`);
    }
  });

  test('uses the exact New API wordmark asset without changing the default navbar component', async () => {
    const [docsWordmark, appWordmark] = await Promise.all([
      readFile(join(siteRoot, 'static/img/molii-wordmark.png')),
      readFile(join(siteRoot, '../web/public/molii-wordmark.png')),
    ]);

    expect(docsWordmark.equals(appWordmark)).toBe(true);
  });

  test('uses the exact New API favicon asset', async () => {
    const config = await source('src/site-config.ts');
    const [docsFavicon, appFavicon] = await Promise.all([
      readFile(join(siteRoot, 'static/img/molii-favicon-32.png')),
      readFile(join(siteRoot, '../web/public/molii-favicon-32.png')),
    ]);

    expect(config).toContain('favicon: brand.faviconPath');
    expect(config).not.toContain("favicon: 'img/molii-mark.svg'");
    expect(docsFavicon.equals(appFavicon)).toBe(true);
  });

  test('removes the standalone portal so the server owns the root redirect', async () => {
    await expect(access(join(siteRoot, 'src/pages/index.tsx'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/pages/index.module.css'), constants.F_OK)).rejects.toThrow();
  });

  test('uses the same five-part information architecture as the Molii home footer', async () => {
    const [config, footer] = await Promise.all([
      source('docusaurus.config.ts'),
      source('src/theme/Footer/index.tsx'),
    ]);

    expect(config).not.toContain('footer: {');
    for (const title of ['产品', '开发者', '厂商', '支持']) {
      expect(footer).toContain(title);
    }
    expect(footer).toContain('通过统一 API Key 连接语言、图片与视频模型');
    expect(footer).toContain('OpenAI、Anthropic 与 Gemini 兼容 API');
    expect(footer).toContain('docsBrand');
    expect(footer).toContain('New API');
    expect(footer).toContain('QuantumNous');
  });

  test('supports site-selected serif and sans typography without changing code fonts', async () => {
    const fonts = await source('src/css/fonts.css');

    expect(fonts).toContain("[data-docs-font='serif']");
    expect(fonts).toContain("[data-docs-font='sans']");
    expect(fonts).toContain(':is(code, kbd, pre, samp)');
  });

  test('keeps platform links at the origin root and docs links under the configured base path', async () => {
    const footer = await source('src/theme/Footer/index.tsx');

    expect(footer).toContain("platformUrl('/pricing')");
    expect(footer).toContain("useBaseUrl('/quick-start')");
    expect(footer).toContain("useBaseUrl('/api-reference')");
  });

  test('registers API reference pages in the ordinary Docs sidebar', async () => {
    const sidebar = await source('sidebars.ts');

    for (const id of [
      'api-reference/index',
      'api-reference/models',
      'api-reference/images',
      'api-reference/videos',
      'api-reference/files',
      'api-reference/seedance',
      'api-reference/assets',
      'api-reference/errors',
    ]) {
      expect(sidebar).toContain(`'${id}'`);
    }
  });

  test('documentation components rely on the stock Docusaurus table and list styling', async () => {
    for (const relativePath of ['src/components/ApiLifecycle.tsx', 'src/components/ParameterTable.tsx']) {
      const component = await source(relativePath);
      expect(component, relativePath).not.toContain('style=');
      expect(component, relativePath).not.toContain('className=');
    }
  });
});
