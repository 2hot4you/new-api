import { expect, test } from 'bun:test';
import { resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dirname, '../..');
const docsWorkflowPath = resolve(repositoryRoot, '.github/workflows/docs-deploy.yml');
const appWorkflowPath = resolve(repositoryRoot, '.github/workflows/deploy.yml');

test('documentation deploys independently for main and develop', async () => {
  const workflowFile = Bun.file(docsWorkflowPath);
  expect(await workflowFile.exists()).toBe(true);
  const workflow = await workflowFile.text();

  expect(workflow).toContain('main');
  expect(workflow).toContain('develop');
  expect(workflow).toContain('https://molii.co');
  expect(workflow).toContain('https://dev.molii.co');
  expect(workflow).toContain('DOCS_BASE_URL: /docs/');
  expect(workflow).toContain("DOCS_ENV: ${{ needs.prepare.outputs.environment }}");
  expect(workflow).toContain("cancel-in-progress: false");
  expect(workflow).toContain('docs-deploy-${{ needs.prepare.outputs.environment }}');
});

test('documentation build uses the pinned toolchain and complete safety gates', async () => {
  const workflow = await Bun.file(docsWorkflowPath).text();

  expect(workflow).toContain("node-version: '22.12.0'");
  expect(workflow).toContain("bun-version: '1.3.14'");
  expect(workflow).toContain('bun install --frozen-lockfile');
  expect(workflow).toContain("! -name '*.browser.test.ts'");
  expect(workflow).toContain('test_files+=("$test_file")');
  expect(workflow).toContain('bun test "${test_files[@]}"');
  expect(workflow).not.toContain('bun test $test_files');
  expect(workflow).toContain('bun test scripts/api-reference.browser.test.ts');
  expect(workflow).toMatch(
    /- name: Run browser documentation tests\n\s+working-directory: docs-site\n\s+env:\n\s+DOCS_ENV: development\n\s+DOCS_SITE_URL: http:\/\/127\.0\.0\.1:3197\n\s+DOCS_BASE_URL: \/\n\s+DOCS_API_BASE_URL: http:\/\/127\.0\.0\.1:3000\n\s+run:/,
  );
  expect(workflow).toContain('bun run check:forbidden');
  expect(workflow).toContain('bun run check:secrets');
  expect(workflow).toContain('bun run api:lint');
  expect(workflow).toContain('bun run catalog:check');
  expect(workflow).toContain('bun run build');
  expect(workflow).toContain('bun run check:links');
  expect(workflow).toContain('actions/upload-artifact@');
  expect(workflow).toContain('actions/download-artifact@');
});

test('injects the public Algolia search configuration only into Development builds', async () => {
  const workflow = await Bun.file(docsWorkflowPath).text();

  for (const variableName of [
    'DOCS_ALGOLIA_APP_ID',
    'DOCS_ALGOLIA_SEARCH_API_KEY',
    'DOCS_ALGOLIA_INDEX_NAME',
  ]) {
    expect(workflow).toContain(`vars.${variableName}`);
    expect(workflow).toContain(
      `needs.prepare.outputs.environment == 'development' && vars.${variableName} || ''`,
    );
    expect(workflow).not.toContain(`secrets.${variableName}`);
  }
});

test('browser checks use a production build instead of the HMR development server', async () => {
  const browserTest = await Bun.file(
    resolve(repositoryRoot, 'docs-site/scripts/api-reference.browser.test.ts'),
  ).text();

  expect(browserTest).toContain("'docusaurus', 'build'");
  expect(browserTest).toContain("'docusaurus', 'serve'");
  expect(browserTest).not.toContain("'docusaurus', 'start'");
  expect(browserTest).not.toContain("waitUntil: 'networkidle'");
  expect(browserTest).toContain("waitUntil: 'domcontentloaded'");
  expect(browserTest.match(/chromium\.launch/g)).toHaveLength(1);
  expect(browserTest).toContain('await page.close()');
});

test('documentation deployment reuses only infrastructure credentials', async () => {
  const workflow = await Bun.file(docsWorkflowPath).text();

  for (const secretName of [
    'DEPLOY_SSH_HOST',
    'DEPLOY_SSH_PORT',
    'DEPLOY_SSH_USER',
    'DEPLOY_SSH_PRIVATE_KEY',
    'DEPLOY_SSH_KNOWN_HOSTS',
    'TELEGRAM_BOT_TOKEN',
    'TELEGRAM_CHAT_ID',
  ]) {
    expect(workflow).toContain(`secrets.${secretName}`);
  }

  expect(workflow).toContain('docs-site/deploy/deploy.sh');
  expect(workflow).toContain('sha256sum');
  expect(workflow).toContain('molii-docs-${GITHUB_SHA}.tar.gz');
  expect(workflow).not.toMatch(
    /SQL_DSN|REDIS_CONN_STRING|SESSION_SECRET|CRYPTO_SECRET|MOLII_API_KEY/,
  );
});

test('application deployment ignores documentation-only pushes', async () => {
  const appWorkflow = await Bun.file(appWorkflowPath).text();

  expect(appWorkflow).toContain('paths-ignore:');
  expect(appWorkflow).toContain("- 'docs-site/**'");
  expect(appWorkflow).toContain("- '.github/workflows/docs-deploy.yml'");
  expect(appWorkflow).toContain("- '.ccg/tasks/**'");
  expect(appWorkflow).toContain("- 'docs/superpowers/**'");
});
