import { describe, expect, test } from 'bun:test';
import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';

const siteRoot = join(import.meta.dir, '..');

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

describe('Docusaurus default-theme contract', () => {
  test('keeps global CSS limited to typography while the portal owns isolated styles', async () => {
    const config = await source('docusaurus.config.ts');
    const fonts = await source('src/css/fonts.css');

    expect(config).toContain("customCss: './src/css/fonts.css'");
    expect(fonts).toContain("@import '@fontsource-variable/lora';");
    expect(fonts).not.toMatch(/(?:^|[;{])\s*(?:color|background|border|margin|padding|gap|display|position|width|height|shadow)\s*:/i);
    await expect(access(join(siteRoot, 'src/css/custom.css'), constants.F_OK)).rejects.toThrow();
  });

  test('uses only the default Docs renderer for guides and API reference pages', async () => {
    const config = await source('docusaurus.config.ts');

    expect(config).toContain("'@cmfcmf/docusaurus-search-local'");
    expect(config).not.toContain('docusaurus-plugin-openapi-docs');
    expect(config).not.toContain('docusaurus-theme-openapi-docs');
    expect(config).not.toContain("docItemComponent: '@theme/ApiItem'");
    expect(config).not.toContain('preserveOpenApiPackagesCommonJs');
    expect(config).toContain('New API（QuantumNous）');
    expect(config).toContain("{ label: 'API 参考', to: '/api-reference' }");
    for (const label of ['开始使用', '平台与账户', '开发指南', '模型与能力', 'API 参考', '帮助与更新']) {
      expect(config).toContain(`label: '${label}'`);
    }
  });

  test('registers API reference pages in the ordinary Docs sidebar', async () => {
    const sidebar = await source('sidebars.ts');

    for (const id of [
      'api-reference/index',
      'api-reference/models',
      'api-reference/images',
      'api-reference/videos',
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
