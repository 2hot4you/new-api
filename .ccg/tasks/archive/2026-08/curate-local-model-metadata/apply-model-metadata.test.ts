import { describe, expect, test } from 'bun:test'

import manifest from './source-manifest.json'
import { buildApplySQL } from './apply-model-metadata'

const containerName = 'molii-new-api-dev-postgres'
const dockerAvailable =
  Bun.spawnSync({ cmd: ['docker', 'inspect', containerName], stdout: 'ignore', stderr: 'ignore' }).exitCode === 0

function runPSQL(sql: string) {
  return Bun.spawnSync({
    cmd: [
      'docker',
      'exec',
      '-i',
      containerName,
      'sh',
      '-lc',
      'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -AtF "|"',
    ],
    stdin: Buffer.from(sql),
    stdout: 'pipe',
    stderr: 'pipe',
  })
}

function tempModelsSetup(modelNames: string[]): string {
  const rows = JSON.stringify(modelNames.map((model_name) => ({ model_name })))
  return `
CREATE TEMP TABLE models (
  model_name text PRIMARY KEY,
  description text,
  description_en text NOT NULL DEFAULT '',
  icon text,
  release_date text,
  knowledge_cutoff text,
  input_modalities text,
  output_modalities text,
  capabilities text,
  supported_parameters text NOT NULL DEFAULT '[]',
  context_length bigint,
  max_output_tokens bigint,
  deleted_at timestamptz,
  protected_value text NOT NULL DEFAULT 'keep'
);
INSERT INTO models (model_name)
SELECT model_name
FROM jsonb_to_recordset($fixture$${rows}$fixture$::jsonb) AS x(model_name text);
`
}

describe.skipIf(!dockerAvailable)('buildApplySQL PostgreSQL contract', () => {
  test('updates all 17 target rows while preserving non-target columns and unicode quoting', () => {
    const input = structuredClone(manifest)
    input.models[0].description = "O'Reilly 的模型\n第二行"
    const sql = `${tempModelsSetup(input.models.map((model) => model.model_name))}
${buildApplySQL(input)}
SELECT count(*), count(*) FILTER (WHERE description <> ''), min(protected_value), max(protected_value)
FROM models;
SELECT encode(convert_to(description, 'UTF8'), 'hex') FROM models WHERE model_name = 'minimax-m3';
`

    const result = runPSQL(sql)

    expect(result.exitCode).toBe(0)
    expect(result.stderr.toString()).toBe('')
    expect(result.stdout.toString()).toContain('17|17|keep|keep')
    expect(result.stdout.toString()).toContain(Buffer.from("O'Reilly 的模型\n第二行").toString('hex'))
  })

  test('rolls back when the database target set no longer matches the manifest', () => {
    const modelNames = manifest.models.slice(1).map((model) => model.model_name)
    const result = runPSQL(`${tempModelsSetup(modelNames)}\n${buildApplySQL(structuredClone(manifest))}`)

    expect(result.exitCode).not.toBe(0)
    expect(result.stderr.toString()).toContain('target model set changed')
  })
})
