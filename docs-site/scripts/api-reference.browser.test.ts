import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { join } from 'node:path';
import { chromium } from 'playwright-core';

const siteRoot = join(import.meta.dir, '..');
const chromePath = process.env.DOCS_CHROME_PATH ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
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
});

describe('generated API reference', () => {
  test('hydrates the generated introduction without browser console errors', async () => {
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
});
