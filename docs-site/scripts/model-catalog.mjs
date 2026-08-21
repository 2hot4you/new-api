export const CATALOG_SOURCE = 'https://dev.molii.co/api/pricing';

export const ENDPOINT_TYPES = new Set([
  'openai',
  'openai-response',
  'anthropic',
  'gemini',
  'image-generation',
  'openai-video',
]);

const MODEL_STRING_FIELDS = ['display_name', 'description', 'description_en', 'knowledge_cutoff', 'release_date'];
const MODEL_ARRAY_FIELDS = [
  'input_modalities',
  'output_modalities',
  'capabilities',
  'supported_parameters',
  'supported_resolutions',
  'supported_aspect_ratios',
  'output_formats',
  'reference_modalities',
];
const MODEL_NUMBER_FIELDS = ['context_length', 'max_output_tokens', 'max_input_images', 'min_duration', 'max_duration'];

function fail(message) {
  throw new Error(`Invalid Development model catalog: ${message}`);
}

function asObject(value, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail(`${label} must be an object`);
  return value;
}

function requiredString(value, label) {
  if (typeof value !== 'string' || !value.trim()) fail(`${label} must be a non-empty string`);
  return value.trim();
}

function requiredPositiveInteger(value, label) {
  if (!Number.isSafeInteger(value) || value < 1) fail(`${label} must be a positive integer`);
  return value;
}

function optionalString(value, label) {
  if (value === undefined) return undefined;
  return requiredString(value, label);
}

function optionalPositiveInteger(value, label) {
  if (value === undefined) return undefined;
  return requiredPositiveInteger(value, label);
}

function publicStrings(value, label) {
  if (value === undefined) return [];
  if (!Array.isArray(value) || value.some((entry) => typeof entry !== 'string' || !entry.trim())) {
    fail(`${label} must be an array of non-empty strings`);
  }
  return value.map((entry) => entry.trim());
}

export function slugify(value) {
  const slug = requiredString(value, 'slug source')
    .normalize('NFKD')
    .replace(/\p{Mark}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  if (!slug) fail(`empty slug for ${JSON.stringify(value)}`);
  return slug;
}

function ordered(items) {
  return [...items].sort((left, right) => left.display_order - right.display_order || left.id.localeCompare(right.id));
}

/**
 * Converts the Development pricing response into the committed public catalog.
 * The returned object is deliberately an allowlist so pricing, channel, group,
 * and upstream-routing fields cannot reach generated documentation.
 */
export function sanitizeCatalogResponse(raw) {
  const response = asObject(raw, 'response');
  if (response.success !== true) fail('success must be true');
  const pricingVersion = requiredString(response.pricing_version, 'pricing_version');
  if (!Array.isArray(response.vendors)) fail('vendors must be an array');
  if (!Array.isArray(response.data)) fail('data must be an array');

  const providerSlugs = new Set();
  const vendors = response.vendors.map((vendorValue, index) => {
    const vendor = asObject(vendorValue, `vendors[${index}]`);
    const name = requiredString(vendor.name, `vendors[${index}].name`);
    const slug = slugify(name);
    if (providerSlugs.has(slug)) fail(`Duplicate Provider slug: ${slug}`);
    providerSlugs.add(slug);
    return {
      id: requiredPositiveInteger(vendor.id, `vendors[${index}].id`),
      name,
      display_order: requiredPositiveInteger(vendor.display_order, `vendors[${index}].display_order`),
      description: optionalString(vendor.description, `vendors[${index}].description`) ?? '',
    };
  });

  const vendorIds = new Set();
  for (const vendor of vendors) {
    if (vendorIds.has(vendor.id)) fail(`Duplicate Provider id: ${vendor.id}`);
    vendorIds.add(vendor.id);
  }

  const modelSlugs = new Set();
  const models = response.data.map((modelValue, index) => {
    const model = asObject(modelValue, `data[${index}]`);
    const id = requiredString(model.model_name, `data[${index}].model_name`);
    const slug = slugify(id);
    if (modelSlugs.has(slug)) fail(`Duplicate model slug: ${slug}`);
    modelSlugs.add(slug);

    const vendorId = requiredPositiveInteger(model.vendor_id, `data[${index}].vendor_id`);
    if (!vendorIds.has(vendorId)) fail(`model ${id} references an unknown Provider: ${vendorId}`);

    const endpointTypes = publicStrings(model.supported_endpoint_types, `data[${index}].supported_endpoint_types`);
    if (!endpointTypes.length) fail(`model ${id} must declare at least one endpoint type`);
    for (const endpointType of endpointTypes) {
      if (!ENDPOINT_TYPES.has(endpointType)) fail(`Unknown endpoint type: ${endpointType}`);
    }
    if (new Set(endpointTypes).size !== endpointTypes.length) fail(`model ${id} has duplicate endpoint types`);

    const publicModel = {
      id,
      display_name: optionalString(model.display_name, `data[${index}].display_name`) ?? id,
      description: optionalString(model.description, `data[${index}].description`) ?? '',
      vendor_id: vendorId,
      display_order: requiredPositiveInteger(model.display_order, `data[${index}].display_order`),
      supported_endpoint_types: endpointTypes,
    };
    for (const field of MODEL_STRING_FIELDS) {
      if (field === 'display_name' || field === 'description') continue;
      const value = optionalString(model[field], `data[${index}].${field}`);
      if (value !== undefined) publicModel[field] = value;
    }
    for (const field of MODEL_ARRAY_FIELDS) {
      if (model[field] !== undefined) publicModel[field] = publicStrings(model[field], `data[${index}].${field}`);
    }
    for (const field of MODEL_NUMBER_FIELDS) {
      const value = optionalPositiveInteger(model[field], `data[${index}].${field}`);
      if (value !== undefined) publicModel[field] = value;
    }
    return publicModel;
  });

  return {
    source: CATALOG_SOURCE,
    pricing_version: pricingVersion,
    vendors: ordered(vendors),
    models: [...models].sort((left, right) => {
      const leftProvider = vendors.find((vendor) => vendor.id === left.vendor_id);
      const rightProvider = vendors.find((vendor) => vendor.id === right.vendor_id);
      return leftProvider.display_order - rightProvider.display_order
        || left.display_order - right.display_order
        || left.id.localeCompare(right.id);
    }),
  };
}
