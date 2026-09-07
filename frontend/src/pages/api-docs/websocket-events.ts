export interface WebSocketEventDoc {
  type: string;
  summary: string;
  payloadSchema: Record<string, unknown>;
  example: {
    type: string;
    payload: unknown;
    time: number;
  };
}

const eventTypes = [
  'status',
  'traffic',
  'client_stats',
  'inbounds',
  'outbounds',
  'nodes',
  'notification',
  'xray_state',
  'invalidate',
] as const;

const timestamp = 1735689600000;
const int64 = { type: 'integer', format: 'int64' };
const stringArray = { type: 'array', items: { type: 'string' } };
const stringArrayMap = { type: 'object', additionalProperties: stringArray };
const timestampMap = { type: 'object', additionalProperties: int64 };
const currentTotal = {
  type: 'object',
  required: ['current', 'total'],
  properties: { current: int64, total: int64 },
};

const statusPayloadSchema = {
  type: 'object',
  required: [
    'cpu',
    'cpuCores',
    'logicalPro',
    'cpuSpeedMhz',
    'mem',
    'swap',
    'disk',
    'diskIO',
    'diskTraffic',
    'xray',
    'amneziawg',
    'panelVersion',
    'panelGuid',
    'uptime',
    'loads',
    'tcpCount',
    'udpCount',
    'netIO',
    'netTraffic',
    'publicIP',
    'appStats',
  ],
  properties: {
    cpu: { type: 'number' },
    cpuCores: { type: 'integer' },
    logicalPro: { type: 'integer' },
    cpuSpeedMhz: { type: 'number' },
    mem: currentTotal,
    swap: currentTotal,
    disk: currentTotal,
    diskIO: {
      type: 'object',
      required: ['read', 'write'],
      properties: { read: int64, write: int64 },
    },
    diskTraffic: {
      type: 'object',
      required: ['read', 'write'],
      properties: { read: int64, write: int64 },
    },
    xray: {
      type: 'object',
      required: ['state', 'errorMsg', 'version'],
      properties: {
        state: { type: 'string', enum: ['running', 'stop', 'error'] },
        errorMsg: { type: 'string' },
        version: { type: 'string' },
      },
    },
    amneziawg: {
      type: 'object',
      required: ['configured', 'running'],
      properties: { configured: { type: 'boolean' }, running: { type: 'boolean' } },
    },
    panelVersion: { type: 'string' },
    panelGuid: { type: 'string' },
    uptime: int64,
    loads: { type: 'array', nullable: true, items: { type: 'number' } },
    tcpCount: { type: 'integer' },
    udpCount: { type: 'integer' },
    netIO: {
      type: 'object',
      required: ['up', 'down', 'pktUp', 'pktDown'],
      properties: { up: int64, down: int64, pktUp: int64, pktDown: int64 },
    },
    netTraffic: {
      type: 'object',
      required: ['sent', 'recv', 'pktSent', 'pktRecv'],
      properties: { sent: int64, recv: int64, pktSent: int64, pktRecv: int64 },
    },
    publicIP: {
      type: 'object',
      required: ['ipv4', 'ipv6'],
      properties: { ipv4: { type: 'string' }, ipv6: { type: 'string' } },
    },
    appStats: {
      type: 'object',
      required: ['threads', 'mem', 'uptime'],
      properties: { threads: { type: 'integer' }, mem: int64, uptime: int64 },
    },
  },
};

const trafficPayloadSchema = {
  type: 'object',
  required: ['onlineClients', 'onlineByGuid', 'activeInbounds', 'lastOnlineMap'],
  properties: {
    traffics: { type: 'array', items: { $ref: '#/components/schemas/Traffic' } },
    clientTraffics: { type: 'array', items: { $ref: '#/components/schemas/ClientTraffic' } },
    nodeTraffics: {
      type: 'array',
      nullable: true,
      items: { $ref: '#/components/schemas/Traffic' },
    },
    onlineClients: stringArray,
    onlineByGuid: stringArrayMap,
    activeInbounds: stringArrayMap,
    lastOnlineMap: timestampMap,
  },
  oneOf: [{ required: ['traffics', 'clientTraffics'] }, { required: ['nodeTraffics'] }],
};

const clientStatsPayloadSchema = {
  type: 'object',
  required: ['snapshot'],
  properties: {
    snapshot: { type: 'boolean' },
    clients: { type: 'array', items: { $ref: '#/components/schemas/ClientTraffic' } },
    inbounds: {
      type: 'array',
      items: { $ref: '#/components/schemas/InboundTrafficSummary' },
    },
  },
  anyOf: [{ required: ['clients'] }, { required: ['inbounds'] }],
};

export const websocketEnvelopeSchema = {
  type: 'object',
  required: ['type', 'payload', 'time'],
  properties: {
    type: { type: 'string', enum: eventTypes },
    payload: { description: 'Shape is selected by type; see x-websocket-events on GET /ws.' },
    time: {
      type: 'integer',
      format: 'int64',
      description: 'Server emission time in Unix milliseconds.',
    },
  },
};

export function buildWebSocketEvents(
  examples: Record<string, unknown>,
): readonly WebSocketEventDoc[] {
  return [
    {
      type: 'status',
      summary:
        'Server health snapshot pushed every two seconds; same payload as server/status obj.',
      payloadSchema: statusPayloadSchema,
      example: {
        type: 'status',
        payload: {
          cpu: 12.5,
          cpuCores: 4,
          logicalPro: 8,
          cpuSpeedMhz: 3200,
          mem: { current: 2147483648, total: 8589934592 },
          swap: { current: 0, total: 2147483648 },
          disk: { current: 53687091200, total: 107374182400 },
          diskIO: { read: 1048576, write: 2097152 },
          diskTraffic: { read: 4096, write: 8192 },
          xray: { state: 'running', errorMsg: '', version: '25.10.31' },
          amneziawg: { configured: false, running: false },
          panelVersion: 'v3.x.x',
          panelGuid: 'panel-guid',
          uptime: 86400,
          loads: [0.1, 0.2, 0.3],
          tcpCount: 24,
          udpCount: 8,
          netIO: { up: 1048576, down: 2097152, pktUp: 100, pktDown: 200 },
          netTraffic: { sent: 4096, recv: 8192, pktSent: 10, pktRecv: 20 },
          publicIP: { ipv4: '192.0.2.1', ipv6: '2001:db8::1' },
          appStats: { threads: 16, mem: 67108864, uptime: 3600 },
        },
        time: timestamp,
      },
    },
    {
      type: 'traffic',
      summary:
        'Live traffic deltas plus online, per-node and last-online maps. Local polls send traffics/clientTraffics; node polls send nodeTraffics.',
      payloadSchema: trafficPayloadSchema,
      example: {
        type: 'traffic',
        payload: {
          traffics: [examples.Traffic],
          clientTraffics: [examples.ClientTraffic],
          onlineClients: ['alice@example.com'],
          onlineByGuid: { 'panel-guid': ['alice@example.com'] },
          activeInbounds: { 'panel-guid': ['inbound-443'] },
          lastOnlineMap: { 'alice@example.com': timestamp },
        },
        time: timestamp,
      },
    },
    {
      type: 'client_stats',
      summary:
        'Absolute client counters and/or inbound summaries; snapshot says whether clients is complete or only recently active rows.',
      payloadSchema: clientStatsPayloadSchema,
      example: {
        type: 'client_stats',
        payload: {
          snapshot: true,
          clients: [examples.ClientTraffic],
          inbounds: [examples.InboundTrafficSummary],
        },
        time: timestamp,
      },
    },
    {
      type: 'inbounds',
      summary: 'Full inbound list after an inbound mutation, unless invalidate is used at scale.',
      payloadSchema: { type: 'array', items: { $ref: '#/components/schemas/Inbound' } },
      example: { type: 'inbounds', payload: [examples.Inbound], time: timestamp },
    },
    {
      type: 'outbounds',
      summary: 'Current outbound traffic rows after the periodic traffic collection.',
      payloadSchema: {
        type: 'array',
        items: { $ref: '#/components/schemas/OutboundTraffics' },
      },
      example: { type: 'outbounds', payload: [examples.OutboundTraffics], time: timestamp },
    },
    {
      type: 'nodes',
      summary: 'Current node tree after the heartbeat probe cycle.',
      payloadSchema: { type: 'array', items: { $ref: '#/components/schemas/NodeView' } },
      example: { type: 'nodes', payload: [examples.NodeView], time: timestamp },
    },
    {
      type: 'notification',
      summary: 'An in-panel notification emitted by server actions.',
      payloadSchema: {
        type: 'object',
        required: ['title', 'message', 'level'],
        properties: {
          title: { type: 'string' },
          message: { type: 'string' },
          level: { type: 'string', enum: ['success', 'warning'] },
        },
      },
      example: {
        type: 'notification',
        payload: {
          title: 'Xray service restarted',
          message: 'Xray service has been restarted successfully',
          level: 'success',
        },
        time: timestamp,
      },
    },
    {
      type: 'xray_state',
      summary: 'Xray process state change after a stop, restart or error.',
      payloadSchema: {
        type: 'object',
        required: ['state', 'errorMsg'],
        properties: {
          state: { type: 'string', enum: ['running', 'stop', 'error'] },
          errorMsg: { type: 'string' },
        },
      },
      example: {
        type: 'xray_state',
        payload: { state: 'running', errorMsg: '' },
        time: timestamp,
      },
    },
    {
      type: 'invalidate',
      summary:
        'Requests a REST re-fetch. clients is an invalidate payload type, not a top-level event.',
      payloadSchema: {
        type: 'object',
        required: ['type'],
        properties: {
          type: {
            type: 'string',
            enum: [
              'status',
              'traffic',
              'client_stats',
              'inbounds',
              'outbounds',
              'nodes',
              'notification',
              'xray_state',
              'clients',
            ],
          },
        },
      },
      example: { type: 'invalidate', payload: { type: 'inbounds' }, time: timestamp },
    },
  ];
}
