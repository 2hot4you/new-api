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

function fencedCode(source: string, language: string): string {
  return source.match(new RegExp(`\\\`\\\`\\\`${language}\\n([\\s\\S]*?)\\n\\\`\\\`\\\``))?.[1] ?? '';
}

function fencedCodes(source: string, language: string): string[] {
  return [...source.matchAll(new RegExp(`\\\`\\\`\\\`${language}\\n([\\s\\S]*?)\\n\\\`\\\`\\\``, 'g'))]
    .map((match) => match[1]);
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
      expect(source).toContain('```ts');
      expect(source).toContain('fetch(');
      expect(source).toContain('Authorization');
      expect(source).toContain('Bearer');
      expect(source).toContain('MOLII_API_KEY');
      expect(source).toContain('MOLII_API_BASE_URL');
    }
    expect(quickstart).toContain('requests.');
    expect(video).toContain('httpx.AsyncClient');
    expect(video).toContain("import { writeFile } from 'node:fs/promises';");
    expect(video).not.toContain('Bun.write');
    expect(video).toContain('retryAfterMilliseconds');
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

  test('video polling applies request timeouts, retry headers, and strict wall-clock deadlines', async () => {
    const video = await page('docs/getting-started/video-workflow.mdx');
    const shell = fencedCode(video, 'bash');
    const python = fencedCode(video, 'python');
    const typeScript = fencedCode(video, 'ts');

    expect(video).toContain("--write-out '%{http_code}'");
    expect(video).toContain('429|5??)');
    expect(video).toContain('"retry-after:"');
    expect(video).toMatch(/retry_after=.*awk[^\n]+\$headers_file/);
    expect(shell).toContain('--connect-timeout "$connect_timeout"');
    expect(shell).toContain('--connect-timeout "$download_connect_timeout"');
    expect(video).toContain('poll_deadline=');
    expect(video).toContain('asyncio.get_running_loop().time()');
    expect(video).toContain('async with asyncio.timeout(remaining):');
    expect(video).toContain('await client.get(url, follow_redirects=follow_redirects)');
    expect(video).toContain('Python 3.11+');
    expect(video).toContain('pip install httpx');
    expect(video).not.toContain('requests` 的 `(connect, read)`');
    expect(video).toContain('pollDeadline = Date.now()');
    expect(video.match(/signal:\s*AbortSignal\.timeout/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(shell.indexOf('poll_remaining=')).toBeLessThan(shell.indexOf('\n  http_code="$(curl'));
    expect(shell).toContain('--max-time "$poll_timeout"');
    expect(shell.indexOf('download_remaining=')).toBeLessThan(shell.indexOf('download_http_code="$(curl'));
    expect(shell).toContain('--max-time "$download_timeout"');
    expect(python.match(/await client\.post\(/g)?.length ?? 0).toBe(1);
    expect(python).not.toMatch(/AsyncClient\([^)]*follow_redirects=True/);
    expect(python).toContain('follow_redirects=True');
    expect(python).toContain('response = await get_before_deadline(');
    expect(python).toContain('content = await get_before_deadline(');
    expect(python).toContain('poll_deadline = asyncio.get_running_loop().time() + 180');
    expect(python.indexOf('remaining = deadline - asyncio.get_running_loop().time()'))
      .toBeLessThan(python.indexOf('async with asyncio.timeout(remaining):'));
    expect(python.indexOf('async with asyncio.timeout(remaining):'))
      .toBeLessThan(python.indexOf('return await client.get(url, follow_redirects=follow_redirects)'));
    expect(typeScript.indexOf('const pollRemaining = pollDeadline - Date.now()'))
      .toBeLessThan(typeScript.indexOf('const response = await fetch('));
    expect(typeScript).toContain('const pollTimeout = Math.min(30_000, pollRemaining)');
    expect(typeScript).toContain('AbortSignal.timeout(pollTimeout)');
    expect(typeScript.indexOf('const downloadRemaining = pollDeadline - Date.now()'))
      .toBeLessThan(typeScript.indexOf('const content = await fetch('));
    expect(typeScript).toContain('const downloadTimeout = Math.min(120_000, downloadRemaining)');
    expect(typeScript).toContain('AbortSignal.timeout(downloadTimeout)');
  });

  test('Python and TypeScript sanitize request ids before every response error', async () => {
    const video = await page('docs/getting-started/video-workflow.mdx');
    const python = fencedCode(video, 'python');
    const typeScript = fencedCode(video, 'ts');

    for (const [before, after] of [
      ['created_request_id = sanitize_request_id(created)', 'raise_with_request_id(created, created_request_id)'],
      ['response_request_id = sanitize_request_id(response)', 'raise_with_request_id(response, response_request_id)'],
      ['content_request_id = sanitize_request_id(content)', 'raise_with_request_id(content, content_request_id)'],
    ]) {
      expect(python.indexOf(before), before).toBeGreaterThan(-1);
      expect(python.indexOf(before), before).toBeLessThan(python.indexOf(after));
    }
    for (const [before, after] of [
      ['const createdRequestId = sanitizeRequestId(createdResponse)', 'throwResponseError(createdResponse, createdRequestId)'],
      ['const responseRequestId = sanitizeRequestId(response)', 'throwResponseError(response, responseRequestId)'],
      ['const contentRequestId = sanitizeRequestId(content)', 'throwResponseError(content, contentRequestId)'],
    ]) {
      expect(typeScript.indexOf(before), before).toBeGreaterThan(-1);
      expect(typeScript.indexOf(before), before).toBeLessThan(typeScript.indexOf(after));
    }
  });

  test('Retry-After parsing supports delta seconds and HTTP dates with safe fallback', async () => {
    const video = await page('docs/getting-started/video-workflow.mdx');
    const python = fencedCode(video, 'python');
    const typeScript = fencedCode(video, 'ts');

    expect(python).toContain('def retry_after_seconds(value, now=None):');
    expect(python).toContain('value.isdigit()');
    expect(python).toContain('parsedate_to_datetime(value)');
    expect(python).toContain('return delta if delta >= 0 else None');
    expect(python).toContain('if parsed_retry_after is not None');
    expect(typeScript).toContain('function retryAfterMilliseconds(value: string | null, now = Date.now())');
    expect(typeScript).toContain('/^\\d+$/.test(trimmed)');
    expect(typeScript).toContain('Date.parse(trimmed)');
    expect(typeScript).toContain('return delta >= 0 ? delta : null');
    expect(typeScript).toMatch(/parsedRetryAfter\s*\n\s*\?\?/);
    expect(video).toMatch(/\u7a7a\u503c[^\u3002\n]*\u9000\u907f|\u975e\u6cd5[^\u3002\n]*\u9000\u907f/);
  });

  test('error guidance captures a sanitized public request id for support', async () => {
    const errors = await page('docs/api-basics/errors-retries.mdx');
    const shell = fencedCode(errors, 'bash');
    const python = fencedCode(errors, 'python');
    const typeScript = fencedCodes(errors, 'ts').join('\n');

    expect(errors).toContain('X-Oneapi-Request-Id');
    expect(errors).toMatch(/\u8131\u654f|sanitize/i);
    expect(errors).toMatch(/\u652f\u6301[^\u3002\n]*\u6392\u969c|\u6392\u969c[^\u3002\n]*\u652f\u6301/);
    expect(errors).toContain('```bash');
    expect(errors).toContain('response.headers.get("X-Oneapi-Request-Id"');
    expect(errors).toContain("response.headers.get('X-Oneapi-Request-Id')");
    expect(shell).toContain('set -e');
    expect(shell).toContain('body_file="$(mktemp)"');
    expect(shell).toContain('if http_code="$(curl');
    expect(shell).toContain('--output "$body_file"');
    expect(shell.indexOf('safe_request_id=')).toBeLessThan(shell.indexOf('exit 1'));
    expect(python.indexOf('safe_request_id =')).toBeLessThan(python.indexOf('response.raise_for_status()'));
    expect(typeScript.indexOf('const safeRequestId =')).toBeLessThan(typeScript.indexOf('throw new Error'));
  });

  test('error retryDelay accepts only safe delta seconds or future HTTP dates', async () => {
    const errors = await page('docs/api-basics/errors-retries.mdx');
    const typeScript = fencedCode(errors, 'ts');
    const functionSource = typeScript.match(/function retryDelay\([\s\S]*?\n\}/)?.[0] ?? '';
    const transpiler = new Bun.Transpiler({ loader: 'ts' });
    const javascript = transpiler.transformSync(functionSource);
    const retryDelay = new Function(`${javascript}; return retryDelay;`)() as (
      attempt: number,
      retryAfter: string | null,
      now?: number,
    ) => number;
    const originalRandom = Math.random;
    Math.random = () => 0;
    try {
      const now = Date.parse('2026-08-10T00:00:00Z');
      expect(retryDelay(0, '0', now)).toBe(0);
      expect(retryDelay(0, '12', now)).toBe(12_000);
      expect(retryDelay(0, 'Mon, 10 Aug 2026 00:00:05 GMT', now)).toBe(5_000);
      for (const invalid of [null, '', '   ', '-1', '1.5', 'not-a-date', 'Sun, 09 Aug 2026 23:59:59 GMT']) {
        expect(retryDelay(2, invalid, now), String(invalid)).toBe(4_000);
      }
      expect(retryDelay(2, String(Number.MAX_SAFE_INTEGER), now)).toBe(4_000);
    } finally {
      Math.random = originalRandom;
    }
  });

  test('billing unavailable state never guesses a charge', async () => {
    const billing = await page('docs/api-basics/billing-and-usage.mdx');
    expect(billing).toContain('unavailable');
    expect(billing).toContain('\u8ba1\u8d39\u660e\u7ec6\u6682\u4e0d\u53ef\u7528');
    expect(billing).toMatch(/\u7a0d\u540e\u91cd\u8bd5|\u8054\u7cfb\u652f\u6301/);
    expect(billing).toMatch(/\u4e0d\u8981\u731c\u6d4b[^\u3002\n]*\u8d39\u7528|\u4e0d\u731c\u6d4b[^\u3002\n]*\u8d39\u7528/);
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
