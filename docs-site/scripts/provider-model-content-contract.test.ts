import { describe, expect, test } from 'bun:test';
import { readdir, readFile } from 'node:fs/promises';
import { join, relative } from 'node:path';

import sidebars from '../sidebars';

type CatalogVendor = {
  id: number;
  name: string;
  display_order: number;
};

type CatalogModel = {
  id: string;
  vendor_id: number;
  display_order: number;
  supported_endpoint_types: string[];
};

type Catalog = {
  source: string;
  pricing_version: string;
  vendors: CatalogVendor[];
  models: CatalogModel[];
};

type SidebarItem = {
  type?: string;
  label?: string;
  dirName?: string;
  link?: {
    type?: string;
    slug?: string;
    description?: string;
  };
  items?: Array<SidebarItem | string>;
};

const siteRoot = join(import.meta.dir, '..');
const docsRoot = join(siteRoot, 'docs');
const providersRoot = join(docsRoot, 'providers');

const endpointReferences: Record<string, string> = {
  openai: '/api-reference/chat-completions',
  'openai-response': '/api-reference/responses',
  anthropic: '/api-reference/anthropic-messages',
  gemini: '/api-reference/gemini-generate-content',
  'image-generation': '/api-reference/images',
  'openai-video': '/api-reference/videos',
};

async function source(relativePath: string) {
  return readFile(join(siteRoot, relativePath), 'utf8');
}

async function catalog(): Promise<Catalog> {
  return JSON.parse(await source('data/development-model-catalog.json')) as Catalog;
}

function routeSlug(value: string) {
  return value
    .normalize('NFKD')
    .replace(/\p{Mark}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function frontmatterValue(markdown: string, field: string) {
  return markdown.match(new RegExp(`^${field}:\\s*(.+)$`, 'm'))?.[1]?.trim();
}

function unescapeMarkdown(value: string) {
  return value.replace(/\\(.)/g, '$1');
}

function modelLinks(markdown: string) {
  return [...markdown.matchAll(/^- \[[^\]]+\]\(\.\/([^)]+)\) — `([^`]+)`$/gm)].map((match) => ({
    slug: match[1],
    id: unescapeMarkdown(match[2]),
  }));
}

function collectSidebarItems(items: Array<SidebarItem | string>): SidebarItem[] {
  const result: SidebarItem[] = [];
  for (const item of items) {
    if (typeof item === 'string') continue;
    result.push(item);
    if (item.items) result.push(...collectSidebarItems(item.items));
  }
  return result;
}

async function markdownFiles(root: string): Promise<string[]> {
  const result: string[] = [];
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) result.push(...await markdownFiles(path));
    else if (/\.mdx?$/.test(entry.name)) result.push(path);
  }
  return result;
}

function docRoute(path: string, markdown: string) {
  const explicitSlug = frontmatterValue(markdown, 'slug');
  if (explicitSlug) return explicitSlug.replace(/\/$/, '') || '/';
  const withoutExtension = relative(docsRoot, path).replace(/\.mdx?$/, '').replaceAll('\\', '/');
  const implicitRoute = withoutExtension.replace(/(?:^|\/)index$/, '');
  return `/${implicitRoute}`.replace(/\/$/, '') || '/';
}

describe('Provider and model documentation integration', () => {
  test('generated routes are a one-to-one ordered projection of the Development snapshot', async () => {
    const snapshot = await catalog();
    const orderedVendors = [...snapshot.vendors]
      .sort((left, right) => left.display_order - right.display_order || left.id - right.id);
    expect(await readdir(providersRoot)).not.toContain('index.mdx');

    const actualModelIds: string[] = [];
    for (const vendor of orderedVendors) {
      const vendorSlug = routeSlug(vendor.name);
      const category = JSON.parse(await source(`docs/providers/${vendorSlug}/_category_.json`));
      const vendorIndex = await source(`docs/providers/${vendorSlug}/index.mdx`);
      const orderedModels = snapshot.models
        .filter((model) => model.vendor_id === vendor.id)
        .sort((left, right) => left.display_order - right.display_order || left.id.localeCompare(right.id));

      expect(category.label).toBe(vendor.name);
      expect(category.position).toBe(vendor.display_order);
      expect(modelLinks(vendorIndex)).toEqual(orderedModels.map((model) => ({
        slug: routeSlug(model.id),
        id: model.id,
      })));

      for (const model of orderedModels) {
        const modelPage = await source(`docs/providers/${vendorSlug}/${routeSlug(model.id)}.mdx`);
        expect(modelPage).toContain(`- **模型 ID：** \`${model.id}\``);
        expect(modelPage).toContain(`- **Provider：** ${vendor.name}`);
        expect(frontmatterValue(modelPage, 'sidebar_position')).toBe(String(model.display_order));
        actualModelIds.push(model.id);
      }
    }

    const generatedModelPages = (await markdownFiles(providersRoot))
      .filter((path) => !path.endsWith('/index.mdx'));
    expect(generatedModelPages).toHaveLength(snapshot.models.length);
    expect(actualModelIds.sort()).toEqual(snapshot.models.map((model) => model.id).sort());
  });

  test('sidebar exposes one autogenerated Provider hierarchy and preserves the deep guides', () => {
    const sidebar = sidebars.docsSidebar as Array<SidebarItem | string>;
    const providerCategory = sidebar.find(
      (item): item is SidebarItem => typeof item !== 'string' && item.label === 'Provider 与模型',
    );
    const autogenerated = collectSidebarItems(sidebar)
      .filter((item) => item.type === 'autogenerated' && item.dirName === 'providers');
    const modelGuides = sidebar.find(
      (item): item is SidebarItem => typeof item !== 'string' && item.label === '模型与能力',
    );

    expect(providerCategory).toBeDefined();
    expect(providerCategory?.link).toEqual({
      type: 'generated-index',
      slug: '/providers',
      description: '按 Provider 浏览 Molii 当前公开模型与兼容协议。',
    });
    expect(providerCategory?.items).toEqual([{ type: 'autogenerated', dirName: 'providers' }]);
    expect(autogenerated).toHaveLength(1);
    expect(modelGuides?.items).toEqual(expect.arrayContaining([
      'models/seedance-2',
      'models/grok-imagine-image',
      'models/grok-imagine-video',
    ]));
  });

  test('every declared endpoint type has a reachable public reference linked from the catalog overview', async () => {
    const snapshot = await catalog();
    const overview = await source('docs/models/overview.mdx');
    const endpointTypes = [...new Set(snapshot.models.flatMap((model) => model.supported_endpoint_types))];

    expect(Object.keys(endpointReferences).sort()).toEqual(endpointTypes.sort());
    for (const endpointType of endpointTypes) {
      const referenceRoute = endpointReferences[endpointType];
      expect(overview, endpointType).toContain(`](${referenceRoute})`);
      expect(await source(`docs${referenceRoute}.mdx`), referenceRoute).toContain('# ');
    }

    expect(overview).toContain(`${snapshot.vendors.length} 个 Provider`);
    expect(overview).toContain(`${snapshot.models.length} 个模型`);
    expect(overview).toContain(snapshot.source);
    expect(overview).toContain(snapshot.pricing_version);
    expect(overview).toContain('](/providers)');
  });

  test('generated Grok and Seedance pages retain links to their existing deep guides', async () => {
    const snapshot = await catalog();
    const vendors = new Map(snapshot.vendors.map((vendor) => [vendor.id, vendor]));
    const guideByPrefix = [
      ['grok-imagine-image', '/models/grok-imagine-image'],
      ['grok-imagine-video', '/models/grok-imagine-video'],
      ['doubao-seedance', '/models/seedance-2'],
    ] as const;

    for (const model of snapshot.models) {
      const guide = guideByPrefix.find(([prefix]) => model.id.startsWith(prefix))?.[1];
      if (!guide) continue;
      const vendor = vendors.get(model.vendor_id);
      expect(vendor).toBeDefined();
      const modelPage = await source(
        `docs/providers/${routeSlug(vendor!.name)}/${routeSlug(model.id)}.mdx`,
      );
      expect(modelPage, model.id).toContain(`](${guide})`);
    }
  });

  test('public documentation routes are unique', async () => {
    const routes = new Map<string, string[]>();
    for (const path of await markdownFiles(docsRoot)) {
      const markdown = await readFile(path, 'utf8');
      const route = docRoute(path, markdown);
      routes.set(route, [...(routes.get(route) ?? []), relative(docsRoot, path)]);
    }

    const duplicates = [...routes.entries()].filter(([, paths]) => paths.length > 1);
    expect(duplicates).toEqual([]);
    expect(routes.has('/providers')).toBeFalse();
    const sidebar = sidebars.docsSidebar as Array<SidebarItem | string>;
    const providerCategory = sidebar.find(
      (item): item is SidebarItem => typeof item !== 'string' && item.label === 'Provider 与模型',
    );
    expect(providerCategory?.link?.type).toBe('generated-index');
    expect(providerCategory?.link?.slug).toBe('/providers');
  });

  test('catalog integration remains public-only', async () => {
    const integrationFiles = [
      ...(await markdownFiles(providersRoot)),
      join(docsRoot, 'models.md'),
      join(docsRoot, 'models/overview.mdx'),
      join(docsRoot, 'api-reference/index.mdx'),
    ];
    const combined = (await Promise.all(integrationFiles.map((path) => readFile(path, 'utf8')))).join('\n');

    for (const forbidden of [
      '/api/channel',
      '/api/option',
      '/api/system',
      'X-Admin-Key',
      'upstreamCredential',
      'channel_secret',
      'admin_token',
    ]) {
      expect(combined).not.toContain(forbidden);
    }
  });
});
