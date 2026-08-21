import { mkdir, rename, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';

import { CATALOG_SOURCE, sanitizeCatalogResponse } from './model-catalog.mjs';

const outputPath = resolve(import.meta.dirname, '../data/development-model-catalog.json');

async function main() {
  const response = await fetch(CATALOG_SOURCE);
  if (!response.ok) throw new Error(`Catalog sync failed: ${response.status} ${response.statusText}`);
  const catalog = sanitizeCatalogResponse(await response.json());
  await mkdir(dirname(outputPath), { recursive: true });
  const temporaryPath = `${outputPath}.tmp-${process.pid}`;
  await writeFile(temporaryPath, `${JSON.stringify(catalog, null, 2)}\n`);
  await rename(temporaryPath, outputPath);
  console.log(`Synced ${catalog.vendors.length} Providers and ${catalog.models.length} models from ${CATALOG_SOURCE}.`);
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});
