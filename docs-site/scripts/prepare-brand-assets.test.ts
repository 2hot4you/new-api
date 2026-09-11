import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { downloadPublicAsset, prepareBrandAssets } from './prepare-brand-assets';

const png = Uint8Array.from([137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 0]);
let server: ReturnType<typeof Bun.serve>;
let root: string;

beforeAll(async () => {
  root = await mkdtemp(join(tmpdir(), 'docs-brand-assets-'));
  server = Bun.serve({
    port: 0,
    fetch(request) {
      const path = new URL(request.url).pathname;
      if (path === '/valid.svg') {
        return new Response('<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h1v1z"/></svg>', {
          headers: { 'content-type': 'image/svg+xml' },
        });
      }
      if (path === '/script.svg') {
        return new Response('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>', {
          headers: { 'content-type': 'image/svg+xml' },
        });
      }
      if (path === '/external.svg') {
        return new Response('<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.com/a.png"/></svg>', {
          headers: { 'content-type': 'image/svg+xml' },
        });
      }
      if (path === '/empty.png') {
        return new Response('', { headers: { 'content-type': 'image/png' } });
      }
      if (path === '/wrong.txt') {
        return new Response('not an image', { headers: { 'content-type': 'text/plain' } });
      }
      if (path === '/large.png') {
        return new Response(new Uint8Array(2 * 1024 * 1024 + 1), {
          headers: { 'content-type': 'image/png' },
        });
      }
      if (path === '/redirect') {
        return new Response(null, { status: 302, headers: { location: 'http://example.com/logo.png' } });
      }
      return new Response(png, { headers: { 'content-type': 'image/png' } });
    },
  });
});

afterAll(async () => {
  server.stop(true);
  await rm(root, { force: true, recursive: true });
});

function url(path: string): string {
  return `http://127.0.0.1:${server.port}${path}`;
}

describe('documentation brand assets', () => {
  test('writes valid PNG and sanitized SVG files atomically', async () => {
    await prepareBrandAssets(
      {
        DOCS_ENV: 'development',
        DOCS_SITE_URL: 'http://127.0.0.1:3100',
        DOCS_BASE_URL: '/',
        DOCS_API_BASE_URL: 'http://127.0.0.1:3000',
        DOCS_BRAND_ID: 'ixiaozu',
        DOCS_SITE_TITLE: 'iXiaozu Docs',
        DOCS_TAGLINE: 'Documentation',
        DOCS_NAVBAR_TITLE: 'iXiaozu',
        DOCS_LOGO_PATH: 'img/brand/logo.svg',
        DOCS_FAVICON_PATH: 'img/brand/favicon.png',
        DOCS_SOCIAL_IMAGE_PATH: 'img/brand/social.png',
        DOCS_DEFAULT_FONT: 'sans',
        DOCS_LOGO_SOURCE_URL: url('/valid.svg'),
        DOCS_FAVICON_SOURCE_URL: url('/favicon.png'),
        DOCS_SOCIAL_IMAGE_SOURCE_URL: url('/social.png'),
      },
      root,
    );

    expect(await readFile(join(root, 'img/brand/logo.svg'), 'utf8')).toContain('<path');
    expect(await readFile(join(root, 'img/brand/favicon.png'))).toEqual(Buffer.from(png));
    expect(await readFile(join(root, 'img/brand/social.png'))).toEqual(Buffer.from(png));
  });

  test('requires HTTPS asset sources in production', async () => {
    await expect(
      downloadPublicAsset({
        destination: join(root, 'http.png'),
        environment: 'production',
        source: url('/favicon.png'),
      }),
    ).rejects.toThrow('HTTPS');
  });

  test('rejects redirects, oversized, unsupported, and empty responses', async () => {
    for (const [path, message] of [
      ['/redirect', 'redirect'],
      ['/large.png', '2 MiB'],
      ['/wrong.txt', 'content type'],
      ['/empty.png', 'empty'],
    ] as const) {
      await expect(
        downloadPublicAsset({
          destination: join(root, `${path.slice(1)}.out`),
          environment: 'development',
          source: url(path),
        }),
      ).rejects.toThrow(message);
    }
  });

  test('rejects active or externally-referencing SVG content', async () => {
    for (const path of ['/script.svg', '/external.svg']) {
      await expect(
        downloadPublicAsset({
          destination: join(root, `${path.slice(1)}.out`),
          environment: 'development',
          source: url(path),
        }),
      ).rejects.toThrow('unsafe SVG');
    }
  });
});
