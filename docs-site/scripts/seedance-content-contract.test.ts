import { describe, expect, test } from 'bun:test';
import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const sourceCommit = '8928d7bb8b95';
const pages = [
  'docs/models/overview.mdx',
  'docs/models/seedance-2.mdx',
  'docs/guides/seedance-multimodal.mdx',
  'docs/guides/temporary-assets.mdx',
  'docs/examples/seedance-curl.mdx',
  'docs/examples/seedance-python.mdx',
  'docs/examples/seedance-typescript.mdx',
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

function fencedCode(source: string, language: string): string {
  return source.match(new RegExp(`\\\`\\\`\\\`${language}\\n([\\s\\S]*?)\\n\\\`\\\`\\\``))?.[1] ?? '';
}

async function runCurlExample(downloadStatus: '200' | '302') {
  const workspace = await mkdtemp(join(tmpdir(), 'seedance-curl-contract-'));
  const binDirectory = join(workspace, 'bin');
  const scriptPath = join(workspace, 'example.sh');
  const curlLogPath = join(workspace, 'curl.log');
  const curlCountPath = join(workspace, 'curl.count');
  await mkdir(binDirectory);
  await writeFile(
    scriptPath,
    fencedCode(await page('docs/examples/seedance-curl.mdx'), 'bash'),
  );
  await writeFile(
    join(binDirectory, 'jq'),
    `#!/usr/bin/env bash
set -eu
query="$2"
case "$query" in
  .id) printf 'task_contract_test\\n' ;;
  .status) printf 'completed\\n' ;;
  .progress) printf '100\\n' ;;
  *) printf 'task failed\\n' ;;
esac
`,
  );
  await writeFile(
    join(binDirectory, 'curl'),
    `#!/usr/bin/env bash
set -eu
count=0
[ ! -f "$FAKE_CURL_COUNT" ] || count="$(cat "$FAKE_CURL_COUNT")"
count="$((count + 1))"
printf '%s' "$count" >"$FAKE_CURL_COUNT"

url=''
headers_file=''
body_file=''
has_authorization=0
follow_redirects=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --url) url="$2"; shift 2 ;;
    --header)
      case "$2" in Authorization:*) has_authorization=1 ;; esac
      shift 2
      ;;
    --dump-header) headers_file="$2"; shift 2 ;;
    --output) body_file="$2"; shift 2 ;;
    --location) follow_redirects=1; shift ;;
    --request|--connect-timeout|--max-time|--data|--write-out) shift 2 ;;
    --silent|--show-error) shift ;;
    *) shift ;;
  esac
done
printf 'request call=%s auth=%s follow=%s url=%s\\n' "$count" "$has_authorization" "$follow_redirects" "$url" >>"$FAKE_CURL_LOG"

case "$count" in
  1)
    printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n' >"$headers_file"
    printf '{"id":"task_contract_test"}' >"$body_file"
    printf '200'
    ;;
  2)
    printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n' >"$headers_file"
    printf '{"id":"task_contract_test","status":"completed","progress":100}' >"$body_file"
    printf '200'
    ;;
  3)
    if [ "$FAKE_DOWNLOAD_STATUS" = 302 ]; then
      printf 'HTTP/1.1 302 Found\\r\\nLocation: https://media.example.invalid/result.mp4\\r\\n\\r\\n' >"$headers_file"
      : >"$body_file"
      if [ "$follow_redirects" -eq 1 ]; then
        printf 'redirect-follow auth=%s url=https://media.example.invalid/result.mp4\\n' "$has_authorization" >>"$FAKE_CURL_LOG"
        printf 'HTTP/1.1 302 Found\\r\\nLocation: https://media.example.invalid/result.mp4\\r\\n\\r\\nHTTP/1.1 200 OK\\r\\nContent-Type: video/mp4\\r\\n\\r\\n' >"$headers_file"
        printf 'fake-video' >"$body_file"
        printf '200'
      else
        printf '302'
      fi
    else
      printf 'HTTP/1.1 200 OK\\r\\nContent-Type: video/mp4\\r\\n\\r\\n' >"$headers_file"
      printf 'fake-video' >"$body_file"
      printf '200'
    fi
    ;;
  *)
    printf 'unexpected curl call\\n' >&2
    exit 90
    ;;
esac
`,
  );
  await chmod(join(binDirectory, 'curl'), 0o755);
  await chmod(join(binDirectory, 'jq'), 0o755);

  const child = Bun.spawn(['bash', scriptPath], {
    cwd: workspace,
    env: {
      ...process.env,
      PATH: `${binDirectory}:${process.env.PATH ?? ''}`,
      MOLII_API_KEY: 'contract-test-key',
      MOLII_API_BASE_URL: 'https://api.example.invalid',
      FAKE_CURL_LOG: curlLogPath,
      FAKE_CURL_COUNT: curlCountPath,
      FAKE_DOWNLOAD_STATUS: downloadStatus,
    },
    stdout: 'pipe',
    stderr: 'pipe',
  });
  const [exitCode, stdout, stderr] = await Promise.all([
    child.exited,
    new Response(child.stdout).text(),
    new Response(child.stderr).text(),
  ]);
  const curlLog = await readFile(curlLogPath, 'utf8');
  const resultPath = join(workspace, 'seedance-result.mp4');
  let resultExists = true;
  try {
    await readFile(resultPath);
  } catch {
    resultExists = false;
  }

  return {
    cleanup: () => rm(workspace, { recursive: true, force: true }),
    curlLog,
    exitCode,
    resultExists,
    stderr,
    stdout,
  };
}

describe('Seedance and temporary asset documentation contract', () => {
  test('all Task 4 pages carry user-facing API provenance metadata', async () => {
    for (const relativePath of pages) {
      const metadata = frontmatter(await page(relativePath));
      expect(metadata.audience, relativePath).toBe('user');
      expect(metadata.apiVersion, relativePath).toBe('v1');
      expect(metadata.lastReviewed, relativePath).toBe('2026-08-10');
      expect(metadata.sourceCommit, relativePath).toBe(sourceCommit);
    }
  });

  test('model pages document every registered model and its resolution boundary', async () => {
    const overview = await page('docs/models/overview.mdx');
    const seedance = await page('docs/models/seedance-2.mdx');
    const combined = `${overview}\n${seedance}`;

    expect(combined).toContain('doubao-seedance-2-0-260128');
    expect(combined).toContain('doubao-seedance-2-0-fast-260128');
    expect(combined).toContain('doubao-seedance-2-0-mini-260615');
    expect(combined).toContain('doubao-seedance-2-5-260628');
    expect(seedance).toMatch(/标准版[^\n]*(?:480p|`480p`)[^\n]*(?:720p|`720p`)[^\n]*(?:1080p|`1080p`)[^\n]*(?:4k|`4k`)/i);
    expect(seedance).toMatch(/Fast[^\n]*(?:480p|`480p`)[^\n]*(?:720p|`720p`)/i);
    expect(seedance).toMatch(/Fast[^。\n]*(?:不支持|不可使用)[^。\n]*(?:1080p|`1080p`)[^。\n]*(?:4k|`4k`)/i);
    expect(seedance).toMatch(/Mini[^\n]*(?:480p|`480p`)[^\n]*(?:720p|`720p`)/i);
    expect(seedance).toMatch(/2\.5[^\n]*(?:1080p|`1080p`)/i);
    expect(seedance).toMatch(/2\.5[^\n]*(?:30|`30`)/i);
  });

  test('Seedance parameter guide records defaults, enums, and supported tools', async () => {
    const seedance = await page('docs/models/seedance-2.mdx');

    expect(seedance).toMatch(/duration[^\n]*(?:-1|`-1`)[^\n]*(?:4[^\n]*15|4–15)/i);
    for (const ratio of ['adaptive', '16:9', '4:3', '1:1', '3:4', '9:16', '21:9']) {
      expect(seedance).toContain(ratio);
    }
    expect(seedance).toMatch(/generate_audio[^\n]*true/i);
    expect(seedance).toMatch(/watermark[^\n]*false/i);
    expect(seedance).toContain('web_search');
    expect(seedance).toContain('<ParameterTable');
  });

  test('multimodal guide covers roles, counts, valid requests, and rejected combinations', async () => {
    const multimodal = await page('docs/guides/seedance-multimodal.mdx');

    for (const role of ['first_frame', 'last_frame', 'reference_image', 'reference_video', 'reference_audio']) {
      expect(multimodal).toContain(role);
    }
    expect(multimodal).toMatch(/9[^\n]*(?:图片|图像)|(?:图片|图像)[^\n]*9/);
    expect(multimodal).toMatch(/3[^\n]*视频|视频[^\n]*3/);
    expect(multimodal).toMatch(/3[^\n]*音频|音频[^\n]*3/);
    expect(multimodal).toMatch(/首帧[^。\n]*尾帧[^。\n]*(?:不能|不可|互斥)[^。\n]*(?:参考|多模态)|(?:参考|多模态)[^。\n]*(?:不能|不可|互斥)[^。\n]*首帧/);
    expect(multimodal).toMatch(/音频[^。\n]*(?:不能|不可)[^。\n]*(?:单独|独立)/);
    expect(multimodal).toContain('有效示例');
    expect(multimodal).toContain('无效示例');
  });

  test('temporary asset guide documents public lifecycle without promising a fixed TTL', async () => {
    const assets = await page('docs/guides/temporary-assets.mdx');

    expect(assets).toContain('POST /v1/assets');
    expect(assets).toContain('GET /v1/assets/{id}');
    expect(assets).toContain('DELETE /v1/assets/{id}');
    expect(assets).toContain('asset://asset-molii-');
    for (const type of ['image', 'video', 'audio']) expect(assets).toContain(type);
    for (const state of ['PROCESSING', 'ACTIVE', 'SUCCESS', 'FAILED', 'EXPIRED']) {
      expect(assets).toContain(state);
    }
    expect(assets).toContain('expires_at');
    expect(assets).toMatch(/expires_at[^。\n]*(?:为准|读取|响应)/);
    expect(assets).not.toMatch(/固定\s*24\s*(?:小时|h)/i);
    expect(assets).toMatch(/当前账户|同一账户/);
    expect(assets).toMatch(/公网[^。\n]*(?:HTTP|HTTPS)/i);
    expect(assets).toMatch(/localhost|私有网络|内网/);
    expect(assets).toMatch(/直接访问|可访问|可达/);
  });

  test('language examples submit the paid task once and poll with bounded safe GET requests', async () => {
    const examples = [
      ['docs/examples/seedance-curl.mdx', 'bash'],
      ['docs/examples/seedance-python.mdx', 'python'],
      ['docs/examples/seedance-typescript.mdx', 'ts'],
    ] as const;

    for (const [relativePath, language] of examples) {
      const source = await page(relativePath);
      const code = fencedCode(source, language);
      expect(code, relativePath).toContain('MOLII_API_KEY');
      expect(code, relativePath).toContain('MOLII_API_BASE_URL');
      expect(code, relativePath).toContain('doubao-seedance-2-0-260128');
      expect(code, relativePath).toContain('/v1/videos');
      expect(code, relativePath).toMatch(/queued/);
      expect(code, relativePath).toMatch(/in_progress/);
      expect(code, relativePath).toMatch(/completed/);
      expect(code, relativePath).toMatch(/failed/);
      expect(code, relativePath).toMatch(/unexpected task status/i);
      expect(code, relativePath).toMatch(/Retry-After|retry-after/i);
      expect(code, relativePath).toMatch(/429/);
      expect(code, relativePath).toMatch(/500|5\d\d|>=\s*500/);
      expect(code, relativePath).toMatch(/deadline/i);
      expect(code, relativePath).toContain('/content');
      expect(code, relativePath).toMatch(/Content-Type|content-type/i);
    }

    const shell = fencedCode(await page(examples[0][0]), 'bash');
    const python = fencedCode(await page(examples[1][0]), 'python');
    const typeScript = fencedCode(await page(examples[2][0]), 'ts');
    expect(shell.match(/--request POST/g)?.length ?? 0).toBe(1);
    expect(python.match(/\.post\(/g)?.length ?? 0).toBe(1);
    expect(typeScript.match(/method:\s*'POST'/g)?.length ?? 0).toBe(1);
    expect(python).toContain('follow_redirects=False');
    expect(typeScript).toContain("redirect: 'error'");
  });

  test('curl download never follows a cross-host redirect with the API authorization', async () => {
    const direct = await runCurlExample('200');
    try {
      expect(direct.exitCode).toBe(0);
      expect(direct.resultExists).toBe(true);
      expect(direct.curlLog).toContain('request call=3 auth=1 follow=0');
      expect(direct.curlLog).not.toContain('redirect-follow');
    } finally {
      await direct.cleanup();
    }

    const redirected = await runCurlExample('302');
    try {
      expect(redirected.exitCode).not.toBe(0);
      expect(redirected.resultExists).toBe(false);
      expect(redirected.curlLog).toContain('request call=3 auth=1 follow=0');
      expect(redirected.curlLog).not.toContain('redirect-follow');
      expect(redirected.stderr).toContain('download failed with HTTP 302');
    } finally {
      await redirected.cleanup();
    }
  });

  test('sidebar exposes models, guides, and per-language examples', () => {
    const serialized = JSON.stringify(sidebars);
    for (const id of [
      'models/overview',
      'models/seedance-2',
      'guides/seedance-multimodal',
      'guides/temporary-assets',
      'examples/seedance-curl',
      'examples/seedance-python',
      'examples/seedance-typescript',
    ]) {
      expect(serialized).toContain(id);
    }
  });

  test('Task 4 public pages do not expose internal platform concepts or secret-like values', async () => {
    const sources = await Promise.all(pages.map(page));
    const combined = sources.join('\n');

    expect(combined).not.toMatch(/StarAI|lfxqai|channel_id|upstream_id|\/api\/channel|\/api\/admin|Redis|PostgreSQL|MySQL|SQLite/iu);
    expect(combined).not.toMatch(/\bsk-[A-Za-z0-9_-]{16,}\b/);
  });
});
