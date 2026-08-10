import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { chromium } from 'playwright-core';

import { resolveBrowserExecutable } from './browser-executable.mjs';

const siteRoot = new URL('..', import.meta.url).pathname;
const port = 3197;
const baseUrl = `http://127.0.0.1:${port}`;
let server: ReturnType<typeof Bun.spawn> | undefined;

beforeAll(async () => {
  server = Bun.spawn(
    ['bun', 'x', 'docusaurus', 'start', '--host', '127.0.0.1', '--port', String(port), '--no-open'],
    { cwd: siteRoot, stdout: 'ignore', stderr: 'pipe' },
  );

  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Docusaurus development server exited with ${server.exitCode}`);
    }
    try {
      const response = await fetch(`${baseUrl}/api-reference`);
      if (response.ok) return;
    } catch {
      // Development server is still compiling.
    }
    await Bun.sleep(100);
  }
  throw new Error('Docusaurus development server did not become ready');
}, 35_000);

afterAll(async () => {
  server?.kill();
  await server?.exited;
});

describe('default MDX API reference', () => {
  test('renders the API overview with the stock Docs layout and no OpenAPI explorer', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.stack ?? error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });

    try {
      await page.goto(`${baseUrl}/api-reference`, { waitUntil: 'networkidle' });
      await expect(page.locator('main .theme-doc-markdown h1').filter({ hasText: 'API 参考' }).count())
        .resolves.toBe(1);
      await expect(page.locator('.theme-doc-sidebar-container').count()).resolves.toBe(1);
      await expect(page.locator('.table-of-contents').count()).resolves.toBe(1);
      await expect(page.locator('.pagination-nav').count()).resolves.toBe(1);
      await expect(page.locator('[class*="openapi"], [class*="api-explorer"]').count()).resolves.toBe(0);
      expect(errors).toEqual([]);
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('renders method-style image endpoints as ordinary headings, tables, and code blocks', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

    try {
      await page.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' });
      await expect(page.locator('h2').filter({ hasText: 'POST /v1/images/generations' }).count())
        .resolves.toBe(1);
      await expect(page.locator('h2').filter({ hasText: 'POST /v1/images/edits' }).count())
        .resolves.toBe(1);
      expect(await page.locator('main table').count()).toBeGreaterThanOrEqual(4);
      expect(await page.locator('main pre').count()).toBeGreaterThanOrEqual(4);
      await expect(page.locator('[class*="openapi"], [class*="api-explorer"]').count()).resolves.toBe(0);
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('uses the New API Serif font for prose while preserving monospace code', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

    try {
      await page.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' });
      const bodyFont = await page.locator('body').evaluate((element) => getComputedStyle(element).fontFamily);
      const headingFont = await page.locator('main h1').evaluate((element) => getComputedStyle(element).fontFamily);
      const codeFont = await page.locator('main pre code').first().evaluate((element) => getComputedStyle(element).fontFamily);

      expect(bodyFont).toStartWith('"Lora Variable"');
      expect(headingFont).toStartWith('"Lora Variable"');
      expect(codeFont).not.toContain('Lora');
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('uses compact responsive root typography without page overflow', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const desktop = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });

    try {
      await Promise.all([
        desktop.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' }),
        mobile.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' }),
      ]);

      const desktopLayout = await desktop.evaluate(() => ({
        rootFontSize: getComputedStyle(document.documentElement).fontSize,
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      }));
      const mobileLayout = await mobile.evaluate(() => ({
        rootFontSize: getComputedStyle(document.documentElement).fontSize,
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      }));

      expect(desktopLayout).toEqual({ rootFontSize: '12px', overflows: false });
      expect(mobileLayout).toEqual({ rootFontSize: '14px', overflows: false });
    } finally {
      await browser.close();
    }
  }, 30_000);
});
