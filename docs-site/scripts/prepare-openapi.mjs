import { mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';

const HTTP_METHODS = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options', 'trace']);
const FORBIDDEN_PATHS = new Set(['/api/channel', '/api/models', '/api/assets/admin']);
const FORBIDDEN_SCHEMA = /administrator/i;
const PUBLIC_SECURITY_SCHEMES = new Set(['BearerAuth', 'AnthropicApiKey', 'GeminiApiKey']);

function parseJsonFile(path) {
  return readFile(path, 'utf8').then((contents) => JSON.parse(contents));
}

function clone(value) {
  return structuredClone(value);
}

function operationKey(method, path) {
  return `${method.toUpperCase()} ${path}`;
}

function allowedSecurity(security) {
  return Array.isArray(security)
    && security.length === 1
    && Object.keys(security[0]).length === 1
    && PUBLIC_SECURITY_SCHEMES.has(Object.keys(security[0])[0])
    && Array.isArray(Object.values(security[0])[0])
    && Object.values(security[0])[0].length === 0;
}

function referencesIn(value, references = []) {
  if (Array.isArray(value)) {
    value.forEach((item) => referencesIn(item, references));
  } else if (value && typeof value === 'object') {
    if (typeof value.$ref === 'string') references.push(value.$ref);
    Object.values(value).forEach((item) => referencesIn(item, references));
  }
  return references;
}

function pruneComponents(sourceComponents = {}, paths) {
  const output = {};
  const queue = referencesIn(paths);
  const seen = new Set();

  while (queue.length > 0) {
    const reference = queue.pop();
    const match = /^#\/components\/([^/]+)\/([^/]+)$/.exec(reference);
    if (!match) continue;
    const [, section, name] = match;
    const key = `${section}/${name}`;
    if (seen.has(key)) continue;
    seen.add(key);

    const value = sourceComponents[section]?.[name];
    if (value === undefined) {
      throw new Error(`Missing component referenced by a public operation: ${reference}`);
    }
    output[section] ??= {};
    output[section][name] = clone(value);
    queue.push(...referencesIn(value));
  }

  output.securitySchemes = Object.fromEntries([...PUBLIC_SECURITY_SCHEMES].map((name) => {
    const scheme = sourceComponents.securitySchemes?.[name];
    if (scheme === undefined) throw new Error(`Missing public security scheme: ${name}`);
    return [name, clone(scheme)];
  }));
  return output;
}

function validatePublicDocument(document) {
  for (const path of Object.keys(document.paths)) {
    if (FORBIDDEN_PATHS.has(path)) {
      throw new Error(`Forbidden private path in public OpenAPI document: ${path}`);
    }
  }
  for (const schemaName of Object.keys(document.components?.schemas ?? {})) {
    if (FORBIDDEN_SCHEMA.test(schemaName)) {
      throw new Error(`Forbidden administrator schema in public OpenAPI document: ${schemaName}`);
    }
  }
}

export async function resetGeneratedApiDirectory({ siteRoot } = {}) {
  if (!siteRoot) throw new Error('siteRoot is required to reset generated API docs');
  const normalizedSiteRoot = resolve(siteRoot);
  const generatedRoot = resolve(normalizedSiteRoot, 'generated');
  const generatedApiRoot = resolve(generatedRoot, 'api');
  if (dirname(generatedApiRoot) !== generatedRoot || generatedApiRoot === normalizedSiteRoot) {
    throw new Error(`Refusing to reset unsafe generated API path: ${generatedApiRoot}`);
  }
  await rm(generatedApiRoot, { force: true, recursive: true });
  await mkdir(generatedApiRoot, { recursive: true });
  return generatedApiRoot;
}

export async function prepareOpenApi({ templatePath, allowlistPath, outputPath, apiBaseUrl, document: suppliedDocument } = {}) {
  if (!templatePath || !allowlistPath || !outputPath || !apiBaseUrl) {
    throw new Error('templatePath, allowlistPath, outputPath, and apiBaseUrl are required');
  }
  const source = clone(suppliedDocument ?? await parseJsonFile(templatePath));
  const allowlist = await parseJsonFile(allowlistPath);
  if (!Array.isArray(allowlist.operations) || allowlist.operations.length === 0) {
    throw new Error('Public OpenAPI allowlist must contain a non-empty operations array');
  }

  const selectedPaths = {};
  const selectedOperations = [];
  const seenRoutes = new Set();
  const seenOperationIds = new Set();

  for (const entry of allowlist.operations) {
    const [method, path, ...extra] = String(entry).split(' ');
    const methodName = method?.toLowerCase();
    const route = operationKey(methodName ?? '', path ?? '');
    if (extra.length > 0 || !HTTP_METHODS.has(methodName) || !path?.startsWith('/')) {
      throw new Error(`Invalid allowlist operation: ${entry}`);
    }
    if (seenRoutes.has(route)) throw new Error(`Duplicate allowlist operation: ${route}`);
    seenRoutes.add(route);

    const sourceOperation = source.paths?.[path]?.[methodName];
    if (!sourceOperation) throw new Error(`Allowlisted operation is missing from the public template: ${route}`);
    if (!sourceOperation.summary?.trim()) throw new Error(`Public operation requires a summary: ${route}`);
    if (!sourceOperation.responses || Object.keys(sourceOperation.responses).length === 0) {
      throw new Error(`Public operation requires responses: ${route}`);
    }
    if (!allowedSecurity(sourceOperation.security)) {
      throw new Error(`Public operation must use one documented public security scheme: ${route}`);
    }
    if (!sourceOperation.operationId?.trim()) throw new Error(`Public operation requires an operationId: ${route}`);
    if (seenOperationIds.has(sourceOperation.operationId)) {
      throw new Error(`Duplicate public operationId: ${sourceOperation.operationId}`);
    }
    seenOperationIds.add(sourceOperation.operationId);

    selectedPaths[path] ??= {};
    selectedPaths[path][methodName] = clone(sourceOperation);
    selectedOperations.push(route);
  }

  const result = {
    openapi: source.openapi,
    info: clone(source.info),
    servers: [{ url: apiBaseUrl }],
    security: [{ BearerAuth: [] }],
    tags: clone(source.tags ?? []),
    paths: selectedPaths,
    components: pruneComponents(source.components, selectedPaths),
  };
  validatePublicDocument(result);

  await mkdir(dirname(outputPath), { recursive: true });
  await writeFile(outputPath, `${JSON.stringify(result, null, 2)}\n`);
  return result;
}

async function main() {
  const siteRoot = resolve(import.meta.dirname, '..');
  const args = process.argv.slice(2);
  const prepareForGeneration = args.length === 1 && args[0] === '--prepare-for-generation';
  if (args.length > 0 && !prepareForGeneration) {
    throw new Error(`Unknown prepare-openapi option: ${args.join(' ')}`);
  }
  await prepareOpenApi({
    templatePath: resolve(siteRoot, 'openapi/relay.public.template.yaml'),
    allowlistPath: resolve(siteRoot, 'openapi/public-api-surface.json'),
    outputPath: resolve(siteRoot, 'generated/openapi/relay.public.json'),
    apiBaseUrl: process.env.DOCS_API_BASE_URL ?? 'http://127.0.0.1:3000',
  });
  if (prepareForGeneration) {
    await resetGeneratedApiDirectory({ siteRoot });
  }
}

if (import.meta.main) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
