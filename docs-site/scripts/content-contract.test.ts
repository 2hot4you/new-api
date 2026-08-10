import { describe, expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const sourceCommit = '8928d7bb8b95';
const pages = [
  'docs/getting-started/quickstart.mdx',
  'docs/getting-started/video-workflow.mdx',
  'docs/api-basics/authentication.mdx',
  'docs/api-basics/base-url.mdx',
  'docs/api-basics/async-tasks.mdx',
  'docs/api-basics/media-inputs.mdx',
  'docs/api-basics/errors-retries.mdx',
  'docs/api-basics/billing-and-usage.mdx',
];

function findSecretLike(source: string): string[] {
  const withoutPlaceholders = source
    .replaceAll('Bearer $MOLII_API_KEY', '')
    .replaceAll('Bearer ${apiKey}', '')
    .replaceAll('Bearer {api_key}', '')
    .replaceAll('your-api-key', '');
  const patterns = [
    /\bsk-(?:proj-|ant-)?[A-Za-z0-9_-]{16,}\b/,
    /\bBearer\s+[A-Za-z0-9][A-Za-z0-9._~+/=-]{15,}\b/,
    /\b(?:github_pat_|ghp_|pat_)[A-Za-z0-9_-]{20,}\b/,
    /\bglpat-[A-Za-z0-9_-]{20,}\b/,
    /\bxox[baprs]-[A-Za-z0-9-]{10,}\b/,
    /\bAKIA[A-Z0-9]{16}\b/,
  ];
  return patterns.filter((pattern) => pattern.test(withoutPlaceholders)).map(String);
}

async function page(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

function frontmatter(source: string) {
  const match = source.match(/^---\n([\s\S]*?)\n---/);
  if (!match) return {};
  return Object.fromEntries(
    match[1].split('\n').map((line) => {
      const separator = line.indexOf(':');
      return [line.slice(0, separator).trim(), line.slice(separator + 1).trim()];
    }),
  );
}

describe('public API guide content contract', () => {
  test('all Task 3 pages carry public API provenance metadata', async () => {
    for (const relativePath of pages) {
      const metadata = frontmatter(await page(relativePath));
      expect(metadata.audience, relativePath).toBe('user');
      expect(metadata.apiVersion, relativePath).toBe('v1');
      expect(metadata.lastReviewed, relativePath).toBe('2026-08-10');
      expect(metadata.sourceCommit, relativePath).toBe(sourceCommit);
    }
  });

  test('quick starts provide complete curl, Python, and TypeScript requests without making them', async () => {
    const quickstart = await page('docs/getting-started/quickstart.mdx');
    const video = await page('docs/getting-started/video-workflow.mdx');

    for (const source of [quickstart, video]) {
      expect(source).toContain('```bash');
      expect(source).toContain('curl');
      expect(source).toContain('```python');
      expect(source).toContain('requests.');
      expect(source).toContain('```ts');
      expect(source).toContain('fetch(');
      expect(source).toContain('Authorization');
      expect(source).toContain('Bearer');
      expect(source).toContain('MOLII_API_KEY');
      expect(source).toContain('MOLII_API_BASE_URL');
    }
    expect(video).toContain("import { writeFile } from 'node:fs/promises';");
    expect(video).not.toContain('Bun.write');
    expect(video).toContain('retryAfter === null ? Number.NaN');
    expect(video).toContain("completed='false'");
  });

  test('base URL and authentication guidance is environment-safe', async () => {
    const authentication = await page('docs/api-basics/authentication.mdx');
    const baseUrl = await page('docs/api-basics/base-url.mdx');

    expect(authentication).toMatch(/Authorization:\s*Bearer\s+\$MOLII_API_KEY/);
    expect(authentication).toMatch(/服务端|server/i);
    expect(authentication).toMatch(/浏览器|browser/i);
    expect(baseUrl).toContain('MOLII_API_BASE_URL');
    expect(baseUrl).toContain('http://127.0.0.1:3000');
    expect(baseUrl).toMatch(/生产.*可配置|可配置.*生产/);
    expect(baseUrl).not.toMatch(/https:\/\/api\.molii\.(?:com|cn)/);
  });

  test('async and retry guidance never automatically retries paid POST requests', async () => {
    const asyncTasks = await page('docs/api-basics/async-tasks.mdx');
    const retries = await page('docs/api-basics/errors-retries.mdx');
    const combined = `${asyncTasks}\n${retries}`;

    expect(combined).toMatch(/付费\s*POST[^。\n]*(?:不要|不得|禁止)自动重试/);
    expect(combined).toContain('Retry-After');
    expect(combined).toMatch(/GET[^。\n]*(?:指数退避|exponential backoff)/i);
    expect(combined).toMatch(/抖动|jitter/i);
    expect(combined).toMatch(/429/);
    expect(combined).toMatch(/5\d\d|5xx/i);
    expect(asyncTasks).toMatch(/queued|in_progress/);
    expect(asyncTasks).toMatch(/completed/);
    expect(asyncTasks).toMatch(/failed/);
  });

  test('media and billing guides expose only stable user-facing concepts', async () => {
    const media = await page('docs/api-basics/media-inputs.mdx');
    const billing = await page('docs/api-basics/billing-and-usage.mdx');

    expect(media).toMatch(/HTTP\(S\)|https:\/\//i);
    expect(media).toMatch(/image[^\n]*images|images[^\n]*image/);
    expect(media).not.toContain('file_id');
    expect(billing).toMatch(/预计|预估/);
    expect(billing).toMatch(/最终|实际/);
    expect(billing).toContain('final_cost');
    expect(billing).toContain('refund_pending');
    expect(billing).toMatch(/不要.*重复提交|重复提交.*额外计费/);
  });

  test('billing modes match the public runtime contract', async () => {
    const billing = await page('docs/api-basics/billing-and-usage.mdx');
    expect(billing).toContain('grok_video');
    expect(billing).toContain('seedance');
    expect(billing).not.toContain('per_call');
  });

  test('media guide distinguishes model-specific URL, Data URL, and asset URI support', async () => {
    const media = await page('docs/api-basics/media-inputs.mdx');

    expect(media).toContain('Data URL');
    expect(media).toContain('data:');
    expect(media).toContain('asset://');
    expect(media).toMatch(/Grok[^。\n]*(?:仅|只)[^。\n]*URL/);
    expect(media).toMatch(/Seedance[^。\n]*(?:Data URL|asset:\/\/)/);
    expect(media).toMatch(/具体模型页|模型页面/);
  });

  test('all video download examples verify video content before writing the output file', async () => {
    const video = await page('docs/getting-started/video-workflow.mdx');

    expect(video).toContain("headers_file=");
    expect(video).toContain("case \"$content_type\" in");
    expect(video).toContain('video/*)');
    const pythonHeaderCheck = video.indexOf('content.headers.get("Content-Type"');
    const pythonWrite = video.indexOf('with open("result.mp4"');
    expect(pythonHeaderCheck).toBeGreaterThan(-1);
    expect(pythonHeaderCheck).toBeLessThan(pythonWrite);
    const typeScriptHeaderCheck = video.indexOf("content.headers.get('Content-Type')");
    const typeScriptWrite = video.indexOf("await writeFile('result.mp4'");
    expect(typeScriptHeaderCheck).toBeGreaterThan(-1);
    expect(typeScriptHeaderCheck).toBeLessThan(typeScriptWrite);
  });

  test('video polling applies request timeouts, retry headers, and wall-clock deadlines', async () => {
    const video = await page('docs/getting-started/video-workflow.mdx');

    expect(video).toContain("--write-out '%{http_code}'");
    expect(video).toContain('429|5??)');
    expect(video).toContain('"retry-after:"');
    expect(video).toMatch(/retry_after=.*awk[^\n]+\$headers_file/);
    expect(video.match(/--connect-timeout\s+\d+/g)?.length ?? 0).toBeGreaterThanOrEqual(2);
    expect(video.match(/--max-time\s+\d+/g)?.length ?? 0).toBeGreaterThanOrEqual(2);
    expect(video).toContain('poll_deadline=');
    expect(video).toContain('time.monotonic()');
    expect(video).toContain('pollDeadline = Date.now()');
    expect(video.match(/signal:\s*AbortSignal\.timeout/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
  });

  test('error guidance captures a sanitized public request id for support', async () => {
    const errors = await page('docs/api-basics/errors-retries.mdx');

    expect(errors).toContain('X-Oneapi-Request-Id');
    expect(errors).toMatch(/\u8131\u654f|sanitize/i);
    expect(errors).toMatch(/\u652f\u6301[^\u3002\n]*\u6392\u969c|\u6392\u969c[^\u3002\n]*\u652f\u6301/);
    expect(errors).toContain('```bash');
    expect(errors).toContain('response.headers.get("X-Oneapi-Request-Id"');
    expect(errors).toContain("response.headers.get('X-Oneapi-Request-Id')");
  });

  test('secret-like detector rejects credentials while allowing documented placeholders', async () => {
    const credentialFixtures = [
      `sk-${'a'.repeat(32)}`,
      `Authorization: Bearer live_${'b'.repeat(32)}`,
      `github_pat_${'c'.repeat(32)}`,
      `glpat-${'d'.repeat(24)}`,
      `AKIA${'E'.repeat(16)}`,
    ];
    for (const fixture of credentialFixtures) expect(findSecretLike(fixture)).not.toEqual([]);
    for (const placeholder of [
      'Authorization: Bearer $MOLII_API_KEY',
      'Authorization: Bearer ${apiKey}',
      'Authorization: Bearer {api_key}',
      "MOLII_API_KEY='your-api-key'",
    ]) expect(findSecretLike(placeholder)).toEqual([]);

    const combined = (await Promise.all(pages.map(page))).join('\n');
    expect(findSecretLike(combined)).toEqual([]);
  });

  test('relative documentation links use extensionless Docusaurus routes', async () => {
    const combined = (await Promise.all(pages.map(page))).join('\n');
    expect(combined).not.toMatch(/\]\((?:\.\.\/|\.\/)[^)]+\.mdx(?:#[^)]+)?\)/);
  });

  test('Task 3 pages exclude operator-only surfaces and infrastructure', async () => {
    const combined = (await Promise.all(pages.map(page))).join('\n');
    const forbidden = [
      '/api/channel',
      '/api/option',
      '/api/system',
      'X-Admin-Key',
      'upstreamCredential',
      'Redis',
      'MySQL',
      '管理员',
      '渠道',
      '部署',
      '数据库',
    ];
    for (const term of forbidden) expect(combined).not.toContain(term);
  });

  test('sidebar exposes the new learning path in order', () => {
    expect(sidebars.docsSidebar.slice(0, 2)).toEqual([
      {
        type: 'category',
        label: '开始使用',
        items: ['getting-started/quickstart', 'getting-started/video-workflow'],
      },
      {
        type: 'category',
        label: 'API 基础',
        items: [
          'api-basics/authentication',
          'api-basics/base-url',
          'api-basics/async-tasks',
          'api-basics/media-inputs',
          'api-basics/errors-retries',
          'api-basics/billing-and-usage',
        ],
      },
    ]);
  });

  test('API lifecycle component covers submit, poll, and settlement states accessibly', async () => {
    const source = await page('src/components/ApiLifecycle.tsx');
    expect(source).toContain("aria-label='异步 API 生命周期'");
    expect(source).toContain('提交');
    expect(source).toContain('轮询');
    expect(source).toContain('结算');
    expect(source).toContain('refund_pending');
  });
});
