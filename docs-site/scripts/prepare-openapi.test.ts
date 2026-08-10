import { afterEach, describe, expect, test } from 'bun:test';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import siteConfig from '../docusaurus.config';
import { prepareOpenApi, resetGeneratedApiDirectory } from './prepare-openapi.mjs';

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

  test('resets only the fixed generated API directory', async () => {
    const workspace = await mkdtemp(join(tmpdir(), 'molii-generated-api-'));
    workspaces.push(workspace);
    const generatedApi = join(workspace, 'generated', 'api');
    const generatedSibling = join(workspace, 'generated', 'keep.txt');
    const siteSibling = join(workspace, 'keep.txt');
    await mkdir(generatedApi, { recursive: true });
    await writeFile(join(generatedApi, 'stale.api.mdx'), '# stale');
    await writeFile(generatedSibling, 'generated sibling');
    await writeFile(siteSibling, 'site sibling');

    await resetGeneratedApiDirectory({ siteRoot: workspace });

    expect(await Array.fromAsync(new Bun.Glob('*').scan(generatedApi))).toEqual([]);
    expect(await readFile(generatedSibling, 'utf8')).toBe('generated sibling');
    expect(await readFile(siteSibling, 'utf8')).toBe('site sibling');
  });

  test('publishes Molii Grok image fields implemented by the adaptor', async () => {
    const workspace = await mkdtemp(join(tmpdir(), 'molii-owned-openapi-'));
    workspaces.push(workspace);
    const siteRoot = join(import.meta.dir, '..');
    const document = await prepareOpenApi({
      templatePath: join(siteRoot, 'openapi', 'relay.public.template.yaml'),
      allowlistPath: join(siteRoot, 'openapi', 'public-api-surface.json'),
      outputPath: join(workspace, 'relay.public.json'),
      apiBaseUrl: 'https://api.molii.example',
    });

    const generation = document.components.schemas.ImageGenerationRequest;
    expect(generation.properties).not.toHaveProperty('size');
    expect(generation.properties.resolution).toMatchObject({ enum: ['1k', '2k'], default: '1k' });
    expect(generation.properties.aspect_ratio).toMatchObject({
      enum: ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3', '2:1', '1:2', '19.5:9', '9:19.5', '20:9', '9:20', 'auto'],
      default: '16:9',
    });

    const editInput = document.components.schemas.ImageEditRequest.allOf[1];
    expect(editInput.anyOf).toEqual([{ required: ['image'] }, { required: ['images'] }]);
    expect(editInput.properties.image.$ref).toBe('#/components/schemas/ImageInput');
    expect(editInput.properties.images).toMatchObject({ minItems: 1, maxItems: 3 });

    const imageInput = document.components.schemas.ImageInput;
    expect(imageInput.oneOf[1]).toMatchObject({
      type: 'object',
      required: ['url'],
      properties: { url: { type: 'string', format: 'uri', minLength: 1 } },
    });
    expect(JSON.stringify(imageInput)).not.toContain('file_id');
    expect(JSON.stringify(document.paths['/v1/images/edits'])).not.toContain('file_id');
  });

  test('publishes only the supported Grok video 1.5 model example', async () => {
    const workspace = await mkdtemp(join(tmpdir(), 'molii-owned-openapi-'));
    workspaces.push(workspace);
    const siteRoot = join(import.meta.dir, '..');
    const document = await prepareOpenApi({
      templatePath: join(siteRoot, 'openapi', 'relay.public.template.yaml'),
      allowlistPath: join(siteRoot, 'openapi', 'public-api-surface.json'),
      outputPath: join(workspace, 'relay.public.json'),
      apiBaseUrl: 'https://api.molii.example',
    });
    const retiredModel = 'grok-imagine-video-1.5-' + 'pre' + 'view';
    const serialized = JSON.stringify(document);

    expect(serialized).toContain('grok-imagine-video-1.5');
    expect(serialized).not.toContain(retiredModel);
  });

  test('publishes the implemented asset request and mutation response shapes', async () => {
    const workspace = await mkdtemp(join(tmpdir(), 'molii-owned-openapi-'));
    workspaces.push(workspace);
    const siteRoot = join(import.meta.dir, '..');
    const document = await prepareOpenApi({
      templatePath: join(siteRoot, 'openapi', 'relay.public.template.yaml'),
      allowlistPath: join(siteRoot, 'openapi', 'public-api-surface.json'),
      outputPath: join(workspace, 'relay.public.json'),
      apiBaseUrl: 'https://api.molii.example',
    });

    const request = document.components.schemas.CreateAssetRequest;
    expect(request.required).toEqual(['url', 'asset_type', 'name']);
    expect(request.properties.name.maxLength).toBe(80);
    expect(request.properties.asset_type.enum).toEqual(['image', 'video', 'audio']);

    const createSchema = document.components.responses.CreateAssetResult.content['application/json'].schema;
    expect(createSchema.$ref).toBe('#/components/schemas/CreateAssetResponse');
    expect(document.components.schemas.CreateAssetResponse).toEqual({
      type: 'object',
      required: ['id'],
      properties: { id: { type: 'string' } },
    });
    expect(document.components.schemas.DeleteAssetResponse).toEqual({
      type: 'object',
      required: ['success'],
      properties: { success: { type: 'boolean', const: true } },
    });
  });

  test('uses the response builders for compatibility and OpenAI video routes', async () => {
    const workspace = await mkdtemp(join(tmpdir(), 'molii-owned-openapi-'));
    workspaces.push(workspace);
    const siteRoot = join(import.meta.dir, '..');
    const document = await prepareOpenApi({
      templatePath: join(siteRoot, 'openapi', 'relay.public.template.yaml'),
      allowlistPath: join(siteRoot, 'openapi', 'public-api-surface.json'),
      outputPath: join(workspace, 'relay.public.json'),
      apiBaseUrl: 'https://api.molii.example',
    });

    expect(document.paths['/v1/video/generations'].post.responses['200'].$ref)
      .toBe('#/components/responses/OpenAIVideoAccepted');
    expect(document.paths['/v1/videos'].post.responses['200'].$ref)
      .toBe('#/components/responses/OpenAIVideoAccepted');
    expect(document.paths['/v1/videos/edits'].post.responses['200'].$ref)
      .toBe('#/components/responses/OpenAIVideoAccepted');
    expect(document.paths['/v1/video/generations/{task_id}'].get.responses['200'].$ref)
      .toBe('#/components/responses/CompatibilityTaskResult');
    expect(document.paths['/v1/videos/{task_id}'].get.responses['200'].$ref)
      .toBe('#/components/responses/OpenAIVideoResult');

    expect(document.components.schemas.OpenAIVideo).toMatchObject({
      required: ['id', 'object', 'model', 'status', 'progress', 'created_at'],
      properties: {
        object: { type: 'string', const: 'video' },
        status: { type: 'string', enum: ['queued', 'in_progress', 'completed', 'failed'] },
      },
    });
    expect(document.components.schemas.CompatibilityTaskResponse).toMatchObject({
      required: ['code', 'message', 'data'],
      properties: { data: { $ref: '#/components/schemas/CompatibilityTask' } },
    });

    const compatibilityTask = document.components.schemas.CompatibilityTask;
    expect(compatibilityTask.required).toEqual(['task_id', 'status', 'progress']);
    expect(compatibilityTask.properties).toHaveProperty('result_url');
    expect(compatibilityTask.properties).toHaveProperty('fail_reason');
    expect(compatibilityTask.properties).toHaveProperty('billing');
    for (const internalField of ['id', 'platform', 'user_id', 'group', 'channel_id', 'quota']) {
      expect(compatibilityTask.properties).not.toHaveProperty(internalField);
    }

    const compatibilityExample = document.components.responses.CompatibilityTaskResult
      .content['application/json'].examples.completed.value.data;
    expect(compatibilityExample).toMatchObject({
      task_id: 'task_public_123',
      status: 'SUCCESS',
      progress: '100%',
      billing: { mode: 'grok_video' },
    });
    expect(document.components.schemas.PublicTaskBillingSummary.properties.mode.enum)
      .toEqual(['seedance', 'grok_video']);
    for (const internalField of ['id', 'platform', 'user_id', 'group', 'channel_id', 'quota']) {
      expect(compatibilityExample).not.toHaveProperty(internalField);
    }
  });

  test('points primary API reference navigation at the generated introduction', () => {
    const navbar = siteConfig.themeConfig?.navbar as { items?: Array<{ label?: string; to?: string }> };
    const footer = siteConfig.themeConfig?.footer as { links?: Array<{ items?: Array<{ label?: string; to?: string }> }> };

    expect(navbar.items?.find((item) => item.label === 'API 参考')?.to)
      .toBe('/api-reference/molii-public-api');
    expect(footer.links?.flatMap((group) => group.items ?? []).find((item) => item.label === 'API 参考')?.to)
      .toBe('/api-reference/molii-public-api');
  });
});
