import { describe, expect, it } from 'vitest';

import { buildSpec } from '../../scripts/build-openapi.mjs';

interface OpenApiSchema {
  type?: string;
  format?: string;
  description?: string;
  default?: string | number | boolean;
  minLength?: number;
  pattern?: string;
  properties?: Record<string, OpenApiSchema>;
  required?: string[];
  items?: OpenApiSchema;
  anyOf?: OpenApiSchema[];
}

interface OpenApiRequestBody {
  required?: boolean;
  content: Record<
    string,
    {
      schema: OpenApiSchema;
      encoding?: Record<string, { style?: string; explode?: boolean; contentType?: string }>;
    }
  >;
}

interface OpenApiOperation {
  requestBody?: OpenApiRequestBody;
}

const paths = buildSpec().paths as Record<string, Record<string, OpenApiOperation>>;

function requestBody(path: string): OpenApiRequestBody {
  const body = paths[path]?.post?.requestBody;
  if (!body) throw new Error(`${path} has no POST request body`);
  return body;
}

describe('generated OpenAPI request bodies', () => {
  it('preserves JSON, form, and multipart parameter declarations', () => {
    const login = requestBody('/login').content['application/json'];
    expect(login.schema.properties).toHaveProperty('username');
    expect(login.schema.required).toEqual(['username', 'password']);

    const json = requestBody('/panel/api/inbounds/pushClientTraffics').content['application/json'];
    expect(json.schema.properties).toHaveProperty('traffics');

    const form = requestBody('/panel/api/inbounds/import').content[
      'application/x-www-form-urlencoded'
    ];
    expect(form.schema.properties).toHaveProperty('data');

    const logs = requestBody('/panel/api/server/logs/{count}');
    expect(logs.content).toHaveProperty('application/x-www-form-urlencoded');
    expect(logs.content['application/x-www-form-urlencoded'].schema.properties).toHaveProperty(
      'syslog',
    );

    const outboundTest = requestBody('/panel/api/xray/testOutbound').content[
      'application/x-www-form-urlencoded'
    ];
    expect(outboundTest.schema.required).toEqual(['outbound']);

    const outboundUpdate = requestBody('/panel/api/xray/outbound-subs/{id}').content[
      'application/x-www-form-urlencoded'
    ];
    expect(outboundUpdate.schema.required).toEqual(['url']);
    expect(outboundUpdate.schema.properties).toHaveProperty('allowInsecure');

    const balancerUpdate = requestBody('/panel/api/sub-balancers/{id}').content[
      'application/x-www-form-urlencoded'
    ];
    expect(balancerUpdate.schema.required).toEqual(['remark', 'inboundIds']);
    expect(balancerUpdate.encoding?.inboundIds).toEqual({ style: 'form', explode: true });
    expect(balancerUpdate.encoding?.memberWeights).toEqual({ contentType: 'application/json' });

    const inboundUpdate = requestBody('/panel/api/inbounds/update/{id}').content[
      'application/json'
    ];
    expect(inboundUpdate.schema).toEqual({ type: 'object' });

    const multipart = requestBody('/panel/api/server/importDB').content['multipart/form-data'];
    expect(multipart.schema.properties?.db).toEqual({
      type: 'string',
      format: 'binary',
      description: 'Database backup or migration file to upload.',
    });
    expect(multipart.schema.properties).toHaveProperty('keepHostSettings');
    expect(multipart.schema.properties?.keepHostSettings?.default).toBe(true);

    const array = requestBody('/panel/api/server/clientIps').content['application/json'];
    expect(array.schema).toEqual({
      type: 'array',
      items: {
        type: 'object',
        properties: {
          clientEmail: { type: 'string' },
          ips: {
            type: 'array',
            nullable: true,
            items: {
              type: 'object',
              properties: { ip: { type: 'string' }, timestamp: { type: 'integer' } },
              required: ['ip', 'timestamp'],
            },
          },
        },
        required: ['clientEmail', 'ips'],
      },
    });

    const certHash = requestBody('/panel/api/server/getCertHash');
    expect(certHash.required).toBe(true);
    const certSchema = certHash.content['application/x-www-form-urlencoded'].schema;
    expect(certSchema.anyOf?.map((branch) => branch.required)).toEqual([
      ['certFile'],
      ['certContent'],
    ]);
    expect(certSchema.anyOf?.[0].properties?.certFile.pattern).toBe('.*\\S.*');
    expect(certSchema.anyOf?.[0].properties?.certContent).not.toHaveProperty('pattern');
    expect(certSchema.anyOf?.[1].properties?.certFile).not.toHaveProperty('pattern');
    expect(certSchema.anyOf?.[1].properties?.certContent.pattern).toBe('.*\\S.*');

    const routeSchema = requestBody('/panel/api/xray/routeTest').content[
      'application/x-www-form-urlencoded'
    ].schema;
    expect(routeSchema.anyOf?.map((branch) => branch.required)).toEqual([['domain'], ['ip']]);
    expect(routeSchema.anyOf?.[0].properties?.domain?.minLength).toBe(1);
    expect(routeSchema.anyOf?.[0].properties?.ip).not.toHaveProperty('minLength');
    expect(routeSchema.anyOf?.[1].properties?.domain).not.toHaveProperty('minLength');
    expect(routeSchema.anyOf?.[1].properties?.ip?.minLength).toBe(1);

    const bulkAttach = requestBody('/panel/api/clients/bulkAttach').content['application/json'];
    expect(bulkAttach.schema.properties?.emails?.items).toEqual({ type: 'string' });

    expect(paths['/panel/api/server/updateGeofile'].post).not.toHaveProperty('requestBody');
  });
});
