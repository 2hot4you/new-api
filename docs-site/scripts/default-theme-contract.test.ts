import { describe, expect, test } from 'bun:test';
import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';

const siteRoot = join(import.meta.dir, '..');

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

describe('Docusaurus default-theme contract', () => {
  test('uses the stock Infima theme with only the shared font stylesheet and no React landing page', async () => {
    const config = await source('docusaurus.config.ts');
    const fonts = await source('src/css/fonts.css');

    expect(config).toContain("customCss: './src/css/fonts.css'");
    expect(fonts).toContain("@import '@fontsource-variable/lora';");
    expect(fonts).not.toMatch(/(?:^|[;{])\s*(?:color|background|border|margin|padding|gap|display|position|width|height|shadow)\s*:/i);
    await expect(access(join(siteRoot, 'src/css/custom.css'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/pages/index.tsx'), constants.F_OK)).rejects.toThrow();
    await expect(access(join(siteRoot, 'src/pages/index.module.css'), constants.F_OK)).rejects.toThrow();
  });

  test('serves the root route as a plain default Docs page with every primary destination', async () => {
    const index = await source('docs/index.mdx');

    expect(index).toContain('slug: /');
    expect(index).not.toMatch(/<style|className=|gradient|animation/i);
    for (const [label, route] of [
      ['快速开始', '/quick-start'],
      ['平台', '/platform'],
      ['API 基础', '/api-basics'],
      ['模型', '/models'],
      ['API 参考', '/api-reference'],
      ['示例', '/examples'],
      ['帮助', '/help'],
      ['更新日志', '/changelog'],
    ]) {
      expect(index).toContain(`[${label}](${route})`);
    }
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
    for (const label of ['快速开始', '平台', 'API 基础', '模型', 'API 参考', '示例', '帮助', '更新日志']) {
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
