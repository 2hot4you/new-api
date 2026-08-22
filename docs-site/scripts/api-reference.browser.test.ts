import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { chromium } from 'playwright-core';

import { resolveBrowserExecutable } from './browser-executable.mjs';

const siteRoot = new URL('..', import.meta.url).pathname;
const port = 3197;
const baseUrl = `http://127.0.0.1:${port}`;
let server: ReturnType<typeof Bun.spawn> | undefined;

beforeAll(async () => {
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
  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Docusaurus static server exited with ${server.exitCode}`);
    }
    try {
      const response = await fetch(`${baseUrl}/api-reference`);
      if (response.ok) return;
    } catch {
      // Static server is still starting.
    }
    await Bun.sleep(100);
  }
  throw new Error('Docusaurus static server did not become ready');
}, 60_000);

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
      await browser.close();
    }
  }, 30_000);

  test('opens onboarding and account categories by default while keeping them collapsible', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

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
      await browser.close();
    }
  }, 30_000);

  test('renders method-style image endpoints as ordinary headings, tables, and code blocks', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

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
      await browser.close();
    }
  }, 30_000);

  test('uses the New API Serif font for prose while preserving monospace code', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });

    try {
      await page.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'domcontentloaded' });
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
      await browser.close();
    }
  }, 30_000);

  test('renders the developer portal with complete desktop and mobile reading paths', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const desktop = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });
    const errors: string[] = [];
    for (const page of [desktop, mobile]) {
      page.on('pageerror', (error) => errors.push(error.stack ?? error.message));
      page.on('console', (message) => {
        if (message.type() === 'error') errors.push(message.text());
      });
    }

    try {
      await Promise.all([
        desktop.goto(`${baseUrl}/`, { waitUntil: 'domcontentloaded' }),
        mobile.goto(`${baseUrl}/`, { waitUntil: 'domcontentloaded' }),
      ]);

      await expect(desktop.getByRole('heading', {
        level: 1,
        name: '把 AI 图片与视频能力，可靠地接入你的产品',
      }).count()).resolves.toBe(1);
      for (const heading of [
        '选择你的接入路径',
        '模型与能力一目了然',
        '从请求到结算的完整链路',
        '生产环境接入清单',
        '继续深入',
      ]) {
        await expect(desktop.getByRole('heading', { level: 2, name: heading }).count())
          .resolves.toBe(1);
      }
      await expect(desktop.getByRole('link', { name: '5 分钟快速开始' }).getAttribute('href'))
        .resolves.toBe('/quick-start');
      await expect(desktop.getByRole('link', { name: '浏览 API 参考' }).getAttribute('href'))
        .resolves.toBe('/api-reference');

      const desktopLayout = await desktop.evaluate(() => ({
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        scrollHeight: document.documentElement.scrollHeight,
        viewportHeight: innerHeight,
      }));
      const mobileLayout = await mobile.evaluate(() => ({
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        h1Visible: Boolean(document.querySelector('main h1')),
      }));

      expect(desktopLayout.overflows).toBe(false);
      expect(desktopLayout.scrollHeight).toBeGreaterThanOrEqual(desktopLayout.viewportHeight * 2);
      expect(mobileLayout).toEqual({ overflows: false, h1Visible: true });
      expect(errors).toEqual([]);
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('keeps zoomed narrow homepage content and keyboard focus inside visible bounds', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    // A 320 CSS-pixel viewport is the layout width of a 640-pixel window at 200% zoom.
    const page = await browser.newPage({ viewport: { width: 320, height: 720 } });

    try {
      await page.goto(`${baseUrl}/`, { waitUntil: 'domcontentloaded' });
      const primaryAction = page.getByRole('link', { name: '5 分钟快速开始' });
      await primaryAction.focus();

      const layout = await page.evaluate(() => {
        const portal = document.querySelector('main');
        const pre = document.querySelector('main pre');
        const focused = document.activeElement;
        if (!portal || !pre || !focused) throw new Error('Homepage layout fixture is incomplete');

        const gridChildren = [
          ...document.querySelectorAll('main > header > div'),
          ...document.querySelectorAll('main > section > div > article'),
          ...document.querySelectorAll('main > section > div > a'),
        ];
        const elementsWithinViewport = gridChildren.every((element) => {
          const bounds = element.getBoundingClientRect();
          return bounds.left >= 0 && bounds.right <= innerWidth;
        });

        pre.scrollLeft = pre.scrollWidth;
        const focusedBounds = focused.getBoundingClientRect();
        const focusedStyle = getComputedStyle(focused);
        const outlineWidth = Number.parseFloat(focusedStyle.outlineWidth);

        return {
          portalOverflowX: getComputedStyle(portal).overflowX,
          gridChildrenMinWidth: gridChildren.map((element) => getComputedStyle(element).minWidth),
          elementsWithinViewport,
          codeOverflowX: getComputedStyle(pre).overflowX,
          codeScrollsIndependently: pre.scrollWidth > pre.clientWidth && pre.scrollLeft > 0,
          focusVisible: focusedStyle.outlineStyle !== 'none' && outlineWidth > 0,
          focusWithinViewport:
            focusedBounds.left - outlineWidth >= 0
            && focusedBounds.right + outlineWidth <= innerWidth,
        };
      });

      expect(layout.portalOverflowX).toBe('visible');
      expect(layout.gridChildrenMinWidth.every((minWidth) => minWidth === '0px')).toBe(true);
      expect(layout.elementsWithinViewport).toBe(true);
      expect(layout.codeOverflowX).toBe('auto');
      expect(layout.codeScrollsIndependently).toBe(true);
      expect(layout.focusVisible).toBe(true);
      expect(layout.focusWithinViewport).toBe(true);
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('exposes six focused top-level destinations and preserves nested routes', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    try {
      await page.goto(`${baseUrl}/`, { waitUntil: 'domcontentloaded' });
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
        .resolves.toEqual(['/quick-start', '/platform', '/api-basics', '/models', '/api-reference', '/help']);

      for (const route of ['/examples', '/changelog']) {
        await expect(fetch(`${baseUrl}${route}`).then((response) => response.status)).resolves.toBe(200);
      }
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('renders complete onboarding and platform maps instead of sparse introductions', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

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
      await browser.close();
    }
  }, 30_000);

  test('renders complete developer and model decision guides', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

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
      await browser.close();
    }
  }, 30_000);

  for (const [label, route, requiredHeadings, minimumLinks] of [
    ['example', '/examples', ['按任务选择示例', 'Seedance 示例', 'Grok Imagine 示例', '安全运行示例', '从示例进入 API 参考'], 9],
    ['support', '/help', ['按症状选择排障路径', '登录、鉴权与限流', '异步任务状态', '媒体输入与下载', '联系支持前准备', '保护敏感信息', '查看更新'], 8],
  ] as const) {
    test(`renders the complete ${label} page`, async () => {
      const chromePath = await resolveBrowserExecutable();
      const browser = await chromium.launch({ executablePath: chromePath, headless: true });
      const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

      try {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'domcontentloaded' });
        const headings = (await page.locator('main h2').allTextContents())
          .map((heading) => heading.replaceAll('\u200B', ''));
        for (const required of requiredHeadings) expect(headings).toContain(required);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
      } finally {
        await browser.close();
      }
    }, 30_000);
  }
});
