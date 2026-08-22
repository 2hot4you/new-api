import { expect, test } from 'bun:test';
import { resolve } from 'node:path';

const checkerPath = resolve(import.meta.dirname, 'check-links.sh');
const browserTestPath = resolve(import.meta.dirname, 'api-reference.browser.test.ts');

test('the internal link checker serves the configured base path and crawls from quick start', async () => {
  const checker = await Bun.file(checkerPath).text();

  expect(checker).toContain('DOCS_BASE_URL');
  expect(checker).toContain('docs_base_url');
  expect(checker).toContain('site_url="http://127.0.0.1:3100${docs_base_url}"');
  expect(checker).toContain('crawl_url="${site_url}quick-start"');
  expect(checker).toContain('curl --fail --silent "$crawl_url"');
  expect(checker).toContain('linkinator "$crawl_url"');
  expect(checker).not.toContain("site_url='http://127.0.0.1:3100/'");
});

test('the internal link checker rejects URL-shaped base paths', async () => {
  const checker = await Bun.file(checkerPath).text();

  expect(checker).toContain("DOCS_BASE_URL must be a path.");
  expect(checker).toMatch(/\*:\/\/\*\|\*\\\?\*\|\*\\#\*/);
});

test('the browser smoke test honors the configured documentation base path', async () => {
  const browserTest = await Bun.file(browserTestPath).text();

  expect(browserTest).toContain('process.env.DOCS_BASE_URL');
  expect(browserTest).toContain('normalizedBasePath');
});
