# Molii Docusaurus Developer Portal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the sparse Docusaurus home and section entry pages with a professional developer portal, six-item navigation, rich onboarding paths, and verified desktop/mobile behavior.

**Architecture:** Serve `/` from an isolated React landing page using Docusaurus `Layout` and a CSS Module, while keeping guides and API reference pages on the stock Docs renderer. Expand the existing Markdown entry pages into authoritative navigation guides, preserve every public child route, and protect the result with real Playwright browser behavior tests before rebuilding and redeploying the existing LaunchAgent site.

**Tech Stack:** Docusaurus 3.10.2, React 19.2.8, TypeScript 5.9.3, CSS Modules, MD/MDX, Bun 1.3.14, Playwright Core 1.62.1, macOS LaunchAgent.

## Global Constraints

- Keep existing public child URLs, including `/examples` and `/changelog`.
- Keep New API（QuantumNous）attribution and all protected project identity references.
- Keep the existing Serif prose font and monospace code font.
- Keep the root font size at `12px` above `996px` and `14px` at `996px` and below.
- Do not add a UI framework, external image, remote font, runtime API request, online API explorer, SDK generator, or interactive paid request.
- Use only models, endpoints, parameters, media behavior, and billing concepts already implemented and documented in the repository.
- Code examples must use placeholders such as `$MOLII_API_KEY` and must never execute from the browser.
- Homepage-specific visual styles must remain in `src/pages/index.module.css`; ordinary Docs pages must retain the stock Docusaurus renderer.
- The home page must exceed two `1440×900` viewports, and no tested page may introduce page-level horizontal overflow.
- The deployed site must remain available at `http://127.0.0.1:3100` through `io.molii.docs`.

---

### Task 1: Build the isolated developer-portal home page

**Files:**
- Create: `docs-site/src/pages/index.tsx`
- Create: `docs-site/src/pages/index.module.css`
- Delete: `docs-site/docs/index.mdx`
- Modify: `docs-site/scripts/api-reference.browser.test.ts`
- Modify: `docs-site/scripts/default-theme-contract.test.ts`

**Interfaces:**
- Consumes: Docusaurus `Layout`, `Link`, and `Heading`; global typography from `src/css/fonts.css`; existing routes `/quick-start`, `/api-reference`, `/models/seedance-2`, `/models/grok-imagine-video`, `/api-basics/async-tasks`, `/api-basics/billing-and-usage`, `/api-basics/media-inputs`, `/guides/temporary-assets`, `/api-basics/errors-retries`, and `/examples`.
- Produces: default export `Home(): React.JSX.Element` at `/`; isolated CSS classes imported as `styles`; six visible content sections with no runtime data dependency.

- [ ] **Step 1: Add a failing browser behavior test for the approved portal**

Append this test inside `describe('default MDX API reference', ...)` in `docs-site/scripts/api-reference.browser.test.ts`:

```ts
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
        desktop.goto(`${baseUrl}/`, { waitUntil: 'networkidle' }),
        mobile.goto(`${baseUrl}/`, { waitUntil: 'networkidle' }),
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
```

- [ ] **Step 2: Run the browser test and verify RED**

Run from `docs-site/`:

```bash
bun test scripts/api-reference.browser.test.ts
```

Expected: the new test fails because `/` still renders `Molii 开发者文档`, lacks the five portal sections, and is shorter than two viewports; the pre-existing API reference tests pass.

- [ ] **Step 3: Create the home page component with static, auditable content**

Create `docs-site/src/pages/index.tsx` with these imports and data contracts:

```tsx
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import type { ReactNode } from 'react';

import styles from './index.module.css';

type PortalCard = {
  eyebrow: string;
  title: string;
  description: string;
  href: string;
  linkLabel: string;
};

const paths: PortalCard[] = [
  {
    eyebrow: '01 · 第一次使用',
    title: '发送第一个请求',
    description: '创建 API Key，完成图片生成请求，并正确读取响应与费用。',
    href: '/quick-start',
    linkLabel: '开始教程',
  },
  {
    eyebrow: '02 · 视频工作流',
    title: '提交并跟踪异步任务',
    description: '掌握 Seedance 与 Grok 视频创建、轮询、下载和最终结算。',
    href: '/getting-started/video-workflow',
    linkLabel: '查看工作流',
  },
  {
    eyebrow: '03 · 已有系统',
    title: '直接对接 API',
    description: '查阅完整端点、参数范围、错误码、媒体输入与重试边界。',
    href: '/api-reference',
    linkLabel: '打开 API 参考',
  },
];

const taskSteps = [
  ['STEP 1', '提交一次', '付费 POST 不自动重试，保存任务 ID。'],
  ['STEP 2', '安全轮询', '按建议间隔查询，处理超时与 Retry-After。'],
  ['STEP 3', '获取结果', '任务成功后读取可播放的媒体地址。'],
  ['STEP 4', '核对费用', '区分预计价格、实际 Token 与最终结算。'],
] as const;

const resources = [
  ['多模态媒体输入规范', '/api-basics/media-inputs'],
  ['临时素材生命周期', '/guides/temporary-assets'],
  ['错误处理与 Request ID', '/api-basics/errors-retries'],
  ['完整 curl、Python 与 TypeScript 示例', '/examples'],
] as const;
```

The default export must use `Layout` and render this exact semantic order:

```tsx
export default function Home(): ReactNode {
  return (
    <Layout
      title="Molii 开发者文档"
      description="使用 Molii API 构建可靠的 AI 图片与视频创作体验"
    >
      <main className={styles.portal}>
        <header className={styles.hero}>
          <div className={styles.heroCopy}>
            <p className={styles.eyebrow}>Molii Developer Platform</p>
            <Heading as="h1">把 AI 图片与视频能力，可靠地接入你的产品</Heading>
            <p className={styles.lead}>
              从第一个请求到生产上线，完整了解模型选择、媒体输入、异步任务、费用结算和错误恢复。
            </p>
            <div className={styles.actions}>
              <Link className="button button--primary" to="/quick-start">5 分钟快速开始</Link>
              <Link className="button button--secondary" to="/api-reference">浏览 API 参考</Link>
            </div>
            <ul className={styles.proofList}>
              <li><strong>统一鉴权</strong><span>Bearer API Key</span></li>
              <li><strong>多模态</strong><span>图片 · 视频 · 音频</span></li>
              <li><strong>任务可追踪</strong><span>状态 · 用量 · 计费</span></li>
            </ul>
          </div>
          <div className={styles.codeCard} aria-label="Seedance 请求示例">
            <div className={styles.codeHeader}>创建 Seedance 视频任务</div>
            <pre><code>{`curl -X POST https://api.molii.co/v1/video/generations \\
  -H "Authorization: Bearer $MOLII_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "清晨的竹林，电影感运镜",
    "resolution": "1080p",
    "ratio": "16:9",
    "duration": 5
  }'`}</code></pre>
          </div>
        </header>

        <section className={styles.section}>
          <Heading as="h2">选择你的接入路径</Heading>
          <p className={styles.sectionLead}>根据目标选择最短阅读路径，不必先理解全部文档结构。</p>
          <div className={styles.cardGrid}>
            {paths.map((path) => (
              <article className={styles.pathCard} key={path.href}>
                <p className={styles.cardEyebrow}>{path.eyebrow}</p>
                <Heading as="h3">{path.title}</Heading>
                <p>{path.description}</p>
                <Link to={path.href}>{path.linkLabel} →</Link>
              </article>
            ))}
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">模型与能力一目了然</Heading>
          <div className={styles.modelGrid}>
            <article className={styles.featuredModel}>
              <span>推荐 · 视频生成</span>
              <Heading as="h3">Seedance 2.0</Heading>
              <p>支持文生视频、首尾帧、多参考图片、参考视频和参考音频组合。</p>
              <Link to="/models/seedance-2">查看模型能力 →</Link>
            </article>
            <article className={styles.modelCard}>
              <Heading as="h3">Seedance 2.0 Fast</Heading>
              <p>面向 480p 与 720p 的快速视频生成工作流。</p>
              <Link to="/models/seedance-2">查看 Fast 版本 →</Link>
            </article>
            <article className={styles.modelCard}>
              <Heading as="h3">Grok Imagine</Heading>
              <p>覆盖图片生成、图片编辑和异步视频任务。</p>
              <Link to="/models/grok-imagine-video">查看 Grok 模型 →</Link>
            </article>
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">从请求到结算的完整链路</Heading>
          <div className={styles.stepGrid}>
            {taskSteps.map(([number, title, description]) => (
              <article key={number}>
                <small>{number}</small>
                <Heading as="h3">{title}</Heading>
                <p>{description}</p>
              </article>
            ))}
          </div>
        </section>

        <section className={styles.section}>
          <Heading as="h2">生产环境接入清单</Heading>
          <p className={styles.sectionLead}>上线前检查密钥管理、超时、重试、媒体输入、安全下载和费用结算。</p>
          <Link className="button button--secondary" to="/api-basics">查看完整清单</Link>
        </section>

        <section className={styles.section}>
          <Heading as="h2">继续深入</Heading>
          <div className={styles.resourceGrid}>
            {resources.map(([label, href]) => <Link key={href} to={href}>{label}<span>→</span></Link>)}
          </div>
        </section>
      </main>
    </Layout>
  );
}
```

- [ ] **Step 4: Add isolated responsive CSS and remove the conflicting Docs root**

Create `docs-site/src/pages/index.module.css` with these tokens and responsive contracts:

```css
.portal {
  --portal-accent: #b76541;
  --portal-accent-soft: #f5e8df;
  --portal-ink: #22211f;
  --portal-muted: #65625c;
  --portal-line: #ddd8cf;
  --portal-panel: #ffffff;
  --portal-paper: #fbfaf7;
  --portal-navy: #283043;
  overflow: hidden;
  background: var(--portal-paper);
  color: var(--portal-ink);
}

.hero,
.section {
  padding: 5rem max(1.5rem, calc((100vw - 1180px) / 2));
}

.hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(32rem, 0.82fr);
  gap: 4rem;
  align-items: center;
  min-height: 58rem;
  border-bottom: 1px solid var(--portal-line);
  background:
    radial-gradient(circle at 92% 8%, rgb(183 101 65 / 16%), transparent 33%),
    linear-gradient(150deg, #fbfaf7 55%, #f4eee7);
}

.eyebrow,
.cardEyebrow,
.stepGrid small {
  color: var(--portal-accent);
  font-family: var(--ifm-font-family-base);
  font-size: 1rem;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.hero h1 {
  max-width: 70rem;
  margin: 1rem 0 1.5rem;
  font-size: clamp(3.8rem, 5.1vw, 7rem);
  line-height: 1.08;
  letter-spacing: -0.035em;
}

.lead,
.sectionLead {
  color: var(--portal-muted);
  font-size: 1.5rem;
}

.actions,
.proofList {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}

.proofList {
  margin: 2.5rem 0 0;
  padding: 0;
  list-style: none;
}

.proofList li { display: grid; gap: 0.2rem; }
.proofList span { color: var(--portal-muted); }

.codeCard {
  overflow: hidden;
  border: 1px solid #303746;
  border-radius: 1rem;
  background: #171b24;
  box-shadow: 0 2rem 5rem rgb(34 33 31 / 18%);
}

.codeHeader {
  padding: 1rem 1.4rem;
  border-bottom: 1px solid #303746;
  color: #b8c0cf;
  font-family: var(--ifm-font-family-monospace);
}

.codeCard pre {
  margin: 0;
  padding: 1.8rem;
  background: transparent;
  color: #e9edf5;
  font-size: 1.1rem;
}

.section { max-width: 132rem; margin: 0 auto; }
.section + .section { border-top: 1px solid var(--portal-line); }
.section h2 { font-size: clamp(2.6rem, 3vw, 3.8rem); }

.cardGrid,
.modelGrid,
.stepGrid,
.resourceGrid {
  display: grid;
  gap: 1.4rem;
  margin-top: 2.4rem;
}

.cardGrid { grid-template-columns: repeat(3, 1fr); }
.modelGrid { grid-template-columns: 1.2fr 0.8fr 0.8fr; }
.stepGrid { grid-template-columns: repeat(4, 1fr); }
.resourceGrid { grid-template-columns: repeat(2, 1fr); }

.pathCard,
.modelCard,
.stepGrid article,
.resourceGrid a {
  padding: 2.2rem;
  border: 1px solid var(--portal-line);
  border-radius: 0.9rem;
  background: var(--portal-panel);
}

.featuredModel {
  padding: 2.6rem;
  border-radius: 1rem;
  background: var(--portal-navy);
  color: #fff;
}

.featuredModel a { color: #f0b79d; }
.resourceGrid a { display: flex; justify-content: space-between; }

@media (max-width: 996px) {
  .hero,
  .section { padding: 3.5rem 1.5rem; }
  .hero { grid-template-columns: 1fr; min-height: auto; }
  .hero h1 { font-size: 3.4rem; }
  .cardGrid,
  .modelGrid,
  .resourceGrid { grid-template-columns: 1fr; }
  .stepGrid { grid-template-columns: repeat(2, 1fr); }
  .codeCard { box-shadow: none; }
}

@media (max-width: 480px) {
  .stepGrid { grid-template-columns: 1fr; }
}
```

Delete `docs-site/docs/index.mdx` so `src/pages/index.tsx` is the only `/` route.

- [ ] **Step 5: Update the legacy default-theme contract without weakening Docs-page protection**

In `docs-site/scripts/default-theme-contract.test.ts`:

- Rename the first test to `keeps global CSS limited to typography while the portal owns isolated styles`.
- Keep the config, Lora import, forbidden global layout-property regex, and missing `src/css/custom.css` assertions.
- Remove only the two assertions that `src/pages/index.tsx` and `src/pages/index.module.css` do not exist.
- Delete the obsolete test `serves the root route as a plain default Docs page with every primary destination`.
- Keep the API reference renderer, sidebar, and stock component-style tests unchanged.

- [ ] **Step 6: Run focused tests and verify GREEN**

Run from `docs-site/`:

```bash
bun test scripts/default-theme-contract.test.ts scripts/api-reference.browser.test.ts
```

Expected: all focused tests pass; the home page has the five required `h2` sections, exceeds 1,800px at the desktop viewport, has no page overflow, and API reference pages still use stock Docs layout.

- [ ] **Step 7: Commit the portal home page**

```bash
git add docs-site/src/pages/index.tsx docs-site/src/pages/index.module.css \
  docs-site/docs/index.mdx docs-site/scripts/api-reference.browser.test.ts \
  docs-site/scripts/default-theme-contract.test.ts
git commit -m "feat: build Docusaurus developer portal"
```

---

### Task 2: Consolidate the top navigation and sidebar language

**Files:**
- Modify: `docs-site/docusaurus.config.ts`
- Modify: `docs-site/sidebars.ts`
- Modify: `docs-site/scripts/api-reference.browser.test.ts`
- Modify: `docs-site/scripts/default-theme-contract.test.ts`

**Interfaces:**
- Consumes: existing public routes `/quick-start`, `/platform`, `/api-basics`, `/models`, `/api-reference`, `/help`, `/examples`, and `/changelog`.
- Produces: exactly six desktop navbar links labeled `开始使用`, `平台与账户`, `开发指南`, `模型与能力`, `API 参考`, `帮助与更新`; sidebar group labels that use the same vocabulary.

- [ ] **Step 1: Add a failing rendered-navigation test**

Append to `docs-site/scripts/api-reference.browser.test.ts`:

```ts
  test('exposes six focused top-level destinations and preserves nested routes', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    try {
      await page.goto(`${baseUrl}/`, { waitUntil: 'networkidle' });
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
```

- [ ] **Step 2: Run the browser test and verify RED**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: the test receives the old eight labels and fails the exact six-label assertion.

- [ ] **Step 3: Replace the navbar items and align the sidebar labels**

Replace `themeConfig.navbar.items` in `docs-site/docusaurus.config.ts` with:

```ts
items: [
  { label: '开始使用', to: '/quick-start' },
  { label: '平台与账户', to: '/platform' },
  { label: '开发指南', to: '/api-basics' },
  { label: '模型与能力', to: '/models' },
  { label: 'API 参考', to: '/api-reference' },
  { label: '帮助与更新', to: '/help' },
],
```

In `docs-site/sidebars.ts`, reorder the categories to match this progression and use these labels:

```ts
[
  { type: 'category', label: '开始使用', items: ['quick-start', 'getting-started/quickstart', 'getting-started/video-workflow'] },
  { type: 'category', label: '平台与账户', items: ['platform', 'platform/register-and-sign-in', 'platform/dashboard', 'platform/api-keys', 'platform/model-square-and-playground', 'platform/temporary-assets', 'platform/usage-and-generation-records', 'platform/wallet-and-billing', 'platform/profile-and-security'] },
  { type: 'category', label: '开发指南', items: ['api-basics', 'api-basics/authentication', 'api-basics/base-url', 'api-basics/async-tasks', 'api-basics/media-inputs', 'api-basics/errors-retries', 'api-basics/billing-and-usage'] },
  { type: 'category', label: '模型与能力', items: ['models', 'models/overview', 'models/seedance-2', 'models/grok-imagine-image', 'models/grok-imagine-video'] },
  { type: 'category', label: '进阶指南', items: ['guides/seedance-multimodal', 'guides/temporary-assets'] },
  { type: 'category', label: '示例与工具', items: ['examples', 'examples/seedance-curl', 'examples/seedance-python', 'examples/seedance-typescript', 'examples/grok-image-curl', 'examples/grok-video-curl', 'examples/grok-poll-download'] },
  { type: 'category', label: 'API 参考', items: ['api-reference/index', 'api-reference/models', 'api-reference/images', 'api-reference/videos', 'api-reference/seedance', 'api-reference/assets', 'api-reference/errors'] },
  { type: 'category', label: '帮助与更新', items: ['help', 'help/troubleshooting', 'help/contact-support', 'changelog'] },
]
```

Remove the old trailing standalone entries for `quick-start`, `platform`, `api-basics`, `help`, and `changelog`, because the same documents now lead their corresponding categories.

- [ ] **Step 4: Update the legacy config contract to the six labels**

In `docs-site/scripts/default-theme-contract.test.ts`, replace the old eight-label loop with:

```ts
for (const label of ['开始使用', '平台与账户', '开发指南', '模型与能力', 'API 参考', '帮助与更新']) {
  expect(config).toContain(`label: '${label}'`);
}
```

Keep the New API（QuantumNous）attribution and stock Docs renderer assertions unchanged.

- [ ] **Step 5: Run focused tests and verify GREEN**

```bash
cd docs-site
bun test scripts/default-theme-contract.test.ts scripts/api-reference.browser.test.ts
```

Expected: exact rendered navigation labels and hrefs pass; `/examples` and `/changelog` remain HTTP 200; API reference sidebar checks still pass.

- [ ] **Step 6: Commit navigation consolidation**

```bash
git add docs-site/docusaurus.config.ts docs-site/sidebars.ts \
  docs-site/scripts/api-reference.browser.test.ts docs-site/scripts/default-theme-contract.test.ts
git commit -m "docs: consolidate Docusaurus navigation"
```

---

### Task 3: Expand onboarding and platform entry pages

**Files:**
- Modify: `docs-site/docs/quick-start.md`
- Modify: `docs-site/docs/platform.md`
- Modify: `docs-site/scripts/api-reference.browser.test.ts`

**Interfaces:**
- Consumes: existing detailed pages under `getting-started/` and `platform/`.
- Produces: rendered `/quick-start` and `/platform` guides with at least six meaningful `h2` sections, stable links to every relevant detailed page, and clear next actions.

- [ ] **Step 1: Add failing rendered-content tests**

Append to `docs-site/scripts/api-reference.browser.test.ts`:

```ts
  test('renders complete onboarding and platform maps instead of sparse introductions', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    try {
      for (const [route, headings, minimumLinks] of [
        ['/quick-start', ['你将完成什么', '开始前准备', '获取 API Key', '发送第一个请求', '理解响应', '继续完成视频工作流', '接下来'], 6],
        ['/platform', ['平台能力地图', '从注册到生产使用', '看板与模型分析', '管理 API Key', '管理临时素材', '核对生成记录与费用', '保护账户'], 8],
      ] as const) {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle' });
        const renderedHeadings = await page.locator('main h2').allTextContents();
        expect(renderedHeadings).toEqual(headings);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
        expect(await page.locator('main').innerText()).not.toContain('从这里开始接入 Molii。');
      }
    } finally {
      await browser.close();
    }
  }, 30_000);
```

- [ ] **Step 2: Run the browser test and verify RED**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: `/quick-start` and `/platform` each render zero `h2` headings and fail their expected heading arrays.

- [ ] **Step 3: Replace `quick-start.md` with the approved onboarding path**

Write these exact sections and links in `docs-site/docs/quick-start.md`:

```md
---
sidebar_position: 1
---

# 快速开始

这条路径帮助你安全地创建凭证、完成第一个图片请求，并理解如何继续接入异步视频任务。所有示例都使用占位 Key，不会自动执行请求。

## 你将完成什么

- 创建并安全保存 Molii API Key。
- 选择一个已开放模型并发送图片生成请求。
- 读取结果、Request ID 和用量信息。
- 理解视频任务为什么需要提交与轮询两个阶段。

## 开始前准备

你需要一个可登录的 Molii 账户、可用额度，以及能够发送 HTTPS 请求的 curl、Python 或 TypeScript 环境。先阅读[注册、登录与找回密码](/platform/register-and-sign-in)，再确认运行环境不会把密钥写入日志或前端代码。

## 获取 API Key

在平台的 API Key 页面创建只授予所需模型权限的 Key。创建后立即复制并保存在服务端环境变量中；页面不会再次完整展示同一个密钥。完整管理方式见[API 密钥](/platform/api-keys)和[身份验证](/api-basics/authentication)。

```bash
export MOLII_API_KEY='replace-with-your-key'
```

## 发送第一个请求

先使用图片生成接口验证鉴权、模型权限和响应处理。付费 POST 只提交一次，不要配置自动重试。

```bash
curl --fail-with-body --request POST 'http://127.0.0.1:3000/v1/images/generations' \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "grok-imagine-image",
    "prompt": "清晨薄雾中的竹林，自然光，电影感构图",
    "n": 1
  }'
```

生产环境请把基础地址替换为你的 Molii API 地址，并保持协议、主机和密钥来自可信配置。

## 理解响应

成功响应包含生成结果和请求相关信息；失败时记录公开 Request ID，再按状态码决定是否重试。不要在工单、日志或截图中附带完整 Authorization 请求头。接口字段见[图片 API](/api-reference/images)，错误处理见[错误与重试](/api-basics/errors-retries)。

## 继续完成视频工作流

视频生成通常先返回任务 ID，再通过查询接口获取进度、最终媒体和结算信息。按[视频生成工作流](/getting-started/video-workflow)完成一次提交、有限轮询和安全下载；使用参考媒体前先阅读[媒体输入](/api-basics/media-inputs)。

## 接下来

- 按任务选择模型：[模型与能力](/models)
- 了解异步状态：[异步任务](/api-basics/async-tasks)
- 查看完整语言示例：[示例与工具](/examples)
- 进入端点定义：[API 参考](/api-reference)
```

- [ ] **Step 4: Replace `platform.md` with the approved capability map**

Write these exact sections and links in `docs-site/docs/platform.md`:

```md
---
sidebar_position: 2
---

# 平台与账户

Molii 平台把账户、API 凭证、模型调用、临时素材、生成记录和账单放在同一套工作区中。先从当前目标进入对应页面，再回到这里检查完整使用链路。

## 平台能力地图

| 目标 | 入口 | 你可以完成的操作 |
| --- | --- | --- |
| 访问账户 | [注册、登录与找回密码](/platform/register-and-sign-in) | 注册、登录、验证和重置密码 |
| 查看运行情况 | [看板](/platform/dashboard) | 查看额度、调用趋势、模型分析和渠道分流 |
| 管理凭证 | [API 密钥](/platform/api-keys) | 创建、查看、停用和批量管理 Key |
| 准备媒体 | [临时素材](/platform/temporary-assets) | 上传或提交 URL、查询状态、预览和删除素材 |
| 核对结果 | [使用与生成记录](/platform/usage-and-generation-records) | 查找任务、Token、费用和生成状态 |
| 管理资金 | [钱包与账单](/platform/wallet-and-billing) | 充值、兑换和查看订单历史 |

## 从注册到生产使用

推荐顺序是：完成账户验证，创建权限受控的 API Key，发送测试请求，确认生成记录与费用，再为生产环境设置独立 Key。不要让多个环境共享同一个长期凭证。

## 看板与模型分析

[看板](/platform/dashboard)用于查看账户概览、模型调用趋势和分流情况。图表没有数据时先检查时间范围、Key 状态和是否确实产生过请求；看板统计不能替代单次任务日志中的最终结算信息。

## 管理 API Key

为开发、测试和生产分别创建 Key，并按模型、额度和有效期限制使用范围。Key 泄露时立即停用，而不是只从代码仓库删除。具体操作见[API 密钥](/platform/api-keys)。

## 管理临时素材

[临时素材](/platform/temporary-assets)支持本地上传和 URL 来源。素材提交后需要等待上游状态变为可用，过期或删除后不能继续在生成请求中引用。API 工作流见[临时素材指南](/guides/temporary-assets)。

## 核对生成记录与费用

[使用与生成记录](/platform/usage-and-generation-records)展示任务状态、关键参数、预计 Token、实际 Token 和最终费用。异步任务应以成功或失败后的最终结算为准，不要把提交阶段的预估当作最终扣费。

## 保护账户

在[个人资料与安全](/platform/profile-and-security)维护登录方式和安全设置。不要向支持人员发送密码、完整 API Key、Authorization 请求头或包含敏感媒体的公开链接。

## 下一步

- 了解模型：[模型广场与 Playground](/platform/model-square-and-playground)
- 开始调用：[快速开始](/quick-start)
- 查看请求规则：[开发指南](/api-basics)
- 排查问题：[帮助与更新](/help)
```

- [ ] **Step 5: Run the browser test and verify GREEN**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: both pages render exact heading arrays, meet minimum internal-link counts, and no longer contain the sparse introduction.

- [ ] **Step 6: Commit onboarding and platform content**

```bash
git add docs-site/docs/quick-start.md docs-site/docs/platform.md docs-site/scripts/api-reference.browser.test.ts
git commit -m "docs: expand onboarding and platform guides"
```

---

### Task 4: Expand developer-guide and model entry pages

**Files:**
- Modify: `docs-site/docs/api-basics.md`
- Modify: `docs-site/docs/models.md`
- Modify: `docs-site/scripts/api-reference.browser.test.ts`

**Interfaces:**
- Consumes: existing pages under `api-basics/`, `models/`, `guides/`, and `examples/`.
- Produces: rendered `/api-basics` and `/models` pages with exact section maps and links that distinguish public contracts from model-specific behavior.

- [ ] **Step 1: Add failing rendered-content tests**

Append to `docs-site/scripts/api-reference.browser.test.ts`:

```ts
  test('renders complete developer and model decision guides', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    try {
      for (const [route, headings, minimumLinks] of [
        ['/api-basics', ['公共请求约定', '环境与身份验证', '选择媒体输入', '处理异步任务', '错误、超时与重试', '预计费用与最终结算', '选择语言示例', '生产上线清单'], 10],
        ['/models', ['先按任务选择模型', '模型选择矩阵', 'Seedance 视频生成', 'Grok Imagine 图片', 'Grok Imagine 视频', '核对输入与输出能力', '查看权威参数与价格'], 8],
      ] as const) {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle' });
        expect(await page.locator('main h2').allTextContents()).toEqual(headings);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
      }
    } finally {
      await browser.close();
    }
  }, 30_000);
```

- [ ] **Step 2: Run the browser test and verify RED**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: both sparse pages fail exact rendered heading arrays.

- [ ] **Step 3: Expand `api-basics.md` with exact public-contract sections**

Replace the page with a Markdown guide containing these headings and content:

```md
# 开发指南

这里汇总所有模型共享的公共调用规则。先理解环境、鉴权、媒体输入和异步任务，再进入具体模型或端点页面。

## 公共请求约定

所有请求使用 HTTPS、JSON 或端点明确要求的媒体格式。为每次请求设置连接和总超时，保存公开 Request ID，并把付费 POST 与安全 GET 查询区分处理。完整基础地址见[Base URL 与环境](/api-basics/base-url)。

## 环境与身份验证

API Key 只放在服务端 `Authorization: Bearer ...` 请求头中。开发、测试和生产使用不同 Key；日志、前端包、截图和工单不得包含完整密钥。详见[身份验证](/api-basics/authentication)。

## 选择媒体输入

根据模型选择公网 URL、Data URL 或 `asset://` 临时素材。提交前检查协议、Content-Type、文件可达性和有效期；Seedance 多参考输入还需要遵守角色和数量约束。阅读[媒体输入](/api-basics/media-inputs)、[Seedance 多模态输入](/guides/seedance-multimodal)和[临时素材](/guides/temporary-assets)。

## 处理异步任务

视频任务通常先返回任务 ID。付费创建请求只发送一次，查询请求按建议间隔有限轮询，成功后再读取媒体和最终计费。状态定义和查询响应见[异步任务](/api-basics/async-tasks)。

## 错误、超时与重试

只对明确安全且可恢复的查询操作重试。创建请求超时时先使用 Request ID 或任务记录确认是否已受理，不能直接重复提交。状态码矩阵见[错误与重试](/api-basics/errors-retries)，标准结构见[错误 API](/api-reference/errors)。

## 预计费用与最终结算

预估用于提交前展示，最终费用以任务完成后的实际 Token 和结算结果为准。不要自行猜测上游缺失的用量。详见[计费与用量](/api-basics/billing-and-usage)和[使用与生成记录](/platform/usage-and-generation-records)。

## 选择语言示例

- [Seedance curl](/examples/seedance-curl)
- [Seedance Python](/examples/seedance-python)
- [Seedance TypeScript](/examples/seedance-typescript)
- [Grok 图片 curl](/examples/grok-image-curl)
- [Grok 视频 curl](/examples/grok-video-curl)
- [Grok 查询与下载](/examples/grok-poll-download)

## 生产上线清单

- 使用独立生产 Key 和可信基础地址。
- 为连接、响应和异步任务分别设置超时。
- 不自动重试付费 POST。
- 验证媒体 Content-Type、大小、可达性和有效期。
- 记录公开 Request ID、任务 ID、模型、状态和最终费用。
- 对日志、错误原因和用户输入进行脱敏。
- 在上线前完成[快速开始](/quick-start)和目标模型的[API 参考](/api-reference)。
```

Keep the existing frontmatter `sidebar_position: 3` above the title.

- [ ] **Step 4: Expand `models.md` with exact selection guidance**

Replace the page body with:

```md
# 模型与能力

先按任务类型、输入媒体、输出规格和延迟要求选择模型，再进入模型页核对完整参数。模型能力和可用价格以当前模型页与平台模型广场为准。

## 先按任务选择模型

- 需要多参考图片、参考视频或参考音频的视频创作：选择 Seedance 2.0。
- 需要 480p 或 720p 快速视频生成：选择 Seedance 2.0 Fast。
- 需要图片生成或图片编辑：选择 Grok Imagine 图片模型。
- 需要文生视频、图生视频或受支持的视频编辑：选择 Grok Imagine 视频模型。

## 模型选择矩阵

| 任务 | 推荐模型页 | 需要重点核对 |
| --- | --- | --- |
| Seedance 标准视频 | [Seedance 2.0](/models/seedance-2) | 参考媒体、分辨率、比例、时长、音频 |
| Seedance 快速视频 | [Seedance 2.0](/models/seedance-2) | Fast 分辨率边界和输入是否包含视频 |
| 图片生成与编辑 | [Grok Imagine 图片](/models/grok-imagine-image) | 图片数量、质量、输入 URL 和编辑限制 |
| Grok 视频任务 | [Grok Imagine 视频](/models/grok-imagine-video) | 生成、图生视频、编辑和异步状态 |

## Seedance 视频生成

Seedance 2.0 支持文本、首尾帧、多参考图片、参考视频和参考音频组合。Fast 版本面向较低分辨率和更快生成路径。完整内容结构与有效组合见[Seedance 多模态输入](/guides/seedance-multimodal)。

## Grok Imagine 图片

Grok Imagine 图片模型覆盖标准生成、高质量生成、单图编辑和多图编辑。输入媒体、数量和质量字段按[模型说明](/models/grok-imagine-image)与[图片 API](/api-reference/images)执行。

## Grok Imagine 视频

Grok 视频接口覆盖文生视频、受支持的图生视频和视频编辑路径，并通过异步任务返回结果。模型边界见[Grok Imagine 视频](/models/grok-imagine-video)和[视频 API](/api-reference/videos)。

## 核对输入与输出能力

选择模型后依次核对：输入类型和数量、输出分辨率、宽高比、时长、音频开关、媒体有效期和模型特有限制。不要把一个模型的参数默认值套用到另一个模型。

## 查看权威参数与价格

接口参数以对应[API 参考](/api-reference)为准，当前模型价格以模型说明和平台模型广场为准。入口页不复制可能变化的价格，避免多个页面产生不一致。
```

Keep the existing frontmatter `sidebar_position: 4` above the title.

- [ ] **Step 5: Run the browser test and verify GREEN**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: both pages render exact heading arrays and meet minimum link counts; existing API reference behavior remains green.

- [ ] **Step 6: Commit developer and model guides**

```bash
git add docs-site/docs/api-basics.md docs-site/docs/models.md docs-site/scripts/api-reference.browser.test.ts
git commit -m "docs: expand developer and model guides"
```

---

### Task 5: Expand examples, help, and API reference orientation

**Files:**
- Modify: `docs-site/docs/examples.md`
- Modify: `docs-site/docs/help.md`
- Modify: `docs-site/docs/api-reference/index.mdx`
- Modify: `docs-site/scripts/api-reference.browser.test.ts`

**Interfaces:**
- Consumes: all existing example pages, troubleshooting/support pages, changelog, and API reference groups.
- Produces: rendered `/examples`, `/help`, and `/api-reference` entry pages that guide users by task, symptom, and endpoint lifecycle without duplicating parameter tables.

- [ ] **Step 1: Add failing rendered-content tests**

Append to `docs-site/scripts/api-reference.browser.test.ts`:

```ts
  test('renders complete example, support, and API orientation pages', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    try {
      for (const [route, requiredHeadings, minimumLinks] of [
        ['/examples', ['按任务选择示例', 'Seedance 示例', 'Grok Imagine 示例', '安全运行示例', '从示例进入 API 参考'], 9],
        ['/help', ['按症状选择排障路径', '登录、鉴权与限流', '异步任务状态', '媒体输入与下载', '联系支持前准备', '保护敏感信息', '查看更新'], 8],
        ['/api-reference', ['开始前确认', '选择端点', '同步与异步响应', '计费与用量字段', '错误与 Request ID'], 10],
      ] as const) {
        await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle' });
        const headings = await page.locator('main h2').allTextContents();
        for (const required of requiredHeadings) expect(headings).toContain(required);
        expect(await page.locator('main a[href^="/"]').count()).toBeGreaterThanOrEqual(minimumLinks);
      }
    } finally {
      await browser.close();
    }
  }, 30_000);
```

- [ ] **Step 2: Run the browser test and verify RED**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: `/examples` and `/help` lack all required `h2` sections; `/api-reference` lacks the five new orientation headings.

- [ ] **Step 3: Expand the example catalog**

Replace `docs-site/docs/examples.md` with an entry page that uses the exact headings above and includes:

```md
# 示例与工具

示例展示安全、可复制的调用模式。先按任务选择语言和模型，再进入 API 参考核对当前字段；不要把示例中的占位地址或 Key 直接用于生产。

## 按任务选择示例

| 任务 | 示例 | 对应参考 |
| --- | --- | --- |
| Seedance 完整视频流程 | [curl](/examples/seedance-curl) · [Python](/examples/seedance-python) · [TypeScript](/examples/seedance-typescript) | [Seedance API](/api-reference/seedance) |
| Grok 图片生成与编辑 | [Grok 图片 curl](/examples/grok-image-curl) | [图片 API](/api-reference/images) |
| Grok 视频创建与编辑 | [Grok 视频 curl](/examples/grok-video-curl) | [视频 API](/api-reference/videos) |
| Grok 任务查询与下载 | [查询与下载](/examples/grok-poll-download) | [异步任务](/api-basics/async-tasks) |

## Seedance 示例

Seedance 示例覆盖一次付费提交、有限轮询、状态处理和安全下载。Python 与 TypeScript 版本适合直接拆分到服务端任务模块，curl 版本适合验证接口和参数。

## Grok Imagine 示例

图片示例覆盖标准生成、高质量生成和编辑；视频示例区分文生视频、图生视频和编辑路径。查询与下载示例负责处理最终媒体，不应携带 API Authorization 跨域跟随重定向。

## 安全运行示例

- 使用 `$MOLII_API_KEY` 等环境变量占位符。
- 付费 POST 恰好执行一次，不自动重试。
- GET 查询设置请求超时、轮询间隔和总截止时间。
- 下载前验证状态码和视频 Content-Type。
- 错误日志只记录脱敏 Request ID、任务 ID 和公开原因。

## 从示例进入 API 参考

示例用于展示调用顺序，不替代参数定义。提交前核对[模型列表](/api-reference/models)、[图片 API](/api-reference/images)、[视频 API](/api-reference/videos)、[Seedance API](/api-reference/seedance)、[临时素材 API](/api-reference/assets)和[错误 API](/api-reference/errors)。
```

Keep the existing `sidebar_position: 6` frontmatter.

- [ ] **Step 4: Expand the support hub**

Replace `docs-site/docs/help.md` with:

```md
# 帮助与更新

先按症状进入排障路径，并保存公开 Request ID、任务 ID、发生时间和已脱敏的请求摘要。仍未解决时，再携带这些信息联系支持。

## 按症状选择排障路径

| 症状 | 首先检查 | 详细指南 |
| --- | --- | --- |
| 无法登录或找回密码 | 账户状态、验证方式、邮件 | [注册、登录与找回密码](/platform/register-and-sign-in) |
| API 返回 401 / 403 / 429 | Key、权限、额度和频率 | [排障指南](/help/troubleshooting) |
| 任务长时间处理中 | 任务 ID、状态、轮询间隔 | [异步任务](/api-basics/async-tasks) |
| 媒体输入失败 | URL、Content-Type、有效期 | [媒体输入](/api-basics/media-inputs) |
| 下载地址无法播放 | 状态码、重定向和响应类型 | [排障指南](/help/troubleshooting) |

## 登录、鉴权与限流

先区分平台登录问题和 API 鉴权问题。API 请求确认 Bearer 格式、Key 状态、模型权限和账户额度；不要通过重复创建请求来测试限流恢复。

## 异步任务状态

记录任务 ID，并使用查询接口确认真实状态。生成视频本身可能耗时较长；只有超过模型正常范围、失败或达到客户端总截止时间时才按异常处理。

## 媒体输入与下载

媒体 URL 必须可由服务端访问并返回正确 Content-Type。临时素材还受上游状态和有效期约束。下载最终媒体时不要把 Molii Authorization 请求头发送到其他域名。

## 联系支持前准备

准备发生时间、公开 Request ID、任务 ID、模型、端点、HTTP 状态码、脱敏错误原因和最小复现步骤。完整清单见[联系支持](/help/contact-support)。

## 保护敏感信息

不要发送密码、完整 API Key、Authorization 请求头、Cookie、支付凭证或不应公开的媒体。必要日志必须先脱敏。

## 查看更新

通过[更新日志](/changelog)确认模型、端点和文档变化。升级前重新核对目标模型页和 API 参考，不依赖旧示例中的默认值。
```

Keep the existing `sidebar_position: 7` frontmatter.

- [ ] **Step 5: Add orientation sections to the existing API overview**

Keep the existing API reference content, but replace its current opening sections with this ordered orientation before the endpoint groups:

```mdx
## 开始前确认

准备服务端 API Key、可信 Base URL、请求超时和日志中的公开 Request ID 字段。付费创建请求不要配置自动重试。

## 选择端点

- [模型 API](/api-reference/models)：读取当前公开模型。
- [图片 API](/api-reference/images)：图片生成与编辑。
- [视频 API](/api-reference/videos)：Grok 视频创建、编辑、查询和下载。
- [Seedance API](/api-reference/seedance)：Seedance 多模态视频创建与查询。
- [临时素材 API](/api-reference/assets)：创建、查询和删除素材。
- [错误 API](/api-reference/errors)：标准错误、状态码和 Request ID。

## 同步与异步响应

图片请求通常直接返回结果；视频请求通常先返回任务 ID，再通过查询接口获得状态、媒体和最终计费。轮询规则见[异步任务](/api-basics/async-tasks)。

## 计费与用量字段

提交时的预估用于展示和额度检查，任务完成后的实际 Token 与最终结算才是权威结果。详见[计费与用量](/api-basics/billing-and-usage)。

## 错误与 Request ID

失败时保存公开 Request ID，并按状态码和操作类型决定是否重试。不要在日志或工单中记录完整 Authorization。详见[错误与重试](/api-basics/errors-retries)。
```

Retain the existing endpoint grouping table, response compatibility notes, and stock Markdown/MDX structure after these sections.

- [ ] **Step 6: Run the browser test and verify GREEN**

```bash
cd docs-site
bun test scripts/api-reference.browser.test.ts
```

Expected: all three entry pages contain every required heading and minimum internal links; existing method-style API tests remain green.

- [ ] **Step 7: Commit examples, support, and API orientation**

```bash
git add docs-site/docs/examples.md docs-site/docs/help.md \
  docs-site/docs/api-reference/index.mdx docs-site/scripts/api-reference.browser.test.ts
git commit -m "docs: expand examples support and API overview"
```

---

### Task 6: Verify, visually review, build, and deploy the portal

**Files:**
- Verify: `docs-site/`
- Deploy: `docs-site/build/` to `/Users/naf/Library/Application Support/molii-docs/site/`

**Interfaces:**
- Consumes: Tasks 1–5 complete source tree and `io.molii.docs` LaunchAgent.
- Produces: verified production build served at `http://127.0.0.1:3100` with the developer portal, six-item navigation, rich entry pages, and no broken internal links.

- [ ] **Step 1: Run the complete automated test suite**

```bash
cd docs-site
bun test
```

Expected: all tests pass with zero failures, including portal, navigation, content, typography, API reference, content-safety, and configuration tests.

- [ ] **Step 2: Run content safety checks and production build**

```bash
cd docs-site
bun run check:forbidden
bun run check:secrets
bun run build
```

Expected: forbidden-term scan and secret scan pass; Docusaurus generates `build/` and its search index without broken links, anchors, duplicate routes, or compilation errors.

- [ ] **Step 3: Run the internal link crawler without colliding with the live LaunchAgent**

Temporarily unload and always restore the local documentation service:

```bash
cd docs-site
service_target="gui/$(id -u)/io.molii.docs"
plist_path="/Users/naf/Library/LaunchAgents/io.molii.docs.plist"
restore_docs_service() {
  launchctl bootstrap "gui/$(id -u)" "$plist_path" 2>/dev/null || true
  launchctl kickstart -k "$service_target" 2>/dev/null || true
}
trap restore_docs_service EXIT HUP INT TERM
launchctl bootout "$service_target"
bun run check:links
trap - EXIT HUP INT TERM
restore_docs_service
```

Expected: the crawler reports every internal page and asset as HTTP 200 and exits zero; `io.molii.docs` is restored even if the crawler fails.

- [ ] **Step 4: Perform desktop visual verification in the in-app browser**

Serve `docs-site/build/` on an unused preview port, then inspect `/`, `/quick-start`, `/platform`, `/api-basics`, `/models`, `/api-reference`, and `/help` at `1440×900`.

For `/`, verify:

```js
({
  rootFontSize: getComputedStyle(document.documentElement).fontSize,
  overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  portalHeight: document.querySelector('main')?.scrollHeight,
  viewportHeight: innerHeight,
  navLabels: [...document.querySelectorAll('.navbar__item.navbar__link')].map((element) => element.textContent?.trim()),
})
```

Expected: `rootFontSize` is `12px`; `overflows` is `false`; `portalHeight >= viewportHeight * 2`; navigation labels are the approved six-item array; hero, code panel, cards, task steps, checklist, and resource links are visually distinct.

For each Docs entry page, verify stock sidebar and table-of-contents behavior, readable headings, working internal links, and no clipped tables or code blocks.

- [ ] **Step 5: Perform mobile visual verification**

Inspect the same routes at `390×844`.

Expected: `rootFontSize` is `14px`; Docusaurus mobile navigation toggle is visible; portal grids become one column except the two-column task grid where space permits; code scrolls inside its own panel; there is no page-level horizontal overflow; all headings and actions remain readable and operable.

- [ ] **Step 6: Deploy the verified build**

From the repository root:

```bash
rsync -a docs-site/build/ '/Users/naf/Library/Application Support/molii-docs/site/'
launchctl kickstart -k gui/$(id -u)/io.molii.docs
```

- [ ] **Step 7: Verify the live service and rendered contract**

```bash
curl --retry 20 --retry-delay 1 --retry-connrefused \
  --fail --silent --show-error --output /tmp/molii-docs-portal.html \
  --write-out '%{http_code}\n' http://127.0.0.1:3100/
curl --fail --silent http://127.0.0.1:3100/quick-start \
  | rg '你将完成什么|开始前准备|发送第一个请求'
launchctl print gui/$(id -u)/io.molii.docs \
  | rg '^\s*(state|pid|last exit code) ='
```

Expected: root returns HTTP 200; the deployed quick-start HTML contains the rich onboarding headings; LaunchAgent state is `running` with a PID and no nonzero exit code.

- [ ] **Step 8: Confirm the implementation branch is clean and ready for integration**

```bash
git status --short --branch
git log -6 --oneline
```

Expected: no tracked source changes remain; commits for portal, navigation, onboarding/platform, developer/models, and examples/help/API overview are present on the feature branch.
