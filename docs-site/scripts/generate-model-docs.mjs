import { mkdir, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

import { slugify } from './model-catalog.mjs';

const defaultOutputRoot = resolve(import.meta.dirname, '../docs');
const defaultCatalogPath = resolve(import.meta.dirname, '../data/development-model-catalog.json');

function escapeMdx(value) {
  return String(value).replace(/[\\`*_{}\[\]<>#|]/g, '\\$&');
}

function markdownList(values) {
  return values.length ? values.map((value) => `- ${escapeMdx(value)}`).join('\n') : '- 未声明';
}

function category(label, position) {
  return `${JSON.stringify({ label, position, collapsible: true, collapsed: true }, null, 2)}\n`;
}

function endpointSections(model) {
  const id = escapeMdx(model.id);
  const base = '${MOLII_BASE_URL:-https://api.molii.co}';
  const curl = (path, payload) => [
    '```bash',
    `curl -X POST "${base}${path}" \\`,
    '  -H "Authorization: Bearer $MOLII_API_KEY" \\',
    '  -H "Content-Type: application/json" \\',
    `  -d '${payload}'`,
    '```',
  ].join('\n');
  const sections = [];
  for (const type of model.supported_endpoint_types) {
    if (type === 'openai') {
      sections.push(`### OpenAI Chat Completions\n\n**POST /v1/chat/completions**\n\n${curl('/v1/chat/completions', `{"model":"${id}","messages":[{"role":"user","content":"你好"}]}`)}`);
    } else if (type === 'openai-response') {
      sections.push(`### OpenAI Responses\n\n**POST /v1/responses**\n\n${curl('/v1/responses', `{"model":"${id}","input":"你好"}`)}`);
    } else if (type === 'anthropic') {
      sections.push(`### Anthropic Messages\n\n**POST /v1/messages**\n\n${curl('/v1/messages', `{"model":"${id}","max_tokens":256,"messages":[{"role":"user","content":"你好"}]}`)}`);
    } else if (type === 'gemini') {
      sections.push(`### Gemini GenerateContent\n\n**POST /v1beta/models/${id}:generateContent**\n\n${curl(`/v1beta/models/${id}:generateContent`, '{"contents":[{"parts":[{"text":"你好"}]}]}')}`);
    } else if (type === 'image-generation') {
      const edits = (model.capabilities ?? []).includes('image_editing')
        ? `\n\n具备图片编辑能力时，也可调用 **POST /v1/images/edits**。\n\n${curl('/v1/images/edits', `{"model":"${id}","prompt":"描述需要的编辑"}`)}`
        : '';
      sections.push(`### Images\n\n**POST /v1/images/generations**\n\n${curl('/v1/images/generations', `{"model":"${id}","prompt":"描述所需图片"}`)}${edits}`);
    } else if (type === 'openai-video') {
      sections.push(`### OpenAI Video\n\n**POST /v1/videos**\n\n${curl('/v1/videos', `{"model":"${id}","prompt":"描述所需视频"}`)}\n\n创建视频是异步操作。使用返回的任务 ID 轮询 **GET /v1/videos/\\{task_id\\}**，完成后从 **GET /v1/videos/\\{task_id\\}/content** 下载结果。创建请求可能产生费用；保存任务 ID，避免因超时而重复提交。`);
    } else {
      throw new Error(`Unknown endpoint type: ${type}`);
    }
  }
  return sections.join('\n\n');
}

function modelPage(model, provider) {
  const title = escapeMdx(model.display_name);
  const details = [
    ['模型 ID', `\`${escapeMdx(model.id)}\``],
    ['Provider', escapeMdx(provider.name)],
    ['输入模态', (model.input_modalities ?? []).map(escapeMdx).join('、') || '未声明'],
    ['输出模态', (model.output_modalities ?? []).map(escapeMdx).join('、') || '未声明'],
  ];
  if (model.context_length) details.push(['上下文长度', String(model.context_length)]);
  if (model.max_output_tokens) details.push(['最大输出 Token', String(model.max_output_tokens)]);
  if (model.min_duration || model.max_duration) details.push(['时长限制', `${model.min_duration ?? '未声明'}–${model.max_duration ?? '未声明'} 秒`]);
  const guide = model.id.startsWith('grok-imagine-image')
    ? '\n\n延伸阅读：[Grok Imagine 图片深度指南](/models/grok-imagine-image)。'
    : model.id.startsWith('grok-imagine-video')
      ? '\n\n延伸阅读：[Grok Imagine 视频深度指南](/models/grok-imagine-video)。'
      : model.id.startsWith('doubao-seedance')
        ? '\n\n延伸阅读：[Seedance 2.0 深度指南](/models/seedance-2)。'
        : '';
  return `---\ntitle: ${title}\nsidebar_position: ${model.display_order}\n---\n\n# ${title}\n\n${escapeMdx(model.description || model.description_en || '该模型的公开调用说明。')}\n\n${details.map(([name, value]) => `- **${name}：** ${value}`).join('\n')}\n\n## 能力\n\n${markdownList(model.capabilities ?? [])}\n\n## 支持参数\n\n${markdownList(model.supported_parameters ?? [])}\n\n## 兼容协议与安全调用\n\n使用环境变量 \`MOLII_API_KEY\`，不要把密钥写入代码或客户端。${endpointSections(model)}\n\n## 调用提示\n\n同步接口在返回响应后完成。异步接口请保留任务 ID 并轮询结果；网络超时不表示请求未受理，提交前先查询任务，避免重复付费请求。\n\n通用接口参考：[模型列表](/api-reference/models)、[认证](/api-basics/authentication)。${guide}\n`;
}

function providerIndex(provider, models) {
  const rows = models.map((model) => `- [${escapeMdx(model.display_name)}](./${slugify(model.id)}) — \`${escapeMdx(model.id)}\``).join('\n');
  return `---\ntitle: ${escapeMdx(provider.name)}\nsidebar_position: 1\n---\n\n# ${escapeMdx(provider.name)}\n\n${escapeMdx(provider.description || '该 Provider 的公开模型目录。')}\n\n## 模型\n\n${rows || '暂无公开模型。'}\n`;
}

function providersIndex(catalog) {
  const rows = catalog.vendors.map((provider) => `- [${escapeMdx(provider.name)}](./${slugify(provider.name)})`).join('\n');
  return `---\ntitle: Provider 与模型\nsidebar_position: 1\n---\n\n# Provider 与模型\n\n本目录由公开 Development 模型快照生成，按 Provider 展示可调用模型与兼容协议。\n\n${rows}\n`;
}

async function writeGeneratedTree(catalog, temporaryRoot) {
  await writeFile(resolve(temporaryRoot, '_category_.json'), category('Provider 与模型', 1));
  await writeFile(resolve(temporaryRoot, 'index.mdx'), providersIndex(catalog));
  for (const provider of catalog.vendors) {
    const providerDirectory = resolve(temporaryRoot, slugify(provider.name));
    const providerModels = catalog.models.filter((model) => model.vendor_id === provider.id);
    await mkdir(providerDirectory, { recursive: true });
    await writeFile(resolve(providerDirectory, '_category_.json'), category(provider.name, provider.display_order));
    await writeFile(resolve(providerDirectory, 'index.mdx'), providerIndex(provider, providerModels));
    for (const model of providerModels) {
      await writeFile(resolve(providerDirectory, `${slugify(model.id)}.mdx`), modelPage(model, provider));
    }
  }
}

export async function generateCatalogDocs({ catalog, outputRoot = defaultOutputRoot }) {
  if (!catalog || !Array.isArray(catalog.vendors) || !Array.isArray(catalog.models)) throw new Error('Invalid catalog snapshot');
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
  return { providerCount: catalog.vendors.length, modelCount: catalog.models.length, fileCount: 2 + catalog.vendors.length * 2 + catalog.models.length };
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
