import { mkdir, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { modelSlug, slugify } from './model-catalog.mjs';

const defaultOutputRoot = resolve(import.meta.dirname, '../docs');
const defaultCatalogPath = resolve(import.meta.dirname, '../data/development-model-catalog.json');

function oneLine(value) {
  return String(value).replace(/[\u0000-\u001f\u007f-\u009f\u2028\u2029]+/gu, ' ');
}

function escapeMdx(value) {
  return oneLine(value).replace(/[\\`*_{}\[\]<>()#+!|:"'$&?,;=@%^~]/g, '\\$&');
}

function yamlScalar(value) {
  return JSON.stringify(oneLine(value));
}

function inlineCode(value) {
  const text = oneLine(value);
  const longestRun = Math.max(0, ...[...text.matchAll(/`+/g)].map((match) => match[0].length));
  const fence = '`'.repeat(longestRun + 1);
  const padding = text.startsWith('`') || text.endsWith('`') ? ' ' : '';
  return `${fence}${padding}${text}${padding}${fence}`;
}

function shellSingleQuote(value) {
  return `'${String(value).replaceAll("'", `'"'"'`)}'`;
}

function pathSegment(value) {
  return encodeURIComponent(String(value)).replace(/[!'()*]/g, (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`);
}

function markdownList(values) {
  return values.length ? values.map((value) => `- ${escapeMdx(value)}`).join('\n') : '- 未声明';
}

function category(label, position) {
  return `${JSON.stringify({
    label,
    position,
    collapsible: true,
    collapsed: true,
  }, null, 2)}\n`;
}

const base = '${MOLII_API_BASE_URL%/}';
const endpointReferences = {
  openai: '/api-reference/chat-completions',
  'openai-response': '/api-reference/responses',
  anthropic: '/api-reference/anthropic-messages',
  gemini: '/api-reference/gemini-generate-content',
  'image-generation': '/api-reference/images',
  'openai-video': '/api-reference/videos',
};

const seedanceSupportedParameters = [
  'model',
  'content',
  'prompt',
  'generate_audio',
  'resolution',
  'ratio',
  'duration',
  'watermark',
  'tools',
];

function isSeedanceModel(model) {
  return model.id.startsWith('doubao-seedance');
}

function supportedParameters(model) {
  if (model.supported_parameters?.length) return model.supported_parameters;
  return isSeedanceModel(model) ? seedanceSupportedParameters : [];
}

function endpointDeclaration(method, path, type) {
  return `**[${method} ${path}](${endpointReferences[type]})**`;
}

function curlSetup() {
  return [
    ': "${MOLII_API_BASE_URL:?Set MOLII_API_BASE_URL first}"',
    ': "${MOLII_API_KEY:?Set MOLII_API_KEY first}"',
    '',
  ];
}

function postCurl(path, payload, headers) {
  return [
    '```bash',
    ...curlSetup(),
    'curl --fail-with-body --silent --show-error \\',
    '  --max-time 60 \\',
    '  --request POST \\',
    `  --url "${base}${path}" \\`,
    ...headers.map((header) => `  --header "${header}" \\`),
    '  --header "Content-Type: application/json" \\',
    `  --data ${shellSingleQuote(JSON.stringify(payload))}`,
    '```',
  ].join('\n');
}

function getCurl(path, maxTime, { accept = 'application/json', extraOptions = [] } = {}) {
  return [
    '```bash',
    ...curlSetup(),
    'curl --fail-with-body --silent --show-error \\',
    `  --max-time ${maxTime} \\`,
    '  --max-redirs 0 \\',
    '  --request GET \\',
    `  --url "${base}${path}" \\`,
    '  --header "Authorization: Bearer $MOLII_API_KEY" \\',
    ...extraOptions,
    `  --header "Accept: ${accept}"`,
    '```',
  ].join('\n');
}

function endpointSections(model) {
  const id = escapeMdx(model.id);
  const encodedId = pathSegment(model.id);
  const bearerHeaders = ['Authorization: Bearer $MOLII_API_KEY'];
  const sections = [];
  for (const type of model.supported_endpoint_types) {
    if (type === 'openai') {
      sections.push(`### OpenAI Chat Completions\n\n${endpointDeclaration('POST', '/v1/chat/completions', type)}\n\n${postCurl('/v1/chat/completions', { model: model.id, messages: [{ role: 'user', content: '你好' }] }, bearerHeaders)}`);
    } else if (type === 'openai-response') {
      sections.push(`### OpenAI Responses\n\n${endpointDeclaration('POST', '/v1/responses', type)}\n\n${postCurl('/v1/responses', { model: model.id, input: '你好' }, bearerHeaders)}`);
    } else if (type === 'anthropic') {
      sections.push(`### Anthropic Messages\n\n${endpointDeclaration('POST', '/v1/messages', type)}\n\n${postCurl('/v1/messages', { model: model.id, max_tokens: 256, messages: [{ role: 'user', content: '你好' }] }, ['x-api-key: $MOLII_API_KEY', 'anthropic-version: 2023-06-01'])}`);
    } else if (type === 'gemini') {
      sections.push(`### Gemini GenerateContent\n\n${endpointDeclaration('POST', `/v1beta/models/${id}:generateContent`, type)}\n\n${postCurl(`/v1beta/models/${encodedId}:generateContent`, { contents: [{ parts: [{ text: '你好' }] }] }, ['x-goog-api-key: $MOLII_API_KEY'])}`);
    } else if (type === 'image-generation') {
      const edits = (model.capabilities ?? []).includes('image_editing')
        ? `\n\n具备图片编辑能力时，也可调用 ${endpointDeclaration('POST', '/v1/images/edits', type)}。\n\n${postCurl('/v1/images/edits', { model: model.id, prompt: '描述需要的编辑', images: [{ url: 'https://example.invalid/source.png' }] }, bearerHeaders)}`
        : '';
      sections.push(`### Images\n\n${endpointDeclaration('POST', '/v1/images/generations', type)}\n\n${postCurl('/v1/images/generations', { model: model.id, prompt: '描述所需图片' }, bearerHeaders)}${edits}`);
    } else if (type === 'openai-video') {
      if (isSeedanceModel(model)) {
        const status = getCurl('/v1/videos/$TASK_ID', 30);
        const content = getCurl('/v1/videos/$TASK_ID/content?download=1', 300, {
          accept: 'video/*',
          extraOptions: [
            '  --dump-header ./seedance-content.headers \\',
            '  --output ./seedance-result.mp4 \\',
          ],
        });
        const payload = {
          model: model.id,
          content: [
            { type: 'text', text: '镜头缓慢向前推进，晨雾从山谷间流过' },
            { type: 'image_url', image_url: { url: 'https://example.invalid/first-frame.jpg' }, role: 'first_frame' },
          ],
          generate_audio: true,
          resolution: '720p',
          ratio: '16:9',
          duration: 5,
          watermark: false,
        };
        sections.push(`### Seedance Video

${endpointDeclaration('POST', '/v1/videos', type)}

${postCurl('/v1/videos', payload, bearerHeaders)}

创建视频是异步操作。付费 POST 只提交一次，并保存响应中的公共任务 ID：

\`\`\`json
{
  "id": "task_public_123",
  "object": "video",
  "model": "${model.id}",
  "status": "queued",
  "progress": 0
}
\`\`\`

\`\`\`bash
TASK_ID='task_public_123'
\`\`\`

${endpointDeclaration('GET', '/v1/videos/\\{task_id\\}', type)}

${status}

状态为 \`queued\` 或 \`in_progress\` 时继续有限轮询；\`completed\` 与 \`failed\` 为终态。

${endpointDeclaration('GET', '/v1/videos/\\{task_id\\}/content', type)}

${content}

内容请求故意不使用 \`--location\`，不会把 Molii 凭据转发给其他主机。\`download=1\` 让 Molii 返回附件下载响应。

完整字段、组合限制和更多示例见 [Seedance API](/api-reference/seedance)、[多模态输入指南](/guides/seedance-multimodal)、[临时素材 API](/api-reference/assets) 与 [curl 完整流程](/examples/seedance-curl)。创建请求可能产生费用；网络结果不确定时不要重复提交 POST。`);
        continue;
      }
      const status = getCurl('/v1/videos/$TASK_ID', 30);
      const content = getCurl('/v1/videos/$TASK_ID/content', 300, {
        accept: 'video/*',
        extraOptions: [
          '  --dump-header ./video-content.headers \\',
          '  --output ./video-content.bin \\',
        ],
      });
      sections.push(`### OpenAI Video\n\n${endpointDeclaration('POST', '/v1/videos', type)}\n\n${postCurl('/v1/videos', { model: model.id, prompt: '描述所需视频' }, bearerHeaders)}\n\n创建视频是异步操作。保存创建响应中的任务 ID；下面的 GET 请求有单次超时且不自动重试。\n\n\`\`\`bash\nTASK_ID='task_public_123'\n\`\`\`\n\n${endpointDeclaration('GET', '/v1/videos/\\{task_id\\}', type)}\n\n${status}\n\n${endpointDeclaration('GET', '/v1/videos/\\{task_id\\}/content', type)}\n\n${content}\n\n内容请求故意不使用 \`--location\`，不会把 Molii 凭据转发给重定向主机。检查响应头和内容类型后再保存结果，详见[视频 API](/api-reference/videos)。创建请求可能产生费用；网络结果不确定时不要重复提交 POST。`);
    } else {
      throw new Error(`Unknown endpoint type: ${type}`);
    }
  }
  return sections.join('\n\n');
}

function modelPage(model, provider) {
  const title = escapeMdx(model.display_name);
  const details = [
    ['模型 ID', inlineCode(model.id)],
    ['Provider', escapeMdx(provider.name)],
    ['输入模态', (model.input_modalities ?? []).map(escapeMdx).join('、') || '未声明'],
    ['输出模态', (model.output_modalities ?? []).map(escapeMdx).join('、') || '未声明'],
  ];
  if (model.description_en) details.push(['英文简介', escapeMdx(model.description_en)]);
  if (model.release_date) details.push(['发布日期', escapeMdx(model.release_date)]);
  if (model.knowledge_cutoff) details.push(['知识截止', escapeMdx(model.knowledge_cutoff)]);
  if (model.context_length) details.push(['上下文 Token 上限', String(model.context_length)]);
  if (model.max_output_tokens) details.push(['最大输出 Token', String(model.max_output_tokens)]);
  if (model.max_input_images) details.push(['最大输入图片数', String(model.max_input_images)]);
  if (model.min_duration || model.max_duration) details.push(['视频时长范围', `${model.min_duration ?? '未声明'}–${model.max_duration ?? '未声明'} 秒`]);
  if (model.supported_resolutions?.length) details.push(['支持分辨率', model.supported_resolutions.map(escapeMdx).join('、')]);
  if (model.supported_aspect_ratios?.length) details.push(['支持宽高比', model.supported_aspect_ratios.map(escapeMdx).join('、')]);
  if (model.output_formats?.length) details.push(['输出格式', model.output_formats.map(escapeMdx).join('、')]);
  if (model.reference_modalities?.length) details.push(['参考输入模态', model.reference_modalities.map(escapeMdx).join('、')]);
  const guide = model.id.startsWith('grok-imagine-image')
    ? '\n\n延伸阅读：[Grok Imagine 图片深度指南](/models/grok-imagine-image)。'
    : model.id.startsWith('grok-imagine-video')
      ? '\n\n延伸阅读：[Grok Imagine 视频深度指南](/models/grok-imagine-video)。'
      : model.id.startsWith('doubao-seedance')
        ? '\n\n延伸阅读：[Seedance 2.0 深度指南](/models/seedance-2)。'
        : '';
  return `---\ntitle: ${yamlScalar(model.display_name)}\nsidebar_position: ${model.display_order}\n---\n\n# ${title}\n\n${escapeMdx(model.description || model.description_en || '该模型的公开调用说明。')}\n\n${details.map(([name, value]) => `- **${name}：** ${value}`).join('\n')}\n\n## 能力\n\n${markdownList(model.capabilities ?? [])}\n\n## 支持参数\n\n${markdownList(supportedParameters(model))}\n\n## 兼容协议与安全调用\n\n使用环境变量 \`MOLII_API_KEY\`，不要把密钥写入代码或客户端。\n\n${endpointSections(model)}\n\n## 调用提示\n\n同步接口在返回响应后完成。异步接口请保留任务 ID 并轮询结果；网络超时不表示请求未受理，提交前先查询任务，避免重复付费请求。\n\n通用接口参考：[模型列表](/api-reference/models)、[认证](/api-basics/authentication)。${guide}\n`;
}

function providerIndex(provider, models) {
  const rows = models.map((model) => `- [${escapeMdx(model.display_name)}](./${modelSlug(model.id)}) — ${inlineCode(model.id)}`).join('\n');
  const guide = provider.name === 'ByteDance'
    ? '\n\n## 接入指南\n\nSeedance 模型统一使用异步视频接口。先阅读 [Seedance API](/api-reference/seedance)，再按场景查看[多模态输入](/guides/seedance-multimodal)、[临时素材](/guides/temporary-assets)和[curl 完整示例](/examples/seedance-curl)。'
    : '';
  return `---\ntitle: ${yamlScalar(provider.name)}\nsidebar_position: 1\n---\n\n# ${escapeMdx(provider.name)}\n\n${escapeMdx(provider.description || '该 Provider 的公开模型目录。')}\n\n## 模型\n\n${rows || '暂无公开模型。'}${guide}\n`;
}

async function writeGeneratedTree(catalog, temporaryRoot) {
  await writeFile(resolve(temporaryRoot, '_category_.json'), category('Provider 与模型', 1));
  for (const provider of catalog.vendors) {
    const providerDirectory = resolve(temporaryRoot, slugify(provider.name));
    const providerModels = catalog.models.filter((model) => model.vendor_id === provider.id);
    await mkdir(providerDirectory, { recursive: true });
    await writeFile(resolve(providerDirectory, '_category_.json'), category(provider.name, provider.display_order));
    await writeFile(resolve(providerDirectory, 'index.mdx'), providerIndex(provider, providerModels));
    for (const model of providerModels) {
      await writeFile(resolve(providerDirectory, `${modelSlug(model.id)}.mdx`), modelPage(model, provider));
    }
  }
}

export async function generateCatalogDocs({ catalog, outputRoot = defaultOutputRoot }) {
  if (!catalog || !Array.isArray(catalog.vendors) || !Array.isArray(catalog.models)) throw new Error('Invalid catalog snapshot');
  for (const model of catalog.models) modelSlug(model.id);
  const root = resolve(outputRoot);
  const target = resolve(root, 'providers');
  const temporary = resolve(root, `.providers-generated-${process.pid}`);
  await rm(temporary, { recursive: true, force: true });
  await mkdir(temporary, { recursive: true });
  try {
    await writeGeneratedTree(catalog, temporary);
    await rm(target, { recursive: true, force: true });
    await rename(temporary, target);
  } catch (error) {
    await rm(temporary, { recursive: true, force: true });
    throw error;
  }
  return { providerCount: catalog.vendors.length, modelCount: catalog.models.length, fileCount: 1 + catalog.vendors.length * 2 + catalog.models.length };
}

async function main() {
  const check = process.argv.includes('--check');
  const catalog = JSON.parse(await readFile(defaultCatalogPath, 'utf8'));
  if (check) {
    const existing = await mkdtempForCheck();
    try {
      await generateCatalogDocs({ catalog, outputRoot: existing });
      const actual = await serializeTree(resolve(defaultOutputRoot, 'providers'));
      const generated = await serializeTree(resolve(existing, 'providers'));
      if (actual !== generated) throw new Error('Generated Provider docs are out of date. Run bun run catalog:generate.');
      console.log('Provider/model docs match the catalog snapshot.');
    } finally {
      await rm(existing, { recursive: true, force: true });
    }
  } else {
    const result = await generateCatalogDocs({ catalog, outputRoot: defaultOutputRoot });
    console.log(`Generated ${result.providerCount} Providers, ${result.modelCount} models, and ${result.fileCount} files.`);
  }
}

async function mkdtempForCheck() {
  const temporary = resolve(defaultOutputRoot, `.providers-check-${process.pid}`);
  await rm(temporary, { recursive: true, force: true });
  await mkdir(temporary, { recursive: true });
  return temporary;
}

async function serializeTree(root) {
  const names = await readdir(root, { withFileTypes: true });
  const entries = [];
  for (const entry of names.sort((left, right) => left.name.localeCompare(right.name))) {
    const path = resolve(root, entry.name);
    if (entry.isDirectory()) entries.push(`${entry.name}/\n${await serializeTree(path)}`);
    else entries.push(`${entry.name}\n${await readFile(path, 'utf8')}`);
  }
  return entries.join('\n');
}

if (import.meta.main) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  });
}
