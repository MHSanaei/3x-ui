#!/usr/bin/env node
import { writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { sections } from '../src/pages/api-docs/endpoints.ts';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outPath = join(__dirname, '..', 'public', 'openapi.json');

const PANEL_VERSION = process.env.X_UI_VERSION || '3.x';

const SECURITY_SCHEMES = {
  bearerAuth: {
    type: 'http',
    scheme: 'bearer',
    description: 'API token from Settings → Security → API Token. Send as `Authorization: Bearer <token>`.',
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
  if (v === 'number' || v === 'integer' || v === 'int') return 'integer';
  if (v === 'float' || v === 'double') return 'number';
  if (v === 'boolean' || v === 'bool') return 'boolean';
  if (v === 'array') return 'array';
  if (v === 'object') return 'object';
  return 'string';
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
    schema: { type: mapType(p.type) },
  };
  if (p.defaultValue !== undefined) out.schema.default = p.defaultValue;
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
    if (p.in === 'body') {
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

  if (ep.body || bodyParams.length > 0) {
    const example = tryParseJson(ep.body);
    const properties = {};
    const required = [];
    for (const bp of bodyParams) {
      properties[bp.name] = {
        type: mapType(bp.type),
        description: bp.desc || '',
      };
      if (!bp.optional) required.push(bp.name);
    }
    const schema = bodyParams.length > 0
      ? { type: 'object', properties, ...(required.length > 0 ? { required } : {}) }
      : { type: 'object' };

    op.requestBody = {
      required: required.length > 0 || bodyParams.length === 0,
      content: {
        'application/json': {
          schema,
          ...(example !== undefined ? { example } : {}),
        },
      },
    };
  }

  const responses = {};
  const successExample = tryParseJson(ep.response);
  responses['200'] = {
    description: 'Successful response',
    content: {
      'application/json': {
        schema: {
          type: 'object',
          properties: {
            success: { type: 'boolean' },
            msg: { type: 'string' },
            obj: {},
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

function buildSpec() {
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
        'Programmatic interface to a 3X-UI panel. Authenticate either by logging in (cookie) or with an API token from Settings → Security → API Token (Bearer). All endpoints under /panel/api/* honour both modes.',
    },
    servers: [
      { url: '/', description: 'Current panel (basePath aware)' },
    ],
    components: {
      securitySchemes: SECURITY_SCHEMES,
    },
    security: [{ bearerAuth: [] }, { cookieAuth: [] }],
    tags,
    paths,
  };
}

const spec = buildSpec();
writeFileSync(outPath, JSON.stringify(spec, null, 2) + '\n');

const pathCount = Object.keys(spec.paths).length;
let opCount = 0;
for (const ops of Object.values(spec.paths)) opCount += Object.keys(ops).length;
console.log(`[openapi] wrote ${outPath}`);
console.log(`[openapi] paths: ${pathCount}, operations: ${opCount}, tags: ${spec.tags.length}`);

void pathToFileURL;
