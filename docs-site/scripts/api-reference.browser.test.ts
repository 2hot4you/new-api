import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { chromium } from 'playwright-core';

import { resolveBrowserExecutable } from './browser-executable.mjs';

const siteRoot = join(import.meta.dir, '..');
const generatedApiRoot = join(siteRoot, 'generated', 'api');
const staleEndpointPath = join(generatedApiRoot, 'review-stale.api.mdx');
const port = 3197;
let buildOutput = '';
let server: ReturnType<typeof Bun.spawn> | undefined;

async function run(command: string[]) {
  const process = Bun.spawn(command, { cwd: siteRoot, stdout: 'pipe', stderr: 'pipe' });
  const [stdout, stderr, exitCode] = await Promise.all([
    new Response(process.stdout).text(),
    new Response(process.stderr).text(),
    process.exited,
  ]);
  return { exitCode, output: `${stdout}\n${stderr}` };
}

beforeAll(async () => {
  await mkdir(generatedApiRoot, { recursive: true });
  await writeFile(staleEndpointPath, [
    '---',
    'id: review-stale',
    'title: Review stale endpoint',
    '---',
    '',
    '# Review stale endpoint',
  ].join('\n'));
  const generated = await run(['bun', 'run', 'api:generate']);
  if (generated.exitCode !== 0) throw new Error(generated.output);
  const built = await run(['bun', 'run', 'build']);
  buildOutput = built.output;
  if (built.exitCode !== 0) throw new Error(buildOutput);

  server = Bun.spawn(
    ['bun', 'x', 'docusaurus', 'serve', '--host', '127.0.0.1', '--port', String(port), '--no-open'],
    { cwd: siteRoot, stdout: 'pipe', stderr: 'pipe' },
  );
  const deadline = Date.now() + 15_000;
  let ready = false;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/api-reference/molii-public-api`);
      if (response.ok) {
        ready = true;
        break;
      }
    } catch {
      // Server is still starting.
    }
    await Bun.sleep(100);
  }
  if (!ready) throw new Error('Docusaurus preview server did not become ready');
}, 30_000);

afterAll(async () => {
  server?.kill();
  await rm(staleEndpointPath, { force: true });
});

describe('generated API reference', () => {
  test('hydrates the generated introduction without browser console errors', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const page = await browser.newPage();
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.stack ?? error.message));
    page.on('console', (message) => {
      if (message.type() === 'error') errors.push(message.text());
    });

    try {
      await page.goto(`http://127.0.0.1:${port}/api-reference/molii-public-api`, { waitUntil: 'networkidle' });
      await expect(page.locator('h1').filter({ hasText: 'Molii Public API' }).count()).resolves.toBe(1);
      expect(errors).toEqual([]);
    } finally {
      await browser.close();
    }
  }, 30_000);

  test('builds without OpenAPI theme module export warnings', () => {
    expect(buildOutput).not.toContain("export 'default' (imported as 'SchemaTabs') was not found");
  });

  test('removes stale MDX before generation', async () => {
    expect(await Bun.file(staleEndpointPath).exists()).toBe(false);
  });

  test('emits current asset and video response contracts', async () => {
    const assetRequest = JSON.parse(await readFile(join(generatedApiRoot, 'create-asset.RequestSchema.json'), 'utf8'));
    expect(assetRequest.body.content['application/json'].schema).toMatchObject({
      required: ['url', 'asset_type', 'name'],
      properties: { asset_type: { enum: ['image', 'video', 'audio'] }, name: { maxLength: 80 } },
    });
    const assetResponses = JSON.parse(await readFile(join(generatedApiRoot, 'create-asset.StatusCodes.json'), 'utf8'));
    expect(assetResponses.responses['200'].content['application/json'].schema).toMatchObject({
      required: ['id'],
      properties: { id: { type: 'string' } },
    });

    const videoSubmit = JSON.parse(await readFile(join(generatedApiRoot, 'create-video-generation.StatusCodes.json'), 'utf8'));
    expect(videoSubmit.responses['200'].content['application/json'].schema).toMatchObject({
      required: ['id', 'object', 'model', 'status', 'progress', 'created_at'],
      properties: { object: { const: 'video' } },
    });
    const videoFetch = JSON.parse(await readFile(join(generatedApiRoot, 'get-video-generation.StatusCodes.json'), 'utf8'));
    expect(videoFetch.responses['200'].content['application/json'].schema).toMatchObject({
      required: ['code', 'message', 'data'],
      properties: {
        data: {
          required: ['task_id', 'status', 'progress'],
          properties: { result_url: { format: 'uri' }, billing: { type: 'object' } },
        },
      },
    });
  });
});
