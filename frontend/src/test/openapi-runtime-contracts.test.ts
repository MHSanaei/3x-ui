import { describe, expect, it } from 'vitest';

import { buildSpec } from '../../scripts/build-openapi.mjs';

interface OpenApiSchema {
  $ref?: string;
  type?: string;
  format?: string;
  enum?: readonly string[];
  required?: string[];
  properties?: Record<string, OpenApiSchema>;
  items?: OpenApiSchema;
}

interface OpenApiParameter {
  name: string;
  required: boolean;
  description: string;
  schema: OpenApiSchema;
}

interface WebSocketEventDoc {
  type: string;
  summary: string;
  payloadSchema: OpenApiSchema;
  example: { type: string; payload: unknown; time: number };
}

interface OpenApiOperation {
  parameters?: OpenApiParameter[];
  responses: Record<
    string,
    {
      content?: Record<string, { schema: OpenApiSchema }>;
    }
  >;
  security?: Record<string, never[]>[];
  'x-websocket-events'?: WebSocketEventDoc[];
}

interface OpenApiSpec {
  paths: Record<string, Record<string, OpenApiOperation>>;
  components: { schemas: Record<string, OpenApiSchema> };
}

const spec = buildSpec() as unknown as OpenApiSpec;

function operation(path: string, method: string): OpenApiOperation {
  const op = spec.paths[path]?.[method];
  if (!op) throw new Error(`${method.toUpperCase()} ${path} is missing`);
  return op;
}

function responseObjectSchema(path: string, method = 'get'): OpenApiSchema {
  const schema = operation(path, method).responses['200']?.content?.['application/json']?.schema;
  const obj = schema?.properties?.obj;
  if (!obj) throw new Error(`${method.toUpperCase()} ${path} has no response obj schema`);
  return obj;
}

describe('generated OpenAPI runtime contracts', () => {
  it('exports only valid OpenAPI paths and HTTP methods', () => {
    const validMethods = new Set([
      'get',
      'put',
      'post',
      'delete',
      'options',
      'head',
      'patch',
      'trace',
    ]);

    for (const [path, pathItem] of Object.entries(spec.paths)) {
      expect(path.startsWith('/'), path).toBe(true);
      for (const method of Object.keys(pathItem)) {
        expect(validMethods.has(method), `${method.toUpperCase()} ${path}`).toBe(true);
      }
    }
  });

  it('documents the WebSocket handshake and every emitted event', () => {
    const ws = operation('/ws', 'get');
    expect(Object.keys(ws.responses)).toEqual(['101', '401']);
    expect(ws.security).toEqual([{ cookieAuth: [] }]);

    const envelope = spec.components.schemas.WebSocketEnvelope;
    expect(envelope.required).toEqual(['type', 'payload', 'time']);
    expect(envelope.properties?.time).toMatchObject({ type: 'integer', format: 'int64' });

    const events = ws['x-websocket-events'] ?? [];
    expect(events.map((event) => event.type)).toEqual([
      'status',
      'traffic',
      'client_stats',
      'inbounds',
      'outbounds',
      'nodes',
      'notification',
      'xray_state',
      'invalidate',
    ]);
    expect(events.map((event) => event.type)).not.toContain('clients');

    for (const event of events) {
      expect(Object.keys(event.example)).toEqual(['type', 'payload', 'time']);
      expect(event.example.type).toBe(event.type);
      expect(event.example.time).toEqual(expect.any(Number));
    }

    const byType = Object.fromEntries(events.map((event) => [event.type, event]));
    expect(Object.keys(byType.notification.payloadSchema.properties ?? {})).toEqual([
      'title',
      'message',
      'level',
    ]);
    expect(Object.keys(byType.xray_state.payloadSchema.properties ?? {})).toEqual([
      'state',
      'errorMsg',
    ]);
    expect(byType.invalidate.payloadSchema.properties?.type?.enum).toContain('clients');
    expect(byType.xray_state.example.payload).toEqual({ state: 'running', errorMsg: '' });
    expect(byType.invalidate.example.payload).toEqual({ type: 'inbounds' });
    expect(byType.status.payloadSchema.properties?.loads).toMatchObject({
      type: 'array',
      nullable: true,
    });
  });

  it('uses the runtime REST response schemas', () => {
    expect(responseObjectSchema('/panel/api/server/logs/{count}', 'post')).toEqual({
      type: 'array',
      items: { type: 'string' },
    });
    expect(responseObjectSchema('/panel/api/server/xraylogs/{count}', 'post')).toEqual({
      type: 'array',
      items: { $ref: '#/components/schemas/LogEntry' },
    });
    expect(responseObjectSchema('/panel/api/server/getNewUUID')).toEqual({
      $ref: '#/components/schemas/NewUUIDResponse',
    });
    expect(responseObjectSchema('/panel/api/server/getNewmldsa65')).toEqual({
      $ref: '#/components/schemas/MLDSA65Response',
    });
    expect(responseObjectSchema('/panel/api/server/getNewmlkem768')).toEqual({
      $ref: '#/components/schemas/MLKEM768Response',
    });

    const logEntryFields = Object.keys(spec.components.schemas.LogEntry.properties ?? {});
    expect(logEntryFields).toHaveLength(7);
    expect(logEntryFields).toEqual(
      expect.arrayContaining([
        'DateTime',
        'FromAddress',
        'ToAddress',
        'Inbound',
        'Outbound',
        'Email',
        'Event',
      ]),
    );
    expect(spec.components.schemas.LogEntry.properties?.DateTime).toMatchObject({
      type: 'string',
      format: 'date-time',
    });
    expect(Object.keys(spec.components.schemas.NewUUIDResponse.properties ?? {})).toEqual(['uuid']);
    expect(Object.keys(spec.components.schemas.MLDSA65Response.properties ?? {})).toEqual([
      'seed',
      'verify',
    ]);
    const mlkemFields = Object.keys(spec.components.schemas.MLKEM768Response.properties ?? {});
    expect(mlkemFields).toHaveLength(2);
    expect(mlkemFields).toEqual(expect.arrayContaining(['seed', 'client']));
  });

  it('documents every paged-client query and the groups response', () => {
    const paged = operation('/panel/api/clients/list/paged', 'get');
    expect(paged.parameters?.map((param) => param.name)).toEqual([
      'page',
      'pageSize',
      'search',
      'filter',
      'protocol',
      'inbound',
      'sort',
      'order',
      'expiryFrom',
      'expiryTo',
      'usageFrom',
      'usageTo',
      'autoRenew',
      'hasTgId',
      'hasComment',
      'group',
    ]);
    expect(paged.parameters?.every((param) => param.required === false)).toBe(true);

    const params = Object.fromEntries((paged.parameters ?? []).map((param) => [param.name, param]));
    for (const name of ['filter', 'protocol', 'inbound', 'group']) {
      expect(params[name].description).toContain('CSV');
    }
    expect(params.sort.schema.enum).toEqual([
      'enable',
      'email',
      'inboundIds',
      'traffic',
      'remaining',
      'expiryTime',
      'createdAt',
      'updatedAt',
      'lastOnline',
    ]);

    expect(responseObjectSchema('/panel/api/clients/list/paged')).toEqual({
      $ref: '#/components/schemas/ClientPageResponse',
    });
    expect(spec.components.schemas.ClientPageResponse.properties?.groups).toMatchObject({
      type: 'array',
      items: { type: 'string' },
    });
  });

  it('includes HEAD operations for every subscription variant', () => {
    expect(operation('/{subPath}{subid}', 'head')).toBeDefined();
    expect(operation('/{jsonPath}{subid}', 'head')).toBeDefined();
    expect(operation('/{clashPath}{subid}', 'head')).toBeDefined();
  });
});
