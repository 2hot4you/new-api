import { describe, expect, test } from 'bun:test';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

import sidebars from '../sidebars';

const siteRoot = join(import.meta.dir, '..');
const repoRoot = join(siteRoot, '..');
const platformDirectory = 'docs/platform';
const sourceCommit = 'ce71f3ccab9d';

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
      'web/src/features/temporary-assets/components/create-asset-card.tsx',
      'web/src/features/temporary-assets/lib/asset-utils.ts',
    ],
    sharedSourceEvidence: {
      file: 'web/src/features/temporary-assets/index.tsx',
      ordinaryUserMarkers: [
        "'/api/assets/self'",
        "'/api/assets/self/upload-config'",
      ],
    },
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

async function source(relativePath: string) {
  return readFile(join(repoRoot, relativePath), 'utf8');
}

function frontmatter(sourceText: string): Record<string, string> {
  const match = sourceText.match(/^---\n([\s\S]*?)\n---/);
  if (!match) return {};

  return Object.fromEntries(
    match[1].split('\n').map((line) => {
      const separator = line.indexOf(':');
      return [line.slice(0, separator).trim(), line.slice(separator + 1).trim()];
    }),
  );
}

describe('ordinary-user platform documentation contract', () => {
  test('manifest sources exist and the shared temporary-assets page proves its ordinary-user branch', async () => {
    for (const entry of Object.values(guides)) {
      for (const file of [...entry.routeFiles, ...entry.sourceFiles]) {
        expect(file).toMatch(/^web\/src\/(routes|features)\//);
        expect(forbiddenScope.some((forbidden) => file.includes(forbidden))).toBe(false);
        expect((await source(file)).length, file).toBeGreaterThan(0);
      }

      if ('sharedSourceEvidence' in entry) {
        const sharedSource = await source(entry.sharedSourceEvidence.file);
        for (const marker of entry.sharedSourceEvidence.ordinaryUserMarkers) {
          expect(sharedSource, marker).toContain(marker);
        }
      }
    }
  });

  test('every ordinary-user guide has consistent provenance and a safe screenshot placeholder', async () => {
    for (const id of Object.keys(guides) as Array<keyof typeof guides>) {
      const sourceText = await guide(id);
      const metadata = frontmatter(sourceText);
      expect(metadata.audience, id).toBe('user');
      expect(metadata.apiVersion, id).toBe('v1');
      expect(metadata.lastReviewed, id).toBe('2026-08-10');
      expect(metadata.sourceCommit, id).toBe(sourceCommit);
      expect(sourceText, id).toContain('## 截图占位');
      expect(sourceText, id).toMatch(/alt[：:]/);
      expect(sourceText, id).toMatch(/脱敏|合成/);
      for (const forbidden of forbiddenScope) expect(sourceText, id).not.toContain(forbidden);
    }
  });

  test('registration and reset-password steps match the current user flows', async () => {
    const documentation = await guide('register-and-sign-in');
    const registration = await source(
      'web/src/features/auth/sign-up/components/sign-up-form.tsx',
    );
    const reset = await source(
      'web/src/features/auth/reset-password-confirm/index.tsx',
    );

    expect(registration).toContain('const emailVerificationRequired = !!status?.email_verification');
    expect(registration).toContain("name='confirmPassword'");
    expect(registration).toContain("t('Account created! Please sign in')");
    expect(registration).toContain('redirectToLogin()');
    expect(documentation).toMatch(/用户名、密码和确认密码/);
    expect(documentation).toMatch(/仅当.*启用邮箱验证.*邮箱.*验证码/);
    expect(documentation).toMatch(/注册成功.*请登录.*\/sign-in/);
    expect(documentation).not.toContain('进入登录后的页面');

    expect(reset).toContain("api.post('/api/user/reset', { email, token }");
    expect(reset).toContain('setNewPassword(password)');
    expect(documentation).toMatch(/email.*token/i);
    expect(documentation).toMatch(/确认重置.*生成.*新密码/);
    expect(documentation).toMatch(/复制.*新密码.*登录/);
    expect(documentation).not.toContain('设置新密码');
  });

  test('API key guide documents row and batch retrieval of real keys safely', async () => {
    const documentation = await guide('api-keys');
    const rowCell = await source('web/src/features/keys/components/api-keys-cells.tsx');
    const bulkActions = await source(
      'web/src/features/keys/components/data-table-bulk-actions.tsx',
    );

    expect(rowCell).toContain('resolveRealKey(apiKey.id)');
    expect(bulkActions).toContain('resolveRealKeysBatch(ids)');
    expect(documentation).toMatch(/点击.*遮罩.*完整.*Key/i);
    expect(documentation).toMatch(/批量.*复制.*所选.*Key/i);
    expect(documentation).toMatch(/不要.*截图|不得.*截图/);
    expect(documentation).not.toMatch(/只会在创建时显示一次|无法.*找回/);
  });

  test('wallet guide names Order History and limits it to topup/payment records', async () => {
    const documentation = await guide('wallet-and-billing');
    const rechargeCard = await source(
      'web/src/features/wallet/components/recharge-form-card.tsx',
    );
    const historyDialog = await source(
      'web/src/features/wallet/components/dialogs/billing-history-dialog.tsx',
    );

    expect(rechargeCard).toContain("t('Order History')");
    expect(historyDialog).toContain('View your topup transaction records and payment history');
    expect(documentation).toContain('“订单历史”');
    expect(documentation).toMatch(/充值交易记录.*支付历史/);
    expect(documentation).not.toMatch(/充值、消费、退款|核对消费|核对退款/);
  });

  test('temporary-assets guide keeps URL creation available when local upload is disabled', async () => {
    const documentation = await guide('temporary-assets');
    const createCard = await source(
      'web/src/features/temporary-assets/components/create-asset-card.tsx',
    );

    expect(createCard).toContain('disabled={!props.uploadConfig.enabled || submitting}');
    expect(createCard).toContain("<TabsTrigger value='url'>{t('Add by URL')}</TabsTrigger>");
    expect(createCard).toContain("await api.post('/api/assets/self',");
    expect(documentation).toMatch(/本地上传.*显示.*禁用/);
    expect(documentation).toMatch(/URL.*仍可|仍可.*URL/);
    expect(documentation).not.toMatch(/未显示创建控件/);
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

  test('sidebar lists all eight ordinary-user guides in the platform and account category', () => {
    const sidebar = JSON.stringify(sidebars);
    for (const id of Object.keys(guides)) {
      expect(sidebar).toContain(`platform/${id}`);
    }
    expect(sidebar).toContain('平台与账户');
  });
});
