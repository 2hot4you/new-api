import { describe, expect, test } from 'bun:test';
import { access, readFile } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';

const siteRoot = join(import.meta.dir, '..');

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

describe('Docusaurus default-theme contract', () => {
  test('uses the stock Infima theme without a custom stylesheet or React landing page', async () => {
    const config = await source('docusaurus.config.ts');

    expect(config).not.toContain('customCss');
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
      ['API 参考', '/api-reference/molii-public-api'],
      ['示例', '/examples'],
      ['帮助', '/help'],
      ['更新日志', '/changelog'],
    ]) {
      expect(index).toContain(`[${label}](${route})`);
    }
  });

  test('retains the existing navigation, search, attribution, and OpenAPI compatibility wiring', async () => {
    const config = await source('docusaurus.config.ts');

    expect(config).toContain("'@cmfcmf/docusaurus-search-local'");
    expect(config).toContain('docusaurus-plugin-openapi-docs');
    expect(config).toContain('preserveOpenApiPackagesCommonJs');
    expect(config).toContain('New API（QuantumNous）');
    for (const label of ['快速开始', '平台', 'API 基础', '模型', 'API 参考', '示例', '帮助', '更新日志']) {
      expect(config).toContain(`label: '${label}'`);
    }
  });
});
