import { afterEach, describe, expect, test } from 'bun:test';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { prepareOpenApi } from './prepare-openapi.mjs';

const workspaces: string[] = [];

afterEach(async () => {
  await Promise.all(workspaces.splice(0).map((workspace) => rm(workspace, { recursive: true, force: true })));
});

const publicSurface = [
  'GET /v1/models',
  'POST /v1/video/generations',
  'GET /v1/video/generations/{task_id}',
  'POST /v1/images/generations',
  'POST /v1/images/edits',
  'POST /v1/videos',
  'POST /v1/videos/edits',
  'GET /v1/videos/{task_id}',
  'GET /v1/videos/{task_id}/content',
  'POST /v1/assets',
  'GET /v1/assets/{id}',
  'DELETE /v1/assets/{id}',
];

async function fixture() {
  const workspace = await mkdtemp(join(tmpdir(), 'molii-openapi-'));
  workspaces.push(workspace);
  const templatePath = join(workspace, 'relay.public.template.yaml');
  const allowlistPath = join(workspace, 'public-api-surface.json');
  const outputPath = join(workspace, 'generated', 'relay.public.json');

  const paths = publicSurface.reduce<Record<string, Record<string, unknown>>>((result, entry, index) => {
      const [method, path] = entry.split(' ');
      result[path] ??= {};
      result[path][method.toLowerCase()] = {
          summary: `Public operation ${index + 1}`,
          operationId: `publicOperation${index + 1}`,
          security: [{ BearerAuth: [] }],
          responses: {
            '200': {
              description: 'Success',
              content: { 'application/json': { schema: { $ref: '#/components/schemas/PublicResponse' } } },
            },
            '401': { description: 'Authentication failed' },
          },
      };
      return result;
    }, {});
  paths['/api/channel'] = {
    get: {
      summary: 'Private channel administration',
      operationId: 'privateChannelList',
      security: [{ AdminAuth: [] }],
      responses: { '200': { description: 'Private' } },
    },
  };

  await writeFile(templatePath, JSON.stringify({
    openapi: '3.1.0',
    info: { title: 'Fixture', version: '1.0.0' },
    servers: [{ url: 'https://private.invalid' }],
    paths,
    components: {
      securitySchemes: {
        BearerAuth: { type: 'http', scheme: 'bearer' },
        AdminAuth: { type: 'apiKey', in: 'header', name: 'X-Admin-Key' },
      },
      schemas: {
        PublicResponse: { type: 'object', properties: { id: { type: 'string' } } },
        Administrator: { type: 'object', properties: { upstreamCredential: { type: 'string' } } },
      },
    },
  }, null, 2));
  await writeFile(allowlistPath, JSON.stringify({ operations: publicSurface }, null, 2));
  return { templatePath, allowlistPath, outputPath };
}

describe('prepareOpenApi', () => {
  test('publishes exactly the explicit public surface', async () => {
    const files = await fixture();

    const document = await prepareOpenApi({ ...files, apiBaseUrl: 'https://api.molii.example' });

    const operations = Object.entries(document.paths).flatMap(([path, item]) =>
      Object.keys(item).map((method) => `${method.toUpperCase()} ${path}`),
    ).sort();
    expect(operations).toEqual([...publicSurface].sort());
  });

  test('keeps stable unique operation ids and only bearer authentication', async () => {
    const files = await fixture();

    const document = await prepareOpenApi({ ...files, apiBaseUrl: 'https://api.molii.example' });

    const operations = Object.values(document.paths).flatMap((item) => Object.values(item));
    const operationIds = operations.map((operation: any) => operation.operationId);
    expect(new Set(operationIds).size).toBe(operationIds.length);
    expect(operationIds).toEqual(publicSurface.map((_, index) => `publicOperation${index + 1}`));
    expect(document.components.securitySchemes).toEqual({ BearerAuth: { type: 'http', scheme: 'bearer' } });
    expect(operations.every((operation: any) => JSON.stringify(operation.security) === JSON.stringify([{ BearerAuth: [] }]))).toBe(true);
  });

  test('sets the public server, prunes private schemas, and leaves source input unchanged', async () => {
    const files = await fixture();
    const input = JSON.parse(await Bun.file(files.templatePath).text());
    const original = structuredClone(input);

    const document = await prepareOpenApi({ ...files, apiBaseUrl: 'https://api.molii.example', document: input });

    expect(document.servers).toEqual([{ url: 'https://api.molii.example' }]);
    expect(document.components.schemas).toEqual({
      PublicResponse: { type: 'object', properties: { id: { type: 'string' } } },
    });
    expect(input).toEqual(original);
  });

  test('rejects unsafe or incomplete public operation definitions', async () => {
    const files = await fixture();
    const unsafe = JSON.parse(await Bun.file(files.templatePath).text());
    unsafe.paths['/v1/models'].get.security = [{ AdminAuth: [] }];
    unsafe.paths['/v1/videos'].post.summary = '';

    await expect(prepareOpenApi({ ...files, apiBaseUrl: 'https://api.molii.example', document: unsafe }))
      .rejects.toThrow(/summary|BearerAuth/i);
  });
});
