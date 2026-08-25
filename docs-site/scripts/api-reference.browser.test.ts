import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { chromium, type Browser } from 'playwright-core';

import { resolveBrowserExecutable } from './browser-executable.mjs';

const siteRoot = new URL('..', import.meta.url).pathname;
const port = 3197;
const configuredSiteUrl = process.env.DOCS_SITE_URL ?? 'http://127.0.0.1:3100';
const usesAlgoliaSearch = process.env.DOCS_ENV === 'development' &&
  [
    process.env.DOCS_ALGOLIA_APP_ID,
    process.env.DOCS_ALGOLIA_SEARCH_API_KEY,
    process.env.DOCS_ALGOLIA_INDEX_NAME,
  ].every((value) => Boolean(value?.trim()));
const configuredBasePath = process.env.DOCS_BASE_URL ?? '/';
const normalizedBasePath = configuredBasePath === '/'
  ? ''
  : `/${configuredBasePath.replace(/^\/+|\/+$/g, '')}`;
const baseUrl = `http://127.0.0.1:${port}${normalizedBasePath}`;

function docsRoute(path: string) {
  return `${normalizedBasePath}${path}`;
}
let server: ReturnType<typeof Bun.spawn> | undefined;
let browser: Browser | undefined;

function activeBrowser() {
  if (!browser) throw new Error('Chromium browser is not ready');
  return browser;
}

beforeAll(async () => {
  const clear = Bun.spawn(
    ['bun', 'x', 'docusaurus', 'clear'],
    { cwd: siteRoot, stdout: 'ignore', stderr: 'inherit' },
  );
  const clearExitCode = await clear.exited;
  if (clearExitCode !== 0) {
    throw new Error(`Docusaurus cache cleanup exited with ${clearExitCode}`);
  }

  const build = Bun.spawn(
    ['bun', 'x', 'docusaurus', 'build'],
    { cwd: siteRoot, stdout: 'ignore', stderr: 'inherit' },
  );
  const buildExitCode = await build.exited;
  if (buildExitCode !== 0) {
    throw new Error(`Docusaurus production build exited with ${buildExitCode}`);
  }

  server = Bun.spawn(
    ['bun', 'x', 'docusaurus', 'serve', '--host', '127.0.0.1', '--port', String(port), '--no-open'],
    { cwd: siteRoot, stdout: 'ignore', stderr: 'inherit' },
  );

  const deadline = Date.now() + 10_000;
  let serverReady = false;
  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Docusaurus static server exited with ${server.exitCode}`);
    }
    try {
      const response = await fetch(`${baseUrl}/api-reference`);
      if (response.ok) {
        serverReady = true;
        break;
      }
    } catch {
      // Static server is still starting.
    }
    await Bun.sleep(100);
  }
  if (!serverReady) throw new Error('Docusaurus static server did not become ready');

  const chromePath = await resolveBrowserExecutable();
  browser = await chromium.launch({ executablePath: chromePath, headless: true });
}, 60_000);

afterAll(async () => {
  server?.kill();
  await server?.exited;
  await browser?.close();
}, 15_000);

describe('default MDX API reference', () => {
  test('renders the API overview with the stock Docs layout and no OpenAPI explorer', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 1000 } });
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.stack ?? error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });

    try {
      await page.goto(`${baseUrl}/api-reference`, { waitUntil: 'domcontentloaded' });
      await expect(page.locator('main .theme-doc-markdown h1').filter({ hasText: 'API 参考' }).count())
        .resolves.toBe(1);
      await expect(page.locator('.theme-doc-sidebar-container').count()).resolves.toBe(1);
      await expect(page.locator('.table-of-contents').count()).resolves.toBe(1);
      await expect(page.locator('.pagination-nav').count()).resolves.toBe(1);
      await expect(page.locator('[class*="openapi"], [class*="api-explorer"]').count()).resolves.toBe(0);
      const headings = (await page.locator('main h2').allTextContents())
        .map((heading) => heading.replaceAll('\u200B', ''));
      for (const required of ['开始前确认', '选择端点', '同步与异步响应', '计费与用量字段', '错误与 Request ID']) {
        expect(headings).toContain(required);
      }
      expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(10);
      expect(errors).toEqual([]);
    } finally {
      await page.close();
    }
  }, 30_000);

  test('opens onboarding and account categories by default while keeping them collapsible', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

    try {
      await page.goto(`${baseUrl}/api-reference`, { waitUntil: 'domcontentloaded' });
      const sidebar = page.locator('.theme-doc-sidebar-container');
      const gettingStarted = sidebar.getByRole('button', { name: '开始使用', exact: true });
      const platform = sidebar.getByRole('button', { name: '平台与账户', exact: true });
      const developerGuides = sidebar.getByRole('button', { name: '开发指南', exact: true });

      await expect(gettingStarted.getAttribute('aria-expanded')).resolves.toBe('true');
      await expect(platform.getAttribute('aria-expanded')).resolves.toBe('true');
      await expect(developerGuides.getAttribute('aria-expanded')).resolves.toBe('false');

      await gettingStarted.click();
      await expect(gettingStarted.getAttribute('aria-expanded')).resolves.toBe('false');
    } finally {
      await page.close();
    }
  }, 30_000);

  test('renders method-style image endpoints as ordinary headings, tables, and code blocks', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 1000 } });

    try {
      await page.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'domcontentloaded' });
      await expect(page.locator('h2').filter({ hasText: 'POST /v1/images/generations' }).count())
        .resolves.toBe(1);
      await expect(page.locator('h2').filter({ hasText: 'POST /v1/images/edits' }).count())
        .resolves.toBe(1);
      expect(await page.locator('main table').count()).toBeGreaterThanOrEqual(4);
      expect(await page.locator('main pre').count()).toBeGreaterThanOrEqual(4);
      await expect(page.locator('[class*="openapi"], [class*="api-explorer"]').count()).resolves.toBe(0);
    } finally {
      await page.close();
    }
  }, 30_000);

  test('uses the New API Serif font for prose while preserving monospace code', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 1000 } });

    try {
      await page.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'domcontentloaded' });
      const bodyFont = await page.locator('body').evaluate((element) => getComputedStyle(element).fontFamily);
      const headingFont = await page.locator('main h1').evaluate((element) => getComputedStyle(element).fontFamily);
      const codeFont = await page.locator('main pre code').first().evaluate((element) => getComputedStyle(element).fontFamily);

      expect(bodyFont).toStartWith('"Lora Variable"');
      expect(headingFont).toStartWith('"Lora Variable"');
      expect(codeFont).not.toContain('Lora');
    } finally {
      await page.close();
    }
  }, 30_000);

  test('uses compact responsive root typography without page overflow', async () => {
    const desktop = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });
    const mobile = await activeBrowser().newPage({ viewport: { width: 390, height: 844 } });

    try {
      await Promise.all([
        desktop.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'domcontentloaded' }),
        mobile.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'domcontentloaded' }),
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
      await Promise.all([desktop.close(), mobile.close()]);
    }
  }, 30_000);

  test('uses the official Docusaurus shell while keeping search and a fixed light theme', async () => {
    const desktop = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });
    const errors: string[] = [];
    desktop.on('pageerror', (error) => errors.push(error.stack ?? error.message));
    desktop.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });

    try {
      await desktop.goto(`${baseUrl}/quick-start`, { waitUntil: 'domcontentloaded' });
      const brand = desktop.locator('.navbar__brand');
      await expect(brand.getAttribute('href')).resolves.toBe(configuredSiteUrl);
      await expect(brand.getAttribute('target')).resolves.toBe('_self');
      await expect(brand.locator('img').first().getAttribute('src')).resolves.toBe(docsRoute('/img/molii-wordmark.png'));
      await expect(brand.locator('.navbar__title').count()).resolves.toBe(0);
      const searchButton = usesAlgoliaSearch
        ? desktop.locator('.DocSearch-Button')
        : desktop.locator('.aa-DetachedSearchButton');
      await expect(searchButton.count()).resolves.toBe(1);
      await expect(desktop.getByRole('button', { name: /切换.*模式/ }).count()).resolves.toBe(0);
      await expect(desktop.locator('html').getAttribute('data-theme')).resolves.toBe('light');
      await expect(desktop.locator('.footer__title').allTextContents()).resolves.toEqual(['开发者资源']);
      expect(errors).toEqual([]);
    } finally {
      await desktop.close();
    }
  }, 30_000);

  test('uses one Provider category label backed by the generated Provider index', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

    try {
      await page.goto(`${baseUrl}/providers`, { waitUntil: 'domcontentloaded' });
      await expect(page.locator('main h1').filter({ hasText: 'Provider 与模型' }).count())
        .resolves.toBe(1);
      await expect(page.locator('.theme-doc-sidebar-menu').getByText('Provider 与模型', { exact: true }).count())
        .resolves.toBe(1);
    } finally {
      await page.close();
    }
  }, 30_000);

  test('keeps the documentation shell within a zoomed narrow viewport', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 320, height: 720 } });

    try {
      await page.goto(`${baseUrl}/quick-start`, { waitUntil: 'domcontentloaded' });
      await page.getByRole('button', { name: /切换导航栏/ }).click();
      expect(await page.locator('.navbar').getAttribute('class')).toContain('navbar-sidebar--show');
      expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
      await expect(page.locator('footer').count()).resolves.toBe(1);
    } finally {
      await page.close();
    }
  }, 30_000);

  test('exposes the six documentation destinations and removes the portal route', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

    try {
      await page.goto(`${baseUrl}/quick-start`, { waitUntil: 'domcontentloaded' });
      const primaryLinks = page.locator('.navbar__items:not(.navbar__items--right) > .navbar__item.navbar__link');
      await expect(primaryLinks.allTextContents()).resolves.toEqual([
        '开始使用',
        '平台与账户',
        '开发指南',
        '模型与能力',
        'API 参考',
        '帮助与更新',
      ]);
      await expect(primaryLinks.evaluateAll((links) => links.map((link) => link.getAttribute('href'))))
        .resolves.toEqual([
          '/quick-start',
          '/platform',
          '/api-basics',
          '/models',
          '/api-reference',
          '/help',
        ].map(docsRoute));

      for (const route of ['/examples', '/changelog']) {
        await expect(fetch(`${baseUrl}${route}`).then((response) => response.status)).resolves.toBe(200);
      }
      await expect(fetch(`${baseUrl}/`).then((response) => response.status)).resolves.toBe(404);
    } finally {
      await page.close();
    }
  }, 30_000);

  test('renders complete onboarding and platform maps instead of sparse introductions', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

    try {
      for (const [route, headings, minimumLinks] of [
        ['/quick-start', ['你将完成什么', '开始前准备', '获取 API Key', '发送第一个请求', '理解响应', '继续完成视频工作流', '接下来'], 6],
        ['/platform', ['平台能力地图', '从注册到生产使用', '看板与模型分析', 'API Key 管理', '管理临时素材', '核对生成记录与费用', '保护账户', '下一步'], 8],
      ] as const) {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded' });
        const renderedHeadings = (await page.locator('main h2').allTextContents())
          .map((heading) => heading.replaceAll('\u200B', ''));
        expect(renderedHeadings).toEqual(headings);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
        expect(await page.locator('main').innerText()).not.toContain('从这里开始接入 Molii。');
        if (route === '/quick-start') {
          expect(await page.locator('main').innerText()).not.toContain('页面不会再次完整展示同一个密钥。');
        }
      }
    } finally {
      await page.close();
    }
  }, 30_000);

  test('renders complete developer and model decision guides', async () => {
    const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

    try {
      for (const [route, headings, minimumLinks] of [
        ['/api-basics', ['公共请求约定', '环境与身份验证', '选择媒体输入', '处理异步任务', '错误、超时与重试', '预计费用与最终结算', '选择语言示例', '生产上线清单'], 10],
        ['/models', ['先按任务选择模型', '模型选择矩阵', 'Seedance 视频生成', 'Grok Imagine 图片', 'Grok Imagine 视频', '核对输入与输出能力', '查看权威参数与价格'], 8],
      ] as const) {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded' });
        const renderedHeadings = (await page.locator('main h2').allTextContents())
          .map((heading) => heading.replaceAll('\u200B', ''));
        expect(renderedHeadings).toEqual(headings);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
      }
    } finally {
      await page.close();
    }
  }, 30_000);

  for (const [label, route, requiredHeadings, minimumLinks] of [
    ['example', '/examples', ['按任务选择示例', 'Seedance 示例', 'Grok Imagine 示例', '安全运行示例', '从示例进入 API 参考'], 9],
    ['support', '/help', ['按症状选择排障路径', '登录、鉴权与限流', '异步任务状态', '媒体输入与下载', '联系支持前准备', '保护敏感信息', '查看更新'], 8],
  ] as const) {
    test(`renders the complete ${label} page`, async () => {
      const page = await activeBrowser().newPage({ viewport: { width: 1440, height: 900 } });

      try {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded' });
        const headings = (await page.locator('main h2').allTextContents())
          .map((heading) => heading.replaceAll('\u200B', ''));
        for (const required of requiredHeadings) expect(headings).toContain(required);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
      } finally {
        await page.close();
      }
    }, 30_000);
  }
});
