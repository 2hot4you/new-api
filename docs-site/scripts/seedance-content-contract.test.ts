import { describe, expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const sourceCommit = '8928d7bb8b95';
const pages = [
  'docs/models/overview.mdx',
  'docs/models/seedance-2.mdx',
  'docs/guides/seedance-multimodal.mdx',
  'docs/guides/temporary-assets.mdx',
  'docs/examples/seedance-curl.mdx',
  'docs/examples/seedance-python.mdx',
  'docs/examples/seedance-typescript.mdx',
];

async function page(relativePath: string): Promise<string> {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

function frontmatter(source: string): Record<string, string> {
  const match = source.match(/^---\n([\s\S]*?)\n---/);
  if (!match) return {};

  return Object.fromEntries(
    match[1].split('\n').map((line) => {
      const separator = line.indexOf(':');
      return [line.slice(0, separator).trim(), line.slice(separator + 1).trim()];
    }),
  );
}

function fencedCode(source: string, language: string): string {
  return source.match(new RegExp(`\\\`\\\`\\\`${language}\\n([\\s\\S]*?)\\n\\\`\\\`\\\``))?.[1] ?? '';
}

describe('Seedance and temporary asset documentation contract', () => {
  test('all Task 4 pages carry user-facing API provenance metadata', async () => {
    for (const relativePath of pages) {
      const metadata = frontmatter(await page(relativePath));
      expect(metadata.audience, relativePath).toBe('user');
      expect(metadata.apiVersion, relativePath).toBe('v1');
      expect(metadata.lastReviewed, relativePath).toBe('2026-08-10');
      expect(metadata.sourceCommit, relativePath).toBe(sourceCommit);
    }
  });

  test('model pages document both registered models and their resolution boundary', async () => {
    const overview = await page('docs/models/overview.mdx');
    const seedance = await page('docs/models/seedance-2.mdx');
    const combined = `${overview}\n${seedance}`;

    expect(combined).toContain('doubao-seedance-2-0-260128');
    expect(combined).toContain('doubao-seedance-2-0-fast-260128');
    expect(seedance).toMatch(/标准版[^\n]*(?:480p|`480p`)[^\n]*(?:720p|`720p`)[^\n]*(?:1080p|`1080p`)[^\n]*(?:4k|`4k`)/i);
    expect(seedance).toMatch(/Fast[^\n]*(?:480p|`480p`)[^\n]*(?:720p|`720p`)/i);
    expect(seedance).toMatch(/Fast[^。\n]*(?:不支持|不可使用)[^。\n]*(?:1080p|`1080p`)[^。\n]*(?:4k|`4k`)/i);
  });

  test('Seedance parameter guide records defaults, enums, and supported tools', async () => {
    const seedance = await page('docs/models/seedance-2.mdx');

    expect(seedance).toMatch(/duration[^\n]*(?:-1|`-1`)[^\n]*(?:4[^\n]*15|4–15)/i);
    for (const ratio of ['adaptive', '16:9', '4:3', '1:1', '3:4', '9:16', '21:9']) {
      expect(seedance).toContain(ratio);
    }
    expect(seedance).toMatch(/generate_audio[^\n]*true/i);
    expect(seedance).toMatch(/watermark[^\n]*false/i);
    expect(seedance).toContain('web_search');
    expect(seedance).toContain('<ParameterTable');
  });

  test('multimodal guide covers roles, counts, valid requests, and rejected combinations', async () => {
    const multimodal = await page('docs/guides/seedance-multimodal.mdx');

    for (const role of ['first_frame', 'last_frame', 'reference_image', 'reference_video', 'reference_audio']) {
      expect(multimodal).toContain(role);
    }
    expect(multimodal).toMatch(/9[^\n]*(?:图片|图像)|(?:图片|图像)[^\n]*9/);
    expect(multimodal).toMatch(/3[^\n]*视频|视频[^\n]*3/);
    expect(multimodal).toMatch(/3[^\n]*音频|音频[^\n]*3/);
    expect(multimodal).toMatch(/首帧[^。\n]*尾帧[^。\n]*(?:不能|不可|互斥)[^。\n]*(?:参考|多模态)|(?:参考|多模态)[^。\n]*(?:不能|不可|互斥)[^。\n]*首帧/);
    expect(multimodal).toMatch(/音频[^。\n]*(?:不能|不可)[^。\n]*(?:单独|独立)/);
    expect(multimodal).toContain('有效示例');
    expect(multimodal).toContain('无效示例');
  });

  test('temporary asset guide documents public lifecycle without promising a fixed TTL', async () => {
    const assets = await page('docs/guides/temporary-assets.mdx');

    expect(assets).toContain('POST /v1/assets');
    expect(assets).toContain('GET /v1/assets/{id}');
    expect(assets).toContain('DELETE /v1/assets/{id}');
    expect(assets).toContain('asset://asset-molii-');
    for (const type of ['image', 'video', 'audio']) expect(assets).toContain(type);
    for (const state of ['PROCESSING', 'ACTIVE', 'SUCCESS', 'FAILED', 'EXPIRED']) {
      expect(assets).toContain(state);
    }
    expect(assets).toContain('expires_at');
    expect(assets).toMatch(/expires_at[^。\n]*(?:为准|读取|响应)/);
    expect(assets).not.toMatch(/固定\s*24\s*(?:小时|h)/i);
    expect(assets).toMatch(/当前账户|同一账户/);
    expect(assets).toMatch(/公网[^。\n]*(?:HTTP|HTTPS)/i);
    expect(assets).toMatch(/localhost|私有网络|内网/);
    expect(assets).toMatch(/直接访问|可访问|可达/);
  });

  test('language examples submit the paid task once and poll with bounded safe GET requests', async () => {
    const examples = [
      ['docs/examples/seedance-curl.mdx', 'bash'],
      ['docs/examples/seedance-python.mdx', 'python'],
      ['docs/examples/seedance-typescript.mdx', 'ts'],
    ] as const;

    for (const [relativePath, language] of examples) {
      const source = await page(relativePath);
      const code = fencedCode(source, language);
      expect(code, relativePath).toContain('MOLII_API_KEY');
      expect(code, relativePath).toContain('MOLII_API_BASE_URL');
      expect(code, relativePath).toContain('doubao-seedance-2-0-260128');
      expect(code, relativePath).toContain('/v1/videos');
      expect(code, relativePath).toMatch(/queued/);
      expect(code, relativePath).toMatch(/in_progress/);
      expect(code, relativePath).toMatch(/completed/);
      expect(code, relativePath).toMatch(/failed/);
      expect(code, relativePath).toMatch(/unexpected task status/i);
      expect(code, relativePath).toMatch(/Retry-After|retry-after/i);
      expect(code, relativePath).toMatch(/429/);
      expect(code, relativePath).toMatch(/500|5\d\d|>=\s*500/);
      expect(code, relativePath).toMatch(/deadline/i);
      expect(code, relativePath).toContain('/content');
      expect(code, relativePath).toMatch(/Content-Type|content-type/i);
    }

    const shell = fencedCode(await page(examples[0][0]), 'bash');
    const python = fencedCode(await page(examples[1][0]), 'python');
    const typeScript = fencedCode(await page(examples[2][0]), 'ts');
    expect(shell.match(/--request POST/g)?.length ?? 0).toBe(1);
    expect(python.match(/\.post\(/g)?.length ?? 0).toBe(1);
    expect(typeScript.match(/method:\s*'POST'/g)?.length ?? 0).toBe(1);
    expect(python).toContain('follow_redirects=False');
    expect(typeScript).toContain("redirect: 'error'");
  });

  test('sidebar exposes models, guides, and per-language examples', () => {
    const serialized = JSON.stringify(sidebars);
    for (const id of [
      'models/overview',
      'models/seedance-2',
      'guides/seedance-multimodal',
      'guides/temporary-assets',
      'examples/seedance-curl',
      'examples/seedance-python',
      'examples/seedance-typescript',
    ]) {
      expect(serialized).toContain(id);
    }
  });

  test('Task 4 public pages do not expose internal platform concepts or secret-like values', async () => {
    const sources = await Promise.all(pages.map(page));
    const combined = sources.join('\n');

    expect(combined).not.toMatch(/StarAI|lfxqai|channel_id|upstream_id|\/api\/channel|\/api\/admin|Redis|PostgreSQL|MySQL|SQLite/iu);
    expect(combined).not.toMatch(/\bsk-[A-Za-z0-9_-]{16,}\b/);
  });
});
