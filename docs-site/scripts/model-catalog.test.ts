import { afterEach, describe, expect, test } from 'bun:test';
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { sanitizeCatalogResponse } from './model-catalog.mjs';
import { generateCatalogDocs } from './generate-model-docs.mjs';

const temporaryDirectories: string[] = [];

afterEach(async () => {
  await Promise.all(temporaryDirectories.splice(0).map((directory) => rm(directory, { recursive: true, force: true })));
});

function rawCatalog() {
  return {
    success: true,
    pricing_version: 'snapshot-v1',
    auto_groups: ['internal-only'],
    vendors: [
      { id: 2, name: 'Second Provider', display_order: 2, description: '第二个 Provider', icon: 'Hidden.Icon' },
      { id: 1, name: 'First Provider', display_order: 1, description: '第一个 Provider', icon: 'Hidden.Icon' },
    ],
    data: [
      { model_name: 'claude-test', display_name: 'Claude Test', description: 'Claude endpoint', vendor_id: 1, display_order: 1, supported_endpoint_types: ['anthropic'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['tools'], supported_parameters: ['max_tokens'], enable_groups: ['internal-only'], model_ratio: 5 },
      { model_name: 'gemini-test', display_name: 'Gemini Test', description: 'Gemini endpoint', vendor_id: 1, display_order: 2, supported_endpoint_types: ['gemini'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['vision'], supported_parameters: ['temperature'], group_ratio: { internal: 1 } },
      { model_name: 'grok-test', display_name: 'Grok Test', description: 'OpenAI-compatible endpoints', vendor_id: 1, display_order: 3, supported_endpoint_types: ['openai', 'openai-response'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['reasoning'], supported_parameters: ['stream'], billing_expr: 'private' },
      { model_name: 'image-test', display_name: 'Image Test', description: 'Image endpoint', vendor_id: 2, display_order: 1, supported_endpoint_types: ['image-generation'], input_modalities: ['text', 'image'], output_modalities: ['image'], capabilities: ['image_generation', 'image_editing'], supported_parameters: ['quality'], max_input_images: 3, upstream_model: 'private' },
      { model_name: 'video-test', display_name: 'Video Test', description: 'Video endpoint', vendor_id: 2, display_order: 2, supported_endpoint_types: ['openai-video'], input_modalities: ['text', 'image'], output_modalities: ['video'], capabilities: ['video_generation'], supported_parameters: ['duration'], min_duration: 1, max_duration: 15, channel_id: 'private' },
    ],
  };
}

describe('model catalog', () => {
  test('sanitizes only public fields and orders providers and models by display order', () => {
    const catalog = sanitizeCatalogResponse(rawCatalog());

    expect(catalog).toEqual({
      source: 'https://dev.molii.co/api/pricing',
      pricing_version: 'snapshot-v1',
      vendors: [
        { id: 1, name: 'First Provider', display_order: 1, description: '第一个 Provider' },
        { id: 2, name: 'Second Provider', display_order: 2, description: '第二个 Provider' },
      ],
      models: [
        { id: 'claude-test', display_name: 'Claude Test', description: 'Claude endpoint', vendor_id: 1, display_order: 1, supported_endpoint_types: ['anthropic'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['tools'], supported_parameters: ['max_tokens'] },
        { id: 'gemini-test', display_name: 'Gemini Test', description: 'Gemini endpoint', vendor_id: 1, display_order: 2, supported_endpoint_types: ['gemini'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['vision'], supported_parameters: ['temperature'] },
        { id: 'grok-test', display_name: 'Grok Test', description: 'OpenAI-compatible endpoints', vendor_id: 1, display_order: 3, supported_endpoint_types: ['openai', 'openai-response'], input_modalities: ['text'], output_modalities: ['text'], capabilities: ['reasoning'], supported_parameters: ['stream'] },
        { id: 'image-test', display_name: 'Image Test', description: 'Image endpoint', vendor_id: 2, display_order: 1, supported_endpoint_types: ['image-generation'], input_modalities: ['text', 'image'], output_modalities: ['image'], capabilities: ['image_generation', 'image_editing'], supported_parameters: ['quality'], max_input_images: 3 },
        { id: 'video-test', display_name: 'Video Test', description: 'Video endpoint', vendor_id: 2, display_order: 2, supported_endpoint_types: ['openai-video'], input_modalities: ['text', 'image'], output_modalities: ['video'], capabilities: ['video_generation'], supported_parameters: ['duration'], min_duration: 1, max_duration: 15 },
      ],
    });
  });

  test('rejects malformed, unknown endpoint, duplicate model slug, and unknown provider input', () => {
    expect(() => sanitizeCatalogResponse({ success: false })).toThrow('success');

    const unknownEndpoint = rawCatalog();
    unknownEndpoint.data[0].supported_endpoint_types = ['unknown'];
    expect(() => sanitizeCatalogResponse(unknownEndpoint)).toThrow('Unknown endpoint type');

    const duplicateModel = rawCatalog();
    duplicateModel.data[1].model_name = 'claude-test';
    expect(() => sanitizeCatalogResponse(duplicateModel)).toThrow('Duplicate model slug');

    const unknownProvider = rawCatalog();
    unknownProvider.data[0].vendor_id = 99;
    expect(() => sanitizeCatalogResponse(unknownProvider)).toThrow('unknown Provider');
  });

  test('generates a complete deterministic provider tree with public protocol routes and no network dependency', async () => {
    const outputRoot = await mkdtemp(join(tmpdir(), 'molii-catalog-'));
    temporaryDirectories.push(outputRoot);
    const catalog = sanitizeCatalogResponse(rawCatalog());

    const first = await generateCatalogDocs({ catalog, outputRoot });
    const providersRoot = join(outputRoot, 'providers');
    const second = await generateCatalogDocs({ catalog, outputRoot });

    expect(first).toEqual({ providerCount: 2, modelCount: 5, fileCount: 11 });
    expect(second).toEqual(first);
    expect((await readdir(providersRoot)).sort()).toEqual(['_category_.json', 'first-provider', 'index.mdx', 'second-provider']);
    expect(await readFile(join(providersRoot, '_category_.json'), 'utf8')).toContain('"position": 1');
    expect(await readFile(join(providersRoot, 'first-provider', '_category_.json'), 'utf8')).toContain('"position": 1');
    expect(await readFile(join(providersRoot, 'second-provider', '_category_.json'), 'utf8')).toContain('"position": 2');

    const claude = await readFile(join(providersRoot, 'first-provider', 'claude-test.mdx'), 'utf8');
    const gemini = await readFile(join(providersRoot, 'first-provider', 'gemini-test.mdx'), 'utf8');
    const grok = await readFile(join(providersRoot, 'first-provider', 'grok-test.mdx'), 'utf8');
    const image = await readFile(join(providersRoot, 'second-provider', 'image-test.mdx'), 'utf8');
    const video = await readFile(join(providersRoot, 'second-provider', 'video-test.mdx'), 'utf8');

    expect(claude).toContain('POST /v1/messages');
    expect(gemini).toContain('POST /v1beta/models/gemini-test:generateContent');
    expect(grok).toContain('POST /v1/chat/completions');
    expect(grok).toContain('POST /v1/responses');
    expect(image).toContain('POST /v1/images/generations');
    expect(image).toContain('POST /v1/images/edits');
    expect(video).toContain('POST /v1/videos');
    expect(video).toContain('GET /v1/videos/\\{task_id\\}');
    expect(video).toContain('GET /v1/videos/\\{task_id\\}/content');
    expect(video).not.toContain('fetch(');
  });
});
