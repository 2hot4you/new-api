import { mkdir, open, rename, rm } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { randomUUID } from 'node:crypto';

import { resolvePublicConfig, type PublicEnvironment } from '../src/config';

const MAX_ASSET_BYTES = 2 * 1024 * 1024;
const ALLOWED_CONTENT_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/svg+xml',
  'image/webp',
  'image/x-icon',
  'image/vnd.microsoft.icon',
]);

type AssetEnvironment = 'development' | 'production';

interface DownloadPublicAssetOptions {
  destination: string;
  environment: AssetEnvironment;
  source: string;
}

function validateSourceUrl(source: string, environment: AssetEnvironment): URL {
  let parsed: URL;
  try {
    parsed = new URL(source);
  } catch {
    throw new Error('Brand asset source must be a valid HTTP(S) URL.');
  }

  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw new Error('Brand asset source must use HTTP(S).');
  }
  if (environment === 'production' && parsed.protocol !== 'https:') {
    throw new Error('Production brand asset source must use HTTPS.');
  }
  if (parsed.username || parsed.password) {
    throw new Error('Brand asset source must not contain URL credentials.');
  }
  return parsed;
}

function validateSvg(bytes: Uint8Array): void {
  const svg = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  const unsafe = [
    /<\s*script\b/i,
    /\bon[a-z]+\s*=/i,
    /(?:href|src)\s*=\s*["']\s*(?:https?:|\/\/|data:|javascript:)/i,
    /url\(\s*["']?\s*(?:https?:|\/\/|data:|javascript:)/i,
    /<\s*(?:foreignObject|iframe|object|embed)\b/i,
  ];
  if (!/<\s*svg\b/i.test(svg) || unsafe.some((pattern) => pattern.test(svg))) {
    throw new Error('Brand asset contains unsafe SVG content.');
  }
}

async function readBoundedBody(response: Response): Promise<Uint8Array> {
  const declaredLength = Number(response.headers.get('content-length') ?? '0');
  if (Number.isFinite(declaredLength) && declaredLength > MAX_ASSET_BYTES) {
    throw new Error('Brand asset exceeds the 2 MiB limit.');
  }

  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error('Brand asset response is empty.');
  }

  const chunks: Uint8Array[] = [];
  let total = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_ASSET_BYTES) {
      await reader.cancel();
      throw new Error('Brand asset exceeds the 2 MiB limit.');
    }
    chunks.push(value);
  }

  if (total === 0) {
    throw new Error('Brand asset response is empty.');
  }

  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body;
}

async function writeAtomically(destination: string, bytes: Uint8Array): Promise<void> {
  const directory = dirname(destination);
  const temporary = join(directory, `.${randomUUID()}.tmp`);
  await mkdir(directory, { recursive: true });

  try {
    const file = await open(temporary, 'wx', 0o600);
    try {
      await file.writeFile(bytes);
    } finally {
      await file.close();
    }
    await rename(temporary, destination);
  } finally {
    await rm(temporary, { force: true });
  }
}

export async function downloadPublicAsset(
  options: DownloadPublicAssetOptions,
): Promise<void> {
  const source = validateSourceUrl(options.source, options.environment);
  const response = await fetch(source, {
    redirect: 'manual',
    signal: AbortSignal.timeout(10_000),
  });

  if (response.status >= 300 && response.status < 400) {
    throw new Error('Brand asset redirect responses are not allowed.');
  }
  if (!response.ok) {
    throw new Error(`Brand asset request failed with HTTP ${response.status}.`);
  }

  const contentType = response.headers.get('content-type')?.split(';', 1)[0]?.trim().toLowerCase();
  if (!contentType || !ALLOWED_CONTENT_TYPES.has(contentType)) {
    throw new Error('Brand asset response has an unsupported content type.');
  }

  const bytes = await readBoundedBody(response);
  if (contentType === 'image/svg+xml') {
    validateSvg(bytes);
  }
  await writeAtomically(options.destination, bytes);
}

function requiredSource(environment: PublicEnvironment, name: string): string {
  const value = environment[name]?.trim();
  if (!value) {
    throw new Error(`${name} must be set before preparing documentation brand assets.`);
  }
  return value;
}

export async function prepareBrandAssets(
  environment: PublicEnvironment,
  staticDirectory = join(import.meta.dir, '..', 'static'),
): Promise<void> {
  const publicConfig = resolvePublicConfig(environment);
  const docsEnvironment = environment.DOCS_ENV as AssetEnvironment;
  const assets = [
    {
      destination: publicConfig.brand.logoPath,
      source: requiredSource(environment, 'DOCS_LOGO_SOURCE_URL'),
    },
    {
      destination: publicConfig.brand.faviconPath,
      source: requiredSource(environment, 'DOCS_FAVICON_SOURCE_URL'),
    },
    {
      destination: publicConfig.brand.socialImagePath,
      source: requiredSource(environment, 'DOCS_SOCIAL_IMAGE_SOURCE_URL'),
    },
  ];

  await Promise.all(
    assets.map((asset) =>
      downloadPublicAsset({
        destination: join(staticDirectory, asset.destination),
        environment: docsEnvironment,
        source: asset.source,
      }),
    ),
  );
}

if (import.meta.main) {
  await prepareBrandAssets(process.env);
  console.log('Documentation brand assets prepared.');
}
