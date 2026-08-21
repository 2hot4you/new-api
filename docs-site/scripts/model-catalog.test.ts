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
      { model_name: 'image-test', display_name: 'Image Test', description: 'Image endpoint', description_en: 'English image endpoint', release_date: '2026-08-07', vendor_id: 2, display_order: 1, supported_endpoint_types: ['image-generation'], input_modalities: ['text', 'image'], output_modalities: ['image'], capabilities: ['image_generation', 'image_editing'], supported_parameters: ['quality'], supported_resolutions: ['1k', '2k'], supported_aspect_ratios: ['1:1', '16:9'], output_formats: ['url', 'b64_json'], reference_modalities: ['image'], max_input_images: 3, upstream_model: 'private' },
      { model_name: 'video-test', display_name: 'Video Test', description: 'Video endpoint', release_date: '2026-08-08', vendor_id: 2, display_order: 2, supported_endpoint_types: ['openai-video'], input_modalities: ['text', 'image'], output_modalities: ['video'], capabilities: ['video_generation'], supported_parameters: ['duration'], supported_resolutions: ['720p'], supported_aspect_ratios: ['16:9'], output_formats: ['url'], reference_modalities: ['image', 'video'], max_input_images: 7, min_duration: 1, max_duration: 15, channel_id: 'private' },
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
        { id: 'image-test', display_name: 'Image Test', description: 'Image endpoint', vendor_id: 2, display_order: 1, supported_endpoint_types: ['image-generation'], description_en: 'English image endpoint', release_date: '2026-08-07', input_modalities: ['text', 'image'], output_modalities: ['image'], capabilities: ['image_generation', 'image_editing'], supported_parameters: ['quality'], supported_resolutions: ['1k', '2k'], supported_aspect_ratios: ['1:1', '16:9'], output_formats: ['url', 'b64_json'], reference_modalities: ['image'], max_input_images: 3 },
        { id: 'video-test', display_name: 'Video Test', description: 'Video endpoint', vendor_id: 2, display_order: 2, supported_endpoint_types: ['openai-video'], release_date: '2026-08-08', input_modalities: ['text', 'image'], output_modalities: ['video'], capabilities: ['video_generation'], supported_parameters: ['duration'], supported_resolutions: ['720p'], supported_aspect_ratios: ['16:9'], output_formats: ['url'], reference_modalities: ['image', 'video'], max_input_images: 7, min_duration: 1, max_duration: 15 },
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

    const multilineProvider = rawCatalog();
    multilineProvider.vendors[0].name = 'Injected\n---\nsidebar_position: 0';
    expect(() => sanitizeCatalogResponse(multilineProvider)).toThrow('control characters');

    const multilineDescription = rawCatalog();
    multilineDescription.data[0].description = 'Injected\n# Heading';
    expect(() => sanitizeCatalogResponse(multilineDescription)).toThrow('control characters');
  });

  test('orders Providers with tied positions by numeric id', () => {
    const raw = rawCatalog();
    raw.vendors[0].display_order = 1;

    expect(sanitizeCatalogResponse(raw).vendors.map((provider) => provider.id)).toEqual([1, 2]);
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
    expect(claude).toContain('--header "x-api-key: $MOLII_API_KEY"');
    expect(claude).toContain('--header "anthropic-version: 2023-06-01"');
    expect(claude).not.toContain('Authorization: Bearer');
    expect(gemini).toContain('POST /v1beta/models/gemini-test:generateContent');
    expect(gemini).toContain('--header "x-goog-api-key: $MOLII_API_KEY"');
    expect(gemini).not.toContain('Authorization: Bearer');
    expect(gemini).not.toContain('?key=');
    expect(grok).toContain('POST /v1/chat/completions');
    expect(grok).toContain('POST /v1/responses');
    expect(image).toContain('POST /v1/images/generations');
    expect(image).toContain('POST /v1/images/edits');
    expect(image).toContain('"images":[{"url":"https://example.invalid/source.png"}]');
    expect(image).toContain('- **发布日期：** 2026-08-07');
    expect(image).toContain('- **最大输入图片数：** 3');
    expect(image).toContain('- **支持分辨率：** 1k、2k');
    expect(image).toContain('- **支持宽高比：** 1\\:1、16\\:9');
    expect(image).toContain('- **输出格式：** url、b64\\_json');
    expect(image).toContain('- **参考输入模态：** image');
    expect(video).toContain('POST /v1/videos');
    expect(video).toContain('GET /v1/videos/\\{task_id\\}');
    expect(video).toContain('GET /v1/videos/\\{task_id\\}/content');
    expect(video.match(/--fail-with-body --silent --show-error/g)).toHaveLength(3);
    expect(video.match(/--max-time (30|60|300)/g)).toHaveLength(3);
    expect(video.match(/--max-redirs 0/g)).toHaveLength(2);
    expect(video).not.toContain('\n  --location');
    expect(video).toContain('--output ./video-content.bin \\\n  --header "Accept: video/*"');
    expect(video).toContain('- **视频时长范围：** 1–15 秒');
    expect(video).toContain('客户端。\n\n### OpenAI Video');
    expect(video).not.toContain('fetch(');
  });

  test('quotes frontmatter and escapes adversarial public strings in MDX and shell payloads', async () => {
    const outputRoot = await mkdtemp(join(tmpdir(), 'molii-catalog-adversarial-'));
    temporaryDirectories.push(outputRoot);
    const raw = rawCatalog();
    raw.vendors[1].name = 'Provider: "quoted" # <script>{x}</script>';
    raw.data[0].model_name = 'model\'"$()`{x}<tag>';
    raw.data[0].display_name = 'Title: "quoted" # <script>{x}</script>';
    raw.data[0].description = 'Description <img> {danger} # still text';
    const catalog = sanitizeCatalogResponse(raw);

    await generateCatalogDocs({ catalog, outputRoot });

    const providerSlug = 'provider-quoted-script-x-script';
    const modelSlug = 'model-x-tag';
    const page = await readFile(join(outputRoot, 'providers', providerSlug, `${modelSlug}.mdx`), 'utf8');
    const providerPage = await readFile(join(outputRoot, 'providers', providerSlug, 'index.mdx'), 'utf8');

    expect(page).toContain('title: "Title: \\"quoted\\" # <script>{x}</script>"');
    expect(page).toContain('# Title\\: \\"quoted\\" \\# \\<script\\>\\{x\\}\\</script\\>');
    expect(page).toContain('Description \\<img\\> \\{danger\\} \\# still text');
    expect(page).not.toContain('\n# still text');
    expect(page).toContain('\\"$()`{x}<tag>"');
    expect(page).toContain("'\"'\"'");
    expect(providerPage).toContain('title: "Provider: \\"quoted\\" # <script>{x}</script>"');
    expect(providerPage).toContain("``model'\"$()`{x}<tag>``");
  });
});
