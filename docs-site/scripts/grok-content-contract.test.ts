import { describe, expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const redirectFollowingOption = /(?:^|\s)(?:--location(?:-trusted)?|-L)(?:\s|$)/;
const pages = [
  'docs/models/grok-imagine-image.mdx',
  'docs/models/grok-imagine-video.mdx',
  'docs/examples/grok-image-curl.mdx',
  'docs/examples/grok-video-curl.mdx',
  'docs/examples/grok-poll-download.mdx',
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

describe('Grok Imagine public documentation contract', () => {
  test('all Grok pages carry user-facing API provenance metadata', async () => {
    for (const relativePath of pages) {
      const metadata = frontmatter(await page(relativePath));
      expect(metadata.audience, relativePath).toBe('user');
      expect(metadata.apiVersion, relativePath).toBe('v1');
      expect(metadata.lastReviewed, relativePath).toMatch(/^2026-08-(10|11)$/);
      expect(metadata.sourceCommit, relativePath).toMatch(/^[0-9a-f]{7,40}$/);
    }
  });

  test('image guide freezes models, request limits, URL-only edits, and direct billing', async () => {
    const source = await page('docs/models/grok-imagine-image.mdx');
    for (const model of ['grok-imagine-image', 'grok-imagine-image-quality', 'grok-imagine-image-2.0']) expect(source).toContain(model);
    for (const endpoint of ['POST /v1/images/generations', 'POST /v1/images/edits']) expect(source).toContain(endpoint);
    for (const ratio of ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3', '2:1', '1:2', '19.5:9', '9:19.5', '20:9', '9:20', 'auto']) {
      expect(source).toContain(ratio);
    }
    expect(source).toMatch(/n[^\n]*1[^\n]*4/i);
    expect(source).toMatch(/(?:输入图|图片)[^\n]*1[–-]3|1[–-]3[^\n]*(?:输入图|图片)/);
    expect(source).toContain('1k');
    expect(source).toContain('2k');
    expect(source).toContain('quality');
    expect(source).toContain('low');
    expect(source).toContain('medium');
    expect(source).toContain('data:image/');
    expect(source).toContain('| 参数 | 类型 | 必填 | 默认值 |');
    expect(source).toMatch(/输出单价[^\n]*×[^\n]*输出数量/);
    expect(source).toMatch(/输入单价[^\n]*×[^\n]*输入图片数量/);
    expect(source).toMatch(/file_id/i);
  });

  test('video guide documents the two distinct model contracts and final billing source', async () => {
    const source = await page('docs/models/grok-imagine-video.mdx');
    expect(source).toContain('grok-imagine-video');
    expect(source).toContain('grok-imagine-video-1.5');
    expect(source).toMatch(/1\.5[^。\n]*(?:必须|仅支持)[^。\n]*(?:图片|图生视频)/);
    expect(source).toMatch(/视频编辑(?:只允许|仅支持)[^。\n]*grok-imagine-video/);
    expect(source).toMatch(/duration[^\n]*1[^\n]*15/i);
    for (const ratio of ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3']) expect(source).toContain(ratio);
    expect(source).toMatch(/grok-imagine-video[^\n]*480p[^\n]*720p/);
    expect(source).toMatch(/grok-imagine-video-1\.5[^\n]*480p[^\n]*720p[^\n]*1080p/);
    expect(source).toMatch(/视频编辑[^。\n]*(?:不接受|不能传)[^。\n]*duration[^。\n]*aspect_ratio[^。\n]*resolution/);
    expect(source).toMatch(/(?:探测|冻结)[^。\n]*分辨率|分辨率[^。\n]*(?:探测|冻结)/);
    for (const state of ['settled', 'refund_pending', 'unavailable']) expect(source).toContain(state);
    expect(source).toMatch(/file_id/i);
  });

  test('curl examples cover every public Grok operation without real credentials', async () => {
    const combined = (await Promise.all(pages.slice(2).map(page))).join('\n');
    for (const endpoint of [
      '/v1/images/generations',
      '/v1/images/edits',
      '/v1/videos',
      '/v1/videos/edits',
      '/v1/videos/$TASK_ID',
      '/v1/videos/$TASK_ID/content',
    ]) expect(combined).toContain(endpoint);
    expect(combined).toContain('MOLII_API_KEY');
    expect(combined).toContain('MOLII_API_BASE_URL');
    expect(combined).toContain('Content-Type: application/json');
    expect(combined).toMatch(/Content-Type[^\n]*video\//i);
    expect(combined).toMatch(/不要自动重试|不得自动重试/);
    expect(combined).not.toMatch(/sk-[A-Za-z0-9]{16,}/);
    expect(combined).not.toMatch(/file_id/i);
  });

  test('paid Grok POST examples do not follow redirects with API authorization', async () => {
    for (const relativePath of [
      'docs/examples/grok-image-curl.mdx',
      'docs/examples/grok-video-curl.mdx',
    ]) {
      const source = await page(relativePath);
      const snippets = [...source.matchAll(/```bash\n(curl[\s\S]*?--request POST[\s\S]*?)\n```/g)];

      expect(snippets.length, relativePath).toBeGreaterThan(0);
      for (const [, snippet] of snippets) {
        expect(snippet, relativePath).toContain('Authorization: Bearer $MOLII_API_KEY');
        expect(snippet, relativePath).not.toMatch(redirectFollowingOption);
      }
    }
  });

  test('paid Grok POST examples normalize a trailing slash before the versioned path', async () => {
    for (const relativePath of [
      'docs/examples/grok-image-curl.mdx',
      'docs/examples/grok-video-curl.mdx',
    ]) {
      const source = await page(relativePath);
      const snippets = [...source.matchAll(/```bash\n(curl[\s\S]*?--request POST[\s\S]*?)\n```/g)];

      expect(snippets.length, relativePath).toBeGreaterThan(0);
      for (const [, snippet] of snippets) {
        expect(snippet, relativePath).toContain('"${MOLII_API_BASE_URL%/}/v1/');
        expect(snippet, relativePath).not.toContain('"$MOLII_API_BASE_URL/v1/');
      }
    }
  });

  test('redirect matcher detects location-trusted on paid POST snippets', () => {
    const snippet = 'curl --location-trusted --include --request POST --header "Authorization: Bearer $MOLII_API_KEY"';

    expect(snippet).toMatch(redirectFollowingOption);
  });

  test('overview and sidebar expose Grok guides while excluding the retired technical preview', async () => {
    const overview = await page('docs/models/overview.mdx');
    const serialized = `${overview}\n${JSON.stringify(sidebars)}`;
    for (const id of [
      'models/grok-imagine-image',
      'models/grok-imagine-video',
      'examples/grok-image-curl',
      'examples/grok-video-curl',
      'examples/grok-poll-download',
    ]) expect(serialized).toContain(id);
    const retiredModel = 'grok-imagine-video-1.5-' + 'pre' + 'view';
    expect((await Promise.all(pages.map(page))).join('\n')).not.toContain(retiredModel);
  });
});
