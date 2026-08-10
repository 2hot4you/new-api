import { describe, expect, test } from 'bun:test';

import { resolveBrowserExecutable } from './browser-executable.mjs';

describe('resolveBrowserExecutable', () => {
  test('uses an executable DOCS_CHROME_PATH before platform candidates', async () => {
    const checked: string[] = [];
    const result = await resolveBrowserExecutable({
      env: { DOCS_CHROME_PATH: '/custom/chrome' },
      platform: 'linux',
      isExecutable: async (path: string) => {
        checked.push(path);
        return path === '/custom/chrome';
      },
    });

    expect(result).toBe('/custom/chrome');
    expect(checked).toEqual(['/custom/chrome']);
  });

  test('discovers common macOS and Linux browser installations', async () => {
    const mac = await resolveBrowserExecutable({
      env: {},
      platform: 'darwin',
      isExecutable: async (path: string) => path === '/Applications/Chromium.app/Contents/MacOS/Chromium',
    });
    const linux = await resolveBrowserExecutable({
      env: {},
      platform: 'linux',
      isExecutable: async (path: string) => path === '/usr/bin/chromium-browser',
    });

    expect(mac).toBe('/Applications/Chromium.app/Contents/MacOS/Chromium');
    expect(linux).toBe('/usr/bin/chromium-browser');
  });

  test('reports how to configure the smoke test when no browser exists', async () => {
    await expect(resolveBrowserExecutable({
      env: {},
      platform: 'linux',
      isExecutable: async () => false,
    })).rejects.toThrow(/Chrome\/Chromium.*DOCS_CHROME_PATH.*\/usr\/bin\/google-chrome/s);

    await expect(resolveBrowserExecutable({
      env: { DOCS_CHROME_PATH: '/missing/chrome' },
      platform: 'linux',
      isExecutable: async () => false,
    })).rejects.toThrow(/DOCS_CHROME_PATH.*not executable.*\/missing\/chrome/s);
  });
});
