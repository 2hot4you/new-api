import { describe, expect, test } from 'bun:test';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const platformDirectory = 'docs/platform';

const guides = {
  'register-and-sign-in': {
    routeFiles: [
      'web/src/routes/(auth)/sign-up.tsx',
      'web/src/routes/(auth)/sign-in.tsx',
      'web/src/routes/(auth)/forgot-password.tsx',
      'web/src/routes/(auth)/otp.tsx',
      'web/src/routes/(auth)/reset.tsx',
    ],
    sourceFiles: [
      'web/src/features/auth/sign-up/components/sign-up-form.tsx',
      'web/src/features/auth/sign-in/components/user-auth-form.tsx',
      'web/src/features/auth/forgot-password/components/forgot-password-form.tsx',
      'web/src/features/auth/otp/components/otp-form.tsx',
    ],
  },
  dashboard: {
    routeFiles: ['web/src/routes/_authenticated/dashboard/$section.tsx'],
    sourceFiles: [
      'web/src/features/dashboard/index.tsx',
      'web/src/features/dashboard/section-registry.tsx',
      'web/src/features/dashboard/components/overview/overview-dashboard.tsx',
    ],
  },
  'api-keys': {
    routeFiles: ['web/src/routes/_authenticated/keys/index.tsx'],
    sourceFiles: [
      'web/src/features/keys/index.tsx',
      'web/src/features/keys/components/api-keys-mutate-drawer.tsx',
      'web/src/features/keys/components/api-keys-table.tsx',
    ],
  },
  'model-square-and-playground': {
    routeFiles: [
      'web/src/routes/pricing/index.tsx',
      'web/src/routes/pricing/$modelId/index.tsx',
      'web/src/routes/_authenticated/playground/index.tsx',
    ],
    sourceFiles: [
      'web/src/features/pricing/index.tsx',
      'web/src/features/playground/index.tsx',
      'web/src/features/playground/components/input/playground-input.tsx',
    ],
  },
  'temporary-assets': {
    routeFiles: ['web/src/routes/_authenticated/temporary-assets/index.tsx'],
    sourceFiles: [
      'web/src/features/temporary-assets/index.tsx',
      'web/src/features/temporary-assets/components/create-asset-card.tsx',
      'web/src/features/temporary-assets/lib/asset-utils.ts',
    ],
  },
  'usage-and-generation-records': {
    routeFiles: ['web/src/routes/_authenticated/usage-logs/$section.tsx'],
    sourceFiles: [
      'web/src/features/usage-logs/index.tsx',
      'web/src/features/usage-logs/section-registry.tsx',
      'web/src/features/usage-logs/components/usage-logs-table.tsx',
    ],
  },
  'wallet-and-billing': {
    routeFiles: ['web/src/routes/_authenticated/wallet/index.tsx'],
    sourceFiles: [
      'web/src/features/wallet/index.tsx',
      'web/src/features/wallet/components/recharge-form-card.tsx',
      'web/src/features/wallet/components/dialogs/billing-history-dialog.tsx',
    ],
  },
  'profile-and-security': {
    routeFiles: ['web/src/routes/_authenticated/profile/index.tsx'],
    sourceFiles: [
      'web/src/features/profile/index.tsx',
      'web/src/features/profile/components/profile-settings-card.tsx',
      'web/src/features/profile/components/login-sessions-card.tsx',
      'web/src/features/profile/components/two-fa-card.tsx',
    ],
  },
} as const;

const forbiddenScope = [
  '/channels',
  '/users',
  '/system-settings',
  '/api/assets/admin',
  '管理 API',
  '管理员导航',
];

async function guide(id: keyof typeof guides) {
  return readFile(join(siteRoot, platformDirectory, `${id}.mdx`), 'utf8');
}

describe('ordinary-user platform documentation contract', () => {
  test('manifest contains only ordinary-user routes and feature sources', () => {
    for (const entry of Object.values(guides)) {
      for (const file of [...entry.routeFiles, ...entry.sourceFiles]) {
        expect(file).toMatch(/^web\/src\/(routes|features)\//);
        expect(forbiddenScope.some((forbidden) => file.includes(forbidden))).toBe(false);
      }
    }
  });

  test('every ordinary-user guide is present, marked for users, and has a safe screenshot placeholder', async () => {
    for (const id of Object.keys(guides) as Array<keyof typeof guides>) {
      const source = await guide(id);
      expect(source, id).toContain('audience: user');
      expect(source, id).toContain('## 截图占位');
      expect(source, id).toMatch(/alt[：:]/);
      expect(source, id).toMatch(/脱敏|合成/);
      for (const forbidden of forbiddenScope) expect(source, id).not.toContain(forbidden);
    }
  });

  test('the guides cover success, empty, disabled, expired, and error states without exposing sensitive screenshots', async () => {
    const source = await Promise.all(
      (Object.keys(guides) as Array<keyof typeof guides>).map(guide),
    ).then((pages) => pages.join('\n'));

    for (const state of ['成功', '空状态', '未启用', '已过期', '错误']) {
      expect(source).toContain(state);
    }
    expect(await readdir(join(siteRoot, 'static/img/platform'))).toEqual(['.gitkeep']);
  });

  test('model square uses the public pricing routes and Playground is explicitly chat-only', async () => {
    const source = await guide('model-square-and-playground');
    expect(source).toContain('/pricing');
    expect(source).toContain('/playground');
    expect(source).toMatch(/仅支持.*chat|仅支持.*对话/);
    expect(source).toMatch(/不支持.*Seedance/);
    expect(source).toMatch(/不支持.*Grok/);
  });

  test('sidebar lists all eight ordinary-user guides in a dedicated category', () => {
    const sidebar = JSON.stringify(sidebars);
    for (const id of Object.keys(guides)) {
      expect(sidebar).toContain(`platform/${id}`);
    }
    expect(sidebar).toContain('用户平台操作');
  });
});
