import { describe, expect, test } from 'bun:test';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { prepareOpenApi } from './prepare-openapi.mjs';

const siteRoot = join(import.meta.dir, '..');

const referencePages = [
  'docs/api-reference/index.mdx',
  'docs/api-reference/models.mdx',
  'docs/api-reference/images.mdx',
  'docs/api-reference/videos.mdx',
  'docs/api-reference/files.mdx',
  'docs/api-reference/seedance.mdx',
  'docs/api-reference/assets.mdx',
  'docs/api-reference/errors.mdx',
  'docs/api-reference/chat-completions.mdx',
  'docs/api-reference/responses.mdx',
  'docs/api-reference/anthropic-messages.mdx',
  'docs/api-reference/gemini-generate-content.mdx',
];

const endpointPages = {
  'docs/api-reference/models.mdx': ['GET /v1/models'],
  'docs/api-reference/images.mdx': [
    'POST /v1/images/generations',
    'POST /v1/images/edits',
  ],
  'docs/api-reference/videos.mdx': [
    'POST /v1/videos',
    'POST /v1/videos/generations',
    'POST /v1/videos/edits',
    'POST /v1/videos/extensions',
    'GET /v1/videos/{task_id}',
    'GET /v1/videos/{task_id}/content',
  ],
  'docs/api-reference/files.mdx': [
    'GET /v1/files',
    'POST /v1/files',
    'GET /v1/files/{id}',
    'DELETE /v1/files/{id}',
    'GET /v1/files/{id}/content',
  ],
  'docs/api-reference/seedance.mdx': [
    'POST /v1/video/generations',
    'GET /v1/video/generations/{task_id}',
  ],
  'docs/api-reference/assets.mdx': [
    'POST /v1/assets',
    'GET /v1/assets/{id}',
    'DELETE /v1/assets/{id}',
  ],
  'docs/api-reference/chat-completions.mdx': ['POST /v1/chat/completions'],
  'docs/api-reference/responses.mdx': ['POST /v1/responses'],
  'docs/api-reference/anthropic-messages.mdx': ['POST /v1/messages'],
  'docs/api-reference/gemini-generate-content.mdx': ['POST /v1beta/models/{model}:generateContent'],
} as const;

const coreProtocolOperations = [
  {
    signature: 'POST /v1/chat/completions',
    operationId: 'createChatCompletion',
    schema: 'ChatCompletionRequest',
    security: [{ BearerAuth: [] }],
  },
  {
    signature: 'POST /v1/responses',
    operationId: 'createResponse',
    schema: 'ResponsesRequest',
    security: [{ BearerAuth: [] }],
  },
  {
    signature: 'POST /v1/messages',
    operationId: 'createAnthropicMessage',
    schema: 'AnthropicMessagesRequest',
    security: [{ AnthropicApiKey: [] }],
  },
  {
    signature: 'POST /v1beta/models/{model}:generateContent',
    operationId: 'generateGeminiContent',
    schema: 'GeminiGenerateContentRequest',
    security: [{ GeminiApiKey: [] }],
  },
] as const;

const coreProtocolSignatures = new Set(coreProtocolOperations.map(({ signature }) => signature));

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

function frontmatter(document: string) {
  const match = document.match(/^---\n([\s\S]*?)\n---/);
  return match?.[1] ?? '';
}

describe('ordinary MDX API reference contract', () => {
  test('publishes only the documented core LLM protocol operations and public schemas', async () => {
    const template = JSON.parse(await source('openapi/relay.public.template.yaml')) as any;
    const allowlist = JSON.parse(await source('openapi/public-api-surface.json')) as { operations: string[] };
    const workspace = await mkdtemp(join(tmpdir(), 'molii-core-protocol-openapi-'));

    try {
      const generated = await prepareOpenApi({
        templatePath: join(siteRoot, 'openapi/relay.public.template.yaml'),
        allowlistPath: join(siteRoot, 'openapi/public-api-surface.json'),
        outputPath: join(workspace, 'relay.public.json'),
        apiBaseUrl: 'https://api.molii.example',
      }) as any;
      const operationIds = Object.values(generated.paths).flatMap((pathItem: any) =>
        Object.values(pathItem as Record<string, any>).map((operation) => operation.operationId),
      );

      expect(new Set(operationIds).size).toBe(operationIds.length);
      expect(generated.components.securitySchemes).toEqual({
        BearerAuth: expect.any(Object),
        AnthropicApiKey: expect.any(Object),
        GeminiApiKey: expect.any(Object),
      });
      expect(Object.keys(generated.components.schemas).some((name) => /administrator/i.test(name))).toBe(false);

      for (const expected of coreProtocolOperations) {
        const [method, path] = expected.signature.split(' ');
        const templateOperation = template.paths[path][method.toLowerCase()];
        const generatedOperation = generated.paths[path][method.toLowerCase()];

        expect(allowlist.operations).toContain(expected.signature);
        expect(templateOperation.operationId).toBe(expected.operationId);
        expect(templateOperation.requestBody.content['application/json'].schema.$ref)
          .toBe(`#/components/schemas/${expected.schema}`);
        expect(templateOperation.responses).toMatchObject({
          200: expect.any(Object),
          400: expect.any(Object),
          401: expect.any(Object),
          429: expect.any(Object),
        });
        expect(generatedOperation).toMatchObject({
          operationId: expected.operationId,
          security: expected.security,
        });
      }
    } finally {
      await rm(workspace, { recursive: true, force: true });
    }
  });

  test('keeps the Seedance compatibility endpoint on its real multimodal request contract', async () => {
    const spec = JSON.parse(await source('openapi/relay.public.template.yaml')) as any;
    const operation = spec.paths['/v1/video/generations'].post;
    const schema = spec.components.schemas.SeedanceVideoGenerationRequest;

    expect(operation.requestBody.content['application/json'].schema.$ref)
      .toBe('#/components/schemas/SeedanceVideoGenerationRequest');
    expect(schema.properties.model.enum).toEqual([
      'doubao-seedance-2-0-260128',
      'doubao-seedance-2-0-fast-260128',
    ]);
    expect(schema.properties.content.items.$ref).toBe('#/components/schemas/SeedanceContentItem');
    expect(schema.properties.resolution.default).toBe('720p');
    expect(schema.properties.ratio.default).toBe('adaptive');
    expect(schema.properties.duration.default).toBe(5);
    expect(operation.requestBody.content['application/json'].examples.multimodal.value.content)
      .toBeArray();
  });

  test('publishes every public OpenAPI operation exactly once as a method-style MDX heading', async () => {
    const spec = JSON.parse(await source('openapi/relay.public.template.yaml')) as {
      paths: Record<string, Record<string, { operationId?: string }>>;
    };
    const documents = await Promise.all(Object.keys(endpointPages).map(source));

    for (const [path, pathItem] of Object.entries(spec.paths)) {
      for (const method of ['get', 'post', 'put', 'patch', 'delete']) {
        if (!pathItem[method]?.operationId) continue;
        const signature = `${method.toUpperCase()} ${path}`;
        const heading = `## \`${signature}\``;
        expect(documents.filter((document) => document.includes(heading)), signature).toHaveLength(1);
      }
    }
  });

  test('uses the same endpoint groupings as the public product surface', async () => {
    for (const [relativePath, signatures] of Object.entries(endpointPages)) {
      const document = await source(relativePath);
      for (const signature of signatures) {
        expect(document, relativePath).toContain(`## \`${signature}\``);
      }
    }
  });

  test('gives every endpoint a complete reference section', async () => {
    for (const [relativePath, signatures] of Object.entries(endpointPages)) {
      const document = await source(relativePath);
      for (const [index, signature] of signatures.entries()) {
        const start = document.indexOf(`## \`${signature}\``);
        const next = signatures[index + 1];
        const end = next ? document.indexOf(`## \`${next}\``) : document.length;
        const section = document.slice(start, end);

        expect(section, `${signature}: authentication`).toContain('### 鉴权');
        expect(section, `${signature}: parameters`).toContain('### 请求参数');
        expect(section, `${signature}: request`).toContain('### 请求示例');
        expect(section, `${signature}: response`).toContain('### 成功响应');
        expect(section, `${signature}: errors`).toContain('### 错误与重试');
        expect(section, `${signature}: billing`).toContain('### 计费');
      }
    }
  });

  test('uses only stock Markdown and Docusaurus admonitions', async () => {
    for (const relativePath of referencePages) {
      const document = await source(relativePath);
      expect(frontmatter(document), relativePath).toContain('audience: user');
      expect(frontmatter(document), relativePath).toContain('apiVersion: v1');
      expect(document, relativePath).not.toMatch(/@theme\/Api|ApiExplorer|ApiItem|className=|<style/i);
      expect(document, relativePath).not.toMatch(/\/api\/(?:channel|models\/sync|assets\/admin|user\/manage)/);
      expect(document, relativePath).not.toMatch(/\bsk-[A-Za-z0-9_-]{16,}\b/);
    }
  });

  test('gives each core protocol page safe request and response guidance', async () => {
    for (const [relativePath, signatures] of Object.entries(endpointPages)) {
      if (signatures.length !== 1 || !coreProtocolSignatures.has(signatures[0])) continue;
      const document = await source(relativePath);
      const signature = signatures[0];

      expect(document).toContain(`## \`${signature}\``);
      expect(document).toContain('### 请求参数');
      expect(document).toContain('### 请求示例');
      expect(document).toContain('### 成功响应');
      expect(document).toContain('[身份验证](/api-basics/authentication)');
      expect(document).toContain('[错误与重试](/api-basics/errors-retries)');
      expect(document).toMatch(/curl[\s\S]*?\$MOLII_API_KEY/);
      expect(document).not.toMatch(/[?&](?:key|api_key|x-goog-api-key)=/i);
    }
  });

  test('API overview links every default MDX reference group below /api-reference', async () => {
    const index = await source('docs/api-reference/index.mdx');
    for (const [label, route] of [
      ['模型', '/api-reference/models'],
      ['图片', '/api-reference/images'],
      ['视频', '/api-reference/videos'],
      ['文件', '/api-reference/files'],
      ['Seedance', '/api-reference/seedance'],
      ['临时素材', '/api-reference/assets'],
      ['错误', '/api-reference/errors'],
    ]) {
      expect(index).toContain(`[${label}](${route})`);
    }
  });

  test('API overview links developers to the concrete Base URL and authentication contracts', async () => {
    const index = await source('docs/api-reference/index.mdx');

    expect(index).toContain('[Base URL](/api-basics/base-url)');
    expect(index).toContain('[身份验证](/api-basics/authentication)');
  });
});
