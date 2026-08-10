import { constants } from 'node:fs';
import { access } from 'node:fs/promises';

const PLATFORM_CANDIDATES = {
  darwin: [
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    '/Applications/Chromium.app/Contents/MacOS/Chromium',
    '/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary',
  ],
  linux: [
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/snap/bin/chromium',
  ],
};

async function isExecutableFile(path) {
  try {
    await access(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export async function resolveBrowserExecutable({
  env = process.env,
  platform = process.platform,
  isExecutable = isExecutableFile,
} = {}) {
  const configuredPath = env.DOCS_CHROME_PATH?.trim();
  if (configuredPath) {
    if (await isExecutable(configuredPath)) return configuredPath;
    throw new Error(`DOCS_CHROME_PATH is not executable: ${configuredPath}`);
  }

  const candidates = PLATFORM_CANDIDATES[platform] ?? [];
  for (const candidate of candidates) {
    if (await isExecutable(candidate)) return candidate;
  }

  const checked = candidates.length > 0 ? candidates.join(', ') : 'no default paths for this platform';
  throw new Error(
    `No Chrome/Chromium executable found for ${platform}. Set DOCS_CHROME_PATH to an installed browser. Checked: ${checked}`,
  );
}
