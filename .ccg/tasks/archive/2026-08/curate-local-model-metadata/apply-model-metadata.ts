import { resolve } from 'node:path'

import type { CuratedManifest, CuratedModel } from './validate-manifest'
import { validateManifest } from './validate-manifest'

const containerName = 'molii-new-api-dev-postgres'
const defaultManifestPath = resolve(import.meta.dir, 'source-manifest.json')

function sqlPayload(models: CuratedModel[]): string {
  return JSON.stringify(
    models.map((model) => ({
      model_name: model.model_name,
      description: model.description,
      description_en: model.description_en,
      icon: model.icon,
      release_date: model.release_date,
      knowledge_cutoff: model.knowledge_cutoff,
      input_modalities: JSON.stringify(model.input_modalities),
      output_modalities: JSON.stringify(model.output_modalities),
      capabilities: JSON.stringify(model.capabilities),
      supported_parameters: JSON.stringify(model.supported_parameters),
      context_length: model.context_length,
      max_output_tokens: model.max_output_tokens,
    }))
  )
}

function dollarQuote(payload: string): string {
  for (let suffix = 0; ; suffix += 1) {
    const tag = suffix === 0 ? '$molii_metadata$' : `$molii_metadata_${suffix}$`
    if (!payload.includes(tag)) return `${tag}${payload}${tag}`
  }
}

export function buildApplySQL(manifest: CuratedManifest): string {
  const errors = validateManifest(manifest)
  if (errors.length > 0) throw new Error(errors.join('\n'))

  const payload = dollarQuote(sqlPayload(manifest.models))
  return `BEGIN;

CREATE TEMP TABLE molii_model_metadata_payload (
  model_name text PRIMARY KEY,
  description text NOT NULL,
  description_en text NOT NULL,
  icon text NOT NULL,
  release_date text NOT NULL,
  knowledge_cutoff text NOT NULL,
  input_modalities text NOT NULL,
  output_modalities text NOT NULL,
  capabilities text NOT NULL,
  supported_parameters text NOT NULL,
  context_length bigint NOT NULL,
  max_output_tokens bigint NOT NULL
) ON COMMIT DROP;

INSERT INTO molii_model_metadata_payload
SELECT *
FROM jsonb_to_recordset(${payload}::jsonb) AS x(
  model_name text,
  description text,
  description_en text,
  icon text,
  release_date text,
  knowledge_cutoff text,
  input_modalities text,
  output_modalities text,
  capabilities text,
  supported_parameters text,
  context_length bigint,
  max_output_tokens bigint
);

DO $guard$
BEGIN
  IF (SELECT count(*) FROM molii_model_metadata_payload) <> 17 THEN
    RAISE EXCEPTION 'manifest target count changed';
  END IF;
  IF EXISTS (
    (SELECT model_name FROM models WHERE deleted_at IS NULL
     EXCEPT SELECT model_name FROM molii_model_metadata_payload)
    UNION ALL
    (SELECT model_name FROM molii_model_metadata_payload
     EXCEPT SELECT model_name FROM models WHERE deleted_at IS NULL)
  ) THEN
    RAISE EXCEPTION 'target model set changed';
  END IF;
END
$guard$;

DO $apply$
DECLARE
  updated_count integer;
BEGIN
  UPDATE models AS model
  SET description = payload.description,
      description_en = payload.description_en,
      icon = payload.icon,
      release_date = payload.release_date,
      knowledge_cutoff = payload.knowledge_cutoff,
      input_modalities = payload.input_modalities,
      output_modalities = payload.output_modalities,
      capabilities = payload.capabilities,
      supported_parameters = payload.supported_parameters,
      context_length = payload.context_length,
      max_output_tokens = payload.max_output_tokens
  FROM molii_model_metadata_payload AS payload
  WHERE model.model_name = payload.model_name
    AND model.deleted_at IS NULL;

  GET DIAGNOSTICS updated_count = ROW_COUNT;
  IF updated_count <> 17 THEN
    RAISE EXCEPTION 'expected to update 17 model rows, updated %', updated_count;
  END IF;
END
$apply$;

COMMIT;
`
}

async function loadManifest(path: string): Promise<CuratedManifest> {
  return (await Bun.file(path).json()) as CuratedManifest
}

function executeSQL(sql: string): number {
  const result = Bun.spawnSync({
    cmd: [
      'docker',
      'exec',
      '-i',
      containerName,
      'sh',
      '-lc',
      'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1',
    ],
    stdin: Buffer.from(sql),
    stdout: 'inherit',
    stderr: 'inherit',
  })
  return result.exitCode
}

if (import.meta.main) {
  const mode = process.argv[2]
  const manifestPath = resolve(process.argv[3] ?? defaultManifestPath)
  if (mode !== '--preview' && mode !== '--execute') {
    console.error('usage: bun apply-model-metadata.ts <--preview|--execute> [manifest.json]')
    process.exit(2)
  }

  const manifest = await loadManifest(manifestPath)
  const sql = buildApplySQL(manifest)
  if (mode === '--preview') {
    process.stdout.write(sql)
  } else {
    process.exit(executeSQL(sql))
  }
}
