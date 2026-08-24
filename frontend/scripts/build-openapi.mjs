#!/usr/bin/env node
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { sections } from '../src/pages/api-docs/endpoints.ts';
import { EXAMPLES } from '../src/generated/examples.ts';
import { SCHEMAS } from '../src/generated/schemas.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outPath = join(__dirname, '..', 'public', 'openapi.json');

const PANEL_VERSION = process.env.X_UI_VERSION || '3.x';

const SECURITY_SCHEMES = {
  bearerAuth: {
    type: 'http',
    scheme: 'bearer',
    description:
      'API token from Settings → Security → API Token. Send as `Authorization: Bearer <token>`.',
  },
  cookieAuth: {
    type: 'apiKey',
    in: 'cookie',
    name: '3x-ui',
    description: 'Session cookie set by POST /login. Browser-only.',
  },
};

function ginPathToOpenApi(path) {
  return path.replace(/:([A-Za-z_][A-Za-z0-9_]*)/g, '{$1}');
}

function extractPathParams(openApiPath) {
  const params = [];
  const re = /\{([A-Za-z_][A-Za-z0-9_]*)\}/g;
  let m;
  while ((m = re.exec(openApiPath)) !== null) params.push(m[1]);
  return params;
}

function mapType(t) {
  const v = String(t || '').toLowerCase();
  if (v.endsWith('[]')) return 'array';
  if (v === 'number' || v === 'integer' || v === 'int') return 'integer';
  if (v === 'float' || v === 'double') return 'number';
  if (v === 'boolean' || v === 'bool') return 'boolean';
  if (v === 'array') return 'array';
  if (v === 'object') return 'object';
  return 'string';
}

function schemaFromType(t) {
  const v = String(t || '').toLowerCase();
  if (v.endsWith('[]')) {
    const itemType = v.slice(0, -2);
    return { type: 'array', items: { type: mapType(itemType) } };
  }
  if (v === 'file') return { type: 'string', format: 'binary' };
  return { type: mapType(v) };
}

function schemaFromParam(p) {
  const schema = schemaFromType(p.type);
  if (p.defaultValue !== undefined) schema.default = p.defaultValue;
  if (p.minLength !== undefined) schema.minLength = p.minLength;
  if (p.pattern !== undefined) schema.pattern = p.pattern;
  return schema;
}

function requestBodyContentType(bodyParams) {
  const locations = new Set(bodyParams.map((p) => p.in));
  if (locations.size > 1) {
    throw new Error(`request body mixes parameter locations: ${[...locations].join(', ')}`);
  }
  switch (bodyParams[0]?.in) {
    case 'body (form)':
      return 'application/x-www-form-urlencoded';
    case 'body (multipart)':
      return 'multipart/form-data';
    default:
      return 'application/json';
  }
}

function tryParseJson(raw) {
  if (typeof raw !== 'string') return undefined;
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function paramToOpenApi(p) {
  const out = {
    name: p.name,
    in: p.in,
    required: p.in === 'path' ? true : !p.optional,
    description: p.desc || '',
    schema: schemaFromParam(p),
  };
  return out;
}

function buildOperation(ep, tag) {
  const op = {
    tags: [tag],
    summary: ep.summary || '',
    operationId: `${ep.method.toLowerCase()}_${ep.path.replace(/[^A-Za-z0-9]+/g, '_').replace(/^_|_$/g, '')}`,
  };
  if (ep.description) op.description = ep.description;
  if (ep.deprecated) op.deprecated = true;

  const params = [];
  const bodyParams = [];
  for (const p of ep.params || []) {
    if (p.in.startsWith('body')) {
      bodyParams.push(p);
    } else if (p.in === 'path' || p.in === 'query' || p.in === 'header') {
      params.push(paramToOpenApi(p));
    }
  }

  const openApiPath = ginPathToOpenApi(ep.path);
  const declared = new Set(params.filter((x) => x.in === 'path').map((x) => x.name));
  for (const name of extractPathParams(openApiPath)) {
    if (declared.has(name)) continue;
    params.push({
      name,
      in: 'path',
      required: true,
      description: '',
      schema: { type: 'string' },
    });
  }

  if (params.length > 0) op.parameters = params;

  if (ep.body || bodyParams.length > 0 || ep.requestSchema) {
    const contentType = requestBodyContentType(bodyParams);
    const example = contentType === 'application/json' ? tryParseJson(ep.body) : undefined;
    const properties = {};
    const required = [];
    for (const bp of bodyParams) {
      properties[bp.name] = {
        ...schemaFromParam(bp),
        description: bp.desc || '',
      };
      if (!bp.optional) required.push(bp.name);
    }
    let schema;
    if (ep.requestSchema) {
      if (bodyParams.length > 0) {
        throw new Error(
          `${ep.method} ${ep.path}: requestSchema cannot be combined with body parameters`,
        );
      }
      schema = ep.requestSchema;
    } else {
      schema =
        bodyParams.length > 0
          ? { type: 'object', properties, ...(required.length > 0 ? { required } : {}) }
          : { type: 'object' };
      if (ep.bodyRequiredOneOf?.length) {
        schema = {
          anyOf: ep.bodyRequiredOneOf.map((name) => ({
            type: 'object',
            properties,
            required: [...required, name],
          })),
        };
      }
    }

    const encoding = {};
    if (contentType === 'application/x-www-form-urlencoded') {
      for (const bp of bodyParams) {
        if (schemaFromType(bp.type).type === 'array') {
          encoding[bp.name] = { style: 'form', explode: true };
        }
      }
    }

    op.requestBody = {
      required:
        Boolean(ep.requestSchema) ||
        Boolean(ep.bodyRequiredOneOf?.length) ||
        required.length > 0 ||
        bodyParams.length === 0,
      content: {
        [contentType]: {
          schema,
          ...(Object.keys(encoding).length > 0 ? { encoding } : {}),
          ...(example !== undefined ? { example } : {}),
        },
      },
    };
  }

  const responses = {};
  let successExample = tryParseJson(ep.response);
  let objSchema = {};
  if (ep.responseSchema) {
    const obj = EXAMPLES[ep.responseSchema];
    if (obj === undefined) {
      throw new Error(
        `${ep.method} ${ep.path}: responseSchema "${ep.responseSchema}" has no generated example`,
      );
    }
    if (SCHEMAS[ep.responseSchema] === undefined) {
      throw new Error(
        `${ep.method} ${ep.path}: responseSchema "${ep.responseSchema}" has no generated schema`,
      );
    }
    const ref = { $ref: `#/components/schemas/${ep.responseSchema}` };
    objSchema = ep.responseSchemaArray ? { type: 'array', items: ref } : ref;
    if (successExample === undefined) {
      successExample = { success: true, obj: ep.responseSchemaArray ? [obj] : obj };
    }
  }
  responses['200'] = {
    description: 'Successful response',
    content: {
      'application/json': {
        schema: {
          type: 'object',
          properties: {
            success: { type: 'boolean' },
            msg: { type: 'string' },
            obj: objSchema,
          },
        },
        ...(successExample !== undefined ? { example: successExample } : {}),
      },
    },
  };

  const errExample = tryParseJson(ep.errorResponse);
  if (errExample !== undefined || ep.errorStatus) {
    const code = String(ep.errorStatus || 400);
    responses[code] = {
      description: 'Error response',
      content: {
        'application/json': {
          schema: {
            type: 'object',
            properties: {
              success: { type: 'boolean' },
              msg: { type: 'string' },
            },
          },
          ...(errExample !== undefined ? { example: errExample } : {}),
        },
      },
    };
  }

  op.responses = responses;
  return op;
}

export function buildSpec() {
  const paths = {};
  for (const section of sections) {
    const tag = section.title;
    for (const ep of section.endpoints) {
      const openApiPath = ginPathToOpenApi(ep.path);
      if (!paths[openApiPath]) paths[openApiPath] = {};
      paths[openApiPath][ep.method.toLowerCase()] = buildOperation(ep, tag);
    }
  }

  const tags = sections.map((s) => ({
    name: s.title,
    description: s.description || '',
  }));

  return {
    openapi: '3.0.3',
    info: {
      title: '3X-UI Panel API',
      version: PANEL_VERSION,
      description:
        'Programmatic interface to a 3X-UI panel. Authenticate either by logging in (cookie) or with an API token from Settings → Security → API Token (Bearer). All endpoints under /panel/api/* honour both modes — an API token is a full-admin credential, so treat it like the panel password.',
    },
    servers: [{ url: '/', description: 'Current panel (basePath aware)' }],
    components: {
      securitySchemes: SECURITY_SCHEMES,
      schemas: SCHEMAS,
    },
    security: [{ bearerAuth: [] }, { cookieAuth: [] }],
    tags,
    paths,
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const spec = buildSpec();
  writeFileSync(outPath, JSON.stringify(spec, null, 2) + '\n');

  const pathCount = Object.keys(spec.paths).length;
  let opCount = 0;
  for (const ops of Object.values(spec.paths)) opCount += Object.keys(ops).length;
  console.log(`[openapi] wrote ${outPath}`);
  console.log(`[openapi] paths: ${pathCount}, operations: ${opCount}, tags: ${spec.tags.length}`);
}
