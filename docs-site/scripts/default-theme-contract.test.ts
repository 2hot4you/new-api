import { describe, expect, test } from 'bun:test';
import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';

const siteRoot = join(import.meta.dir, '..');

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

describe('Docusaurus default-theme contract', () => {
  test('uses official Docusaurus shell components without custom replacements', async () => {
    const config = await source('docusaurus.config.ts');
    const fonts = await source('src/css/fonts.css');

    expect(config).toContain("customCss: './src/css/fonts.css'");
    expect(fonts).toContain("@import '@fontsource-variable/lora';");
    await expect(access(join(siteRoot, 'src/css/shell.css'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/theme/Footer/Layout/index.tsx'), constants.F_OK)).rejects.toThrow();
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
    const config = await source('docusaurus.config.ts');

    expect(config).toContain("'@cmfcmf/docusaurus-search-local'");
    expect(config).not.toContain('docusaurus-plugin-openapi-docs');
    expect(config).not.toContain('docusaurus-theme-openapi-docs');
    expect(config).not.toContain("docItemComponent: '@theme/ApiItem'");
    expect(config).not.toContain('preserveOpenApiPackagesCommonJs');
    expect(config).toContain('New API（QuantumNous）');
    expect(config).toContain("defaultMode: 'light'");
    expect(config).toContain('disableSwitch: true');
    expect(config).toContain('respectPrefersColorScheme: false');
    expect(config).not.toContain("title: 'Molii'");
    expect(config).toContain("src: 'img/molii-wordmark.png'");
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
    const config = await source('docusaurus.config.ts');
    const [docsFavicon, appFavicon] = await Promise.all([
      readFile(join(siteRoot, 'static/img/molii-favicon-32.png')),
      readFile(join(siteRoot, '../web/public/molii-favicon-32.png')),
    ]);

    expect(config).toContain("favicon: 'img/molii-favicon-32.png?v=4'");
    expect(config).not.toContain("favicon: 'img/molii-mark.svg'");
    expect(docsFavicon.equals(appFavicon)).toBe(true);
  });

  test('removes the standalone portal so the server owns the root redirect', async () => {
    await expect(access(join(siteRoot, 'src/pages/index.tsx'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/pages/index.module.css'), constants.F_OK)).rejects.toThrow();
  });

  test('uses the concise default Docusaurus footer', async () => {
    const config = await source('docusaurus.config.ts');

    expect(config).toContain("title: '开发者资源'");
    for (const label of ['快速开始', 'API 参考', '帮助']) {
      expect(config).toContain(`label: '${label}'`);
    }
    for (const removedTitle of ['产品', '开发者', '厂商', '支持']) {
      expect(config).not.toContain(`title: '${removedTitle}'`);
    }
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
