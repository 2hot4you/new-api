import { expect, test } from 'bun:test';
import { resolve } from 'node:path';

const checkerPath = resolve(import.meta.dirname, 'check-links.sh');

test('the internal link checker serves and crawls the configured base path', async () => {
  const checker = await Bun.file(checkerPath).text();

  expect(checker).toContain('DOCS_BASE_URL');
  expect(checker).toContain('docs_base_url');
  expect(checker).toContain('site_url="http://127.0.0.1:3100${docs_base_url}"');
  expect(checker).not.toContain("site_url='http://127.0.0.1:3100/'");
});

test('the internal link checker rejects URL-shaped base paths', async () => {
  const checker = await Bun.file(checkerPath).text();

  expect(checker).toContain("DOCS_BASE_URL must be a path.");
  expect(checker).toMatch(/\*:\/\/\*\|\*\\\?\*\|\*\\#\*/);
});
