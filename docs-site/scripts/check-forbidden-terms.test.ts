import { afterEach, describe, expect, test } from 'bun:test';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { scanForbiddenTerms } from './check-forbidden-terms.mjs';

const workspaces: string[] = [];

afterEach(async () => {
  await Promise.all(workspaces.splice(0).map((workspace) => rm(workspace, { recursive: true, force: true })));
});

async function fixture(source: string) {
  const workspace = await mkdtemp(join(tmpdir(), 'molii-forbidden-terms-'));
  workspaces.push(workspace);
  const docsDirectory = join(workspace, 'docs');
  const termsPath = join(workspace, 'forbidden-terms.txt');
  const filePath = join(docsDirectory, 'guide.mdx');

  await mkdir(docsDirectory);
  await writeFile(termsPath, '/channels\n上游\n管理员\n');
  await writeFile(filePath, source);

  return { docsDirectory, filePath, termsPath };
}

describe('forbidden-term scanner', () => {
  test('reports administrator paths, provider-internal terms, internal domains, and realistic secrets with exact locations', async () => {
    const files = await fixture([
      '# 用户文档',
      '不要访问 /channels。',
      '不要暴露上游供应商信息。',
      '内部域名 api.internal.molii.example 不属于公开接口。',
      '示例密钥 sk-proj-abcdefghijklmnopqrstuvwxyz123456。',
    ].join('\n'));

    await expect(scanForbiddenTerms(files)).resolves.toEqual([
      `${files.filePath}:2: forbidden term "/channels"`,
      `${files.filePath}:3: forbidden term "上游"`,
      `${files.filePath}:4: internal domain "api.internal.molii.example"`,
      `${files.filePath}:5: realistic secret`,
    ]);
  });

  test('allows documented placeholders and New API / QuantumNous attribution', async () => {
    const files = await fixture([
      '使用 `Bearer $MOLII_API_KEY` 和 `your-api-key` 作为占位符。',
      '媒体 URL 可以使用 https://example.invalid/media.png。',
      'Molii 基于 New API（QuantumNous）构建。',
    ].join('\n'));

    await expect(scanForbiddenTerms(files)).resolves.toEqual([]);
  });
});
