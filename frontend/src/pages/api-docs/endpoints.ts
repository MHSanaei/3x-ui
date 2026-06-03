export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'WS';
export type ParamLocation =
  | 'path'
  | 'query'
  | 'header'
  | 'body'
  | 'body (form)'
  | 'body (json)'
  | 'body (multipart)';
export type ParamType =
  | 'string'
  | 'integer'
  | 'integer[]'
  | 'number'
  | 'boolean'
  | 'object'
  | 'object[]'
  | 'array'
  | 'file';

export interface EndpointParam {
  name: string;
  in: ParamLocation;
  type: ParamType;
  desc?: string;
  optional?: boolean;
  defaultValue?: string | number | boolean;
}

export interface Endpoint {
  method: HttpMethod;
  path: string;
  summary: string;
  description?: string;
  deprecated?: boolean;
  params?: EndpointParam[];
  body?: string;
  response?: string;
  errorResponse?: string;
  errorStatus?: number;
}

export interface SubscriptionHeader {
  name: string;
  desc: string;
}

export interface Section {
  id: string;
  title: string;
  description?: string;
  subHeader?: SubscriptionHeader[];
  endpoints: Endpoint[];
}

export const sections: readonly Section[] = [
  {
    id: 'authentication',
    title: 'Authentication',
    description:
      'Two authentication modes are supported. UI sessions use a cookie set by the login endpoint. Programmatic clients (bots, scripts, remote panels) authenticate with a Bearer token taken from Settings → Security → API Token. Both work for every endpoint under /panel/api/*.',
    endpoints: [
      {
        method: 'POST',
        path: '/login',
        summary: 'Authenticate with username + password and receive a session cookie. Required before any cookie-based API call.',
        params: [
          { name: 'username', in: 'body', type: 'string', desc: 'Panel admin username.' },
          { name: 'password', in: 'body', type: 'string', desc: 'Panel admin password.' },
          { name: 'twoFactorCode', in: 'body', type: 'string', desc: 'OTP code when 2FA is enabled. Omit otherwise.' },
        ],
        body: '{\n  "username": "admin",\n  "password": "admin",\n  "twoFactorCode": "123456"\n}',
        response:
          '{\n  "success": true,\n  "msg": "Logged in successfully"\n}',
        errorResponse:
          '{\n  "success": false,\n  "msg": "Wrong username or password"\n}',
      },
      {
        method: 'POST',
        path: '/logout',
        summary: 'Clear the session cookie. Requires the CSRF header for browser sessions.',
        response: '{\n  "success": true\n}',
      },
      {
        method: 'GET',
        path: '/csrf-token',
        summary: 'Mint a CSRF token for the current session. The SPA replays it in the X-CSRF-Token header on unsafe requests. Bearer-token callers can skip this — the middleware short-circuits CSRF for authenticated API requests.',
        response:
          '{\n  "success": true,\n  "obj": "csrf-token-string"\n}',
      },
      {
        method: 'POST',
        path: '/getTwoFactorEnable',
        summary: 'Returns whether 2FA is enabled on the panel — used by the login page to decide whether to show the OTP field.',
        response: '{\n  "success": true,\n  "obj": false\n}',
      },
    ],
  },

  {
    id: 'inbounds',
    title: 'Inbounds',
    description:
      'Manage inbound configurations and their clients. All endpoints live under /panel/api/inbounds and require a logged-in session or Bearer token. Link-generating endpoints honour forwarded headers only when the request comes from a configured trusted proxy.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/inbounds/list',
        summary: 'List every inbound owned by the authenticated user, including each inbound’s clientStats traffic counters. settings, streamSettings, and sniffing are returned as nested JSON objects (no escaped strings); legacy callers that send them back as JSON-encoded strings are still accepted on write.',
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "userId": 1,\n      "up": 0,\n      "down": 0,\n      "total": 0,\n      "remark": "VLESS-443",\n      "enable": true,\n      "expiryTime": 0,\n      "listen": "",\n      "port": 443,\n      "protocol": "vless",\n      "settings": {\n        "clients": [],\n        "decryption": "none"\n      },\n      "streamSettings": {\n        "network": "tcp",\n        "security": "reality",\n        "realitySettings": { "show": false, "dest": "..." }\n      },\n      "tag": "inbound-443",\n      "sniffing": {\n        "enabled": true,\n        "destOverride": ["http", "tls"]\n      },\n      "clientStats": []\n    }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/list/slim',
        summary: 'Same shape as /list but with settings.clients[] stripped down to {email, enable, comment} and ClientStats not enriched with UUID/SubId. Use this for list pages; fetch /get/:id when you need the full per-client payload (uuid, password, flow, ...).',
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "userId": 1,\n      "remark": "VLESS-443",\n      "settings": {\n        "clients": [\n          { "email": "alice", "enable": true }\n        ],\n        "decryption": "none"\n      },\n      "clientStats": []\n    }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/options',
        summary: 'Lightweight picker projection of the authenticated user’s inbounds. Returns only id, remark, protocol, port, and a server-computed tlsFlowCapable flag (true for VLESS / port-fallback on TCP with tls or reality). Use this for dropdowns and attach pickers — it skips settings, streamSettings, and clientStats so the payload stays small even on panels with thousands of clients.',
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "remark": "VLESS-443",\n      "protocol": "vless",\n      "port": 443,\n      "tlsFlowCapable": true\n    }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/get/:id',
        summary: 'Fetch a single inbound by numeric ID.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/add',
        summary: 'Create a new inbound. Send the full inbound payload (protocol, port, settings, streamSettings, sniffing, remark, expiryTime, total, enable). settings, streamSettings, and sniffing may be sent as nested JSON objects (preferred) or as JSON-encoded strings (legacy).',
        body:
          '{\n  "enable": true,\n  "remark": "VLESS-443",\n  "listen": "",\n  "port": 443,\n  "protocol": "vless",\n  "expiryTime": 0,\n  "total": 0,\n  "settings": {\n    "clients": [{ "id": "...", "email": "user1" }],\n    "decryption": "none",\n    "fallbacks": []\n  },\n  "streamSettings": {\n    "network": "tcp",\n    "security": "reality",\n    "realitySettings": { "show": false, "dest": "..." }\n  },\n  "sniffing": {\n    "enabled": true,\n    "destOverride": ["http", "tls"]\n  }\n}',
        errorResponse:
          '{\n  "success": false,\n  "msg": "Port 443 is already in use"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/del/:id',
        summary: 'Delete an inbound by ID. Also removes its associated client stats rows.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/bulkDel',
        summary: 'Delete many inbounds in one call. Processes the list sequentially; failures are reported per id and the rest still proceed. Restarts xray at most once.',
        body: '{\n  "ids": [1, 2, 3]\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "deleted": 2,\n    "skipped": [\n      { "id": 3, "reason": "..." }\n    ]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/update/:id',
        summary: 'Replace an inbound’s configuration. Body shape mirrors /add. Heavy on inbounds with thousands of clients — prefer /setEnable for enable-only flips.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/setEnable/:id',
        summary: 'Toggle only the enable flag without serialising the whole settings JSON. Recommended for UI switches on large inbounds.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
        body: '{\n  "enable": false\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/resetTraffic',
        summary: 'Zero out upload + download counters for a single inbound. Does not touch per-client counters.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/delAllClients',
        summary: 'Remove every client attached to a single inbound while keeping the inbound itself. Collects emails from settings.clients[] and feeds them into the optimized bulk-delete path (runtime user removal + traffic-row cleanup + SyncInbound). Destructive and cannot be undone.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
        response: '{\n  "success": true,\n  "obj": {\n    "deleted": 12\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/resetAllTraffics',
        summary: 'Reset upload + download counters on every inbound. Destructive — accounting history is lost.',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/import',
        summary: 'Bulk-import an inbound from a JSON blob (e.g. one exported via the UI). The body uses form encoding with a single "data" field.',
        params: [
          { name: 'data', in: 'body (form)', type: 'string', desc: 'JSON-encoded inbound payload.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/:id/fallbacks',
        summary: 'List the fallback rules attached to a master VLESS/Trojan TCP-TLS inbound. Each rule links one child inbound (the dest) to optional SNI/ALPN/path/dest/xver match criteria. When dest is empty the child inbound\'s listen+port is used.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Master inbound ID.' },
        ],
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "masterId": 10,\n      "childId": 11,\n      "name": "",\n      "alpn": "",\n      "path": "/vlws",\n      "dest": "",\n      "xver": 2,\n      "sortOrder": 0\n    }\n  ]\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/fallbacks',
        summary: 'Replace the entire fallback list for a master inbound. Body is JSON. Triggers an Xray restart.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Master inbound ID.' },
          { name: 'fallbacks', in: 'body (json)', type: 'object[]', desc: 'Array of {childId, name, alpn, path, dest, xver, sortOrder} entries. Leave dest empty to auto-resolve from the child inbound\'s listen+port; set it (e.g. "8443", "127.0.0.1:8443", "/dev/shm/x.sock") to override.' },
        ],
        body: '{\n  "fallbacks": [\n    { "childId": 11, "path": "/vlws", "xver": 2 },\n    { "childId": 12, "alpn": "h2", "dest": "8443" }\n  ]\n}',
        response: '{\n  "success": true,\n  "msg": "Inbound updated"\n}',
      },
    ],
  },

  {
    id: 'server',
    title: 'Server',
    description:
      'System status, log retrieval, certificate generators, Xray binary management, and backup/restore. All under /panel/api/server.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/server/status',
        summary: 'Real-time machine snapshot: CPU, memory, swap, disk, network IO, load averages, open connections, Xray state. Cached and refreshed every 2 seconds in the background.',
        response: '{\n  "success": true,\n  "obj": {\n    "cpu": 12.5,\n    "mem": { "current": 2147483648, "total": 8589934592 },\n    "swap": { "current": 0, "total": 4294967296 },\n    "disk": { "current": 53687091200, "total": 268435456000 },\n    "netIO": { "up": 1073741824, "down": 2147483648 },\n    "xray": { "state": "running", "version": "v25.10.31" },\n    "tcpCount": 42,\n    "load": { "load1": 0.5, "load5": 0.3, "load15": 0.2 }\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/cpuHistory/:bucket',
        summary: 'Legacy: aggregated CPU history. Use /history/cpu/:bucket instead — same data with a uniform {t, v} shape.',
        params: [
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/server/history/:metric/:bucket',
        summary: 'Aggregated time-series for one metric. Returns an array of {t, v} samples covering the last ~6 hours.',
        params: [
          { name: 'metric', in: 'path', type: 'string', desc: 'cpu | mem | netUp | netDown | online | load1 | load5 | load15.' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
        response: '{\n  "success": true,\n  "obj": [\n    { "t": 1700000000, "v": 12.5 },\n    { "t": 1700000002, "v": 13.1 }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/xrayMetricsState',
        summary: 'Xray runtime metrics state — whether the xray config has a `metrics` block, which expvar keys are flowing, and the current snapshot values for each. Returns an empty state when metrics are not configured.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/xrayMetricsHistory/:metric/:bucket',
        summary: 'Time-series history for one Xray runtime metric over the last ~6 hours. Same {t, v} shape as /history/:metric/:bucket.',
        params: [
          { name: 'metric', in: 'path', type: 'string', desc: 'xrAlloc | xrSys | xrHeapObjects | xrNumGC | xrPauseNs.' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/server/xrayObservatory',
        summary: 'Latest snapshot from the Xray observatory — per-outbound latency, health status, and last-probe time. Only populated when the Xray config has an observatory configured.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/xrayObservatoryHistory/:tag/:bucket',
        summary: 'Time-series of observatory probe results for one outbound tag. Same {t, v} shape as the other history endpoints.',
        params: [
          { name: 'tag', in: 'path', type: 'string', desc: 'Outbound tag from the observatory config.' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/server/getXrayVersion',
        summary: 'List Xray binary versions available for install on this host.',
        response: '{\n  "success": true,\n  "obj": ["v25.10.31", "v25.9.15", "v25.8.1"]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getPanelUpdateInfo',
        summary: 'Check whether a newer 3x-ui release is available on GitHub.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getConfigJson',
        summary: 'Return the assembled Xray config that\u2019s currently running on this host.',
        response: '{\n  "success": true,\n  "obj": {\n    "log": { "loglevel": "warning" },\n    "inbounds": [...],\n    "outbounds": [...],\n    "routing": { "rules": [...] }\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getDb',
        summary: 'Stream the SQLite database file as an attachment. Use as a manual backup.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewUUID',
        summary: 'Generate a fresh UUID v4. Convenience helper for client IDs.',
        response: '{\n  "success": true,\n  "obj": "550e8400-e29b-41d4-a716-446655440000"\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getWebCertFiles',
        summary: 'Return this panel\'s own web TLS certificate and key file paths. The central panel calls it on a node (via the node API token) so "Set Cert from Panel" fills a node-assigned inbound with paths that exist on the node.',
        response: '{\n  "success": true,\n  "obj": {\n    "webCertFile": "/root/cert/example.com/fullchain.pem",\n    "webKeyFile": "/root/cert/example.com/privkey.pem"\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewX25519Cert',
        summary: 'Generate a new X25519 keypair for Reality.',
        response: '{\n  "success": true,\n  "obj": {\n    "privateKey": "uN9qLfV3zH8w...",\n    "publicKey": "5v8xPqR2sM7k..."\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewmldsa65',
        summary: 'Generate a new ML-DSA-65 keypair (post-quantum signature). Returns {privateKey, publicKey, seed}.',
        response: '{\n  "success": true,\n  "obj": {\n    "privateKey": "mdsa65priv...",\n    "publicKey": "mdsa65pub...",\n    "seed": "random-seed..."\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewmlkem768',
        summary: 'Generate a new ML-KEM-768 keypair (post-quantum KEM). Returns {clientKey, serverKey}.',
        response: '{\n  "success": true,\n  "obj": {\n    "clientKey": "mlkem768-client...",\n    "serverKey": "mlkem768-server..."\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewVlessEnc',
        summary: 'Generate VLESS encryption auth options. Returns an auths array each with id, label, encryption, and decryption fields.',
        response: '{\n  "success": true,\n  "obj": {\n    "auths": [\n      { "id": 0, "label": "Auth #0", "encryption": "aes-256-gcm", "decryption": "" }\n    ]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/stopXrayService',
        summary: 'Stop the Xray binary. All proxies go offline immediately.',
        errorResponse:
          '{\n  "success": false,\n  "msg": "Xray is not running"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/restartXrayService',
        summary: 'Reload Xray with the current config. Typically required after structural inbound or routing changes.',
        errorResponse:
          '{\n  "success": false,\n  "msg": "Xray config is invalid: ..."\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/installXray/:version',
        summary: 'Download and install the specified Xray version. Pass "latest" for the newest release.',
        params: [
          { name: 'version', in: 'path', type: 'string', desc: 'Xray tag (e.g. v25.10.31) or "latest".' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/server/updatePanel',
        summary: 'Self-update the panel to the latest version. The server restarts on success.',
      },
      {
        method: 'POST',
        path: '/panel/api/server/updateGeofile',
        summary: 'Refresh the default GeoIP / GeoSite data files. Body can include a fileName, or use the /:fileName variant.',
        params: [
          { name: 'fileName', in: 'body (form)', type: 'string', desc: 'Filename to update (e.g. geoip.dat, geosite.dat). Omit to update all defaults.' },
        ],
        body: 'fileName=geoip.dat',
      },
      {
        method: 'POST',
        path: '/panel/api/server/updateGeofile/:fileName',
        summary: 'Refresh a single Geo file by filename (e.g. geoip.dat, geosite.dat).',
        params: [
          { name: 'fileName', in: 'path', type: 'string', desc: 'Filename of the data file to refresh.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/server/logs/:count',
        summary: 'Return the last N lines of the panel\u2019s own log.',
        params: [
          { name: 'count', in: 'path', type: 'number', desc: 'Number of trailing log lines.' },
        ],
        body: '{\n  "level": "info",\n  "syslog": false\n}',
        response: '{\n  "success": true,\n  "obj": "2025/01/01 12:00:00 [INFO] Server started\\n2025/01/01 12:00:01 [INFO] Xray is running"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/xraylogs/:count',
        summary: 'Return the last N lines of the Xray process log.',
        params: [
          { name: 'count', in: 'path', type: 'number', desc: 'Number of trailing log lines.' },
          { name: 'filter', in: 'body (form)', type: 'string', desc: 'Keyword filter — only lines containing this string.' },
          { name: 'showDirect', in: 'body (form)', type: 'string', desc: '"true" to include direct (freedom) traffic lines.' },
          { name: 'showBlocked', in: 'body (form)', type: 'string', desc: '"true" to include blocked (blackhole) traffic lines.' },
          { name: 'showProxy', in: 'body (form)', type: 'string', desc: '"true" to include proxy traffic lines.' },
        ],
        body: 'filter=error&showDirect=false&showBlocked=true&showProxy=true',
        response: '{\n  "success": true,\n  "obj": "2025/01/01 12:00:00 rejected  vless  proxy  example.com  reason: no valid user\\n2025/01/01 12:00:01 direct  freedom  ok"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/importDB',
        summary: 'Restore the panel DB from an uploaded SQLite file (multipart form, field name "db"). The panel restarts after restore. Destructive.',
        params: [
          { name: 'db', in: 'body (multipart)', type: 'file', desc: 'SQLite database file to upload.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/server/getNewEchCert',
        summary: 'Generate a new ECH (Encrypted Client Hello) keypair and config list for the given SNI.',
        params: [
          { name: 'sni', in: 'body (form)', type: 'string', desc: 'Server Name Indication to generate the ECH config for.' },
        ],
        body: 'sni=example.com',
        response: '{\n  "success": true,\n  "obj": {\n    "echKeySet": "...",\n    "echServerKeys": [...],\n    "echConfigList": "..."\n  }\n}',
      },
    ],
  },

  {
    id: 'clients',
    title: 'Clients',
    description:
      'Manage clients as first-class entities that can be attached to one or more inbounds. A single client row drives the settings.clients entry in every inbound it belongs to. Endpoints live under /panel/api/clients.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/clients/list',
        summary: 'List every client with its attached inbound IDs and traffic record. The reverse field, if set, is returned as a nested JSON object (legacy JSON-encoded-string form is still accepted on write).',
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "email": "alice@example.com",\n      "subId": "abcd1234",\n      "uuid": "...",\n      "totalGB": 53687091200,\n      "expiryTime": 1735689600000,\n      "enable": true,\n      "reverse": null,\n      "inboundIds": [3, 5],\n      "traffic": { "up": 1024, "down": 4096, "enable": true }\n    }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/list/paged',
        summary: 'Filter, sort, and paginate clients on the server. Each item is a slim row (no uuid/password/auth/flow/security/reverse/tgId) so the clients page can ship 25-ish rows in a few KB instead of the full table. The response also includes a summary computed across the full DB row set so dashboard counters stay stable as the user paginates or filters. Page size capped at 200; fetch /get/:email to obtain the full per-client payload for an edit/info modal.',
        params: [
          { name: 'page', in: 'query', type: 'number', desc: '1-indexed page number. Defaults to 1.' },
          { name: 'pageSize', in: 'query', type: 'number', desc: 'Rows per page. Defaults to 25, capped at 200.' },
          { name: 'search', in: 'query', type: 'string', desc: 'Case-insensitive substring match on email / subId / comment.' },
          { name: 'filter', in: 'query', type: 'string', desc: 'Status bucket: online | active | deactive | depleted | expiring.' },
          { name: 'protocol', in: 'query', type: 'string', desc: 'Match clients attached to at least one inbound of this protocol (vless, vmess, trojan, shadowsocks, ...).' },
          { name: 'sort', in: 'query', type: 'string', desc: 'Sort key: enable | email | inboundIds | traffic | remaining | expiryTime.' },
          { name: 'order', in: 'query', type: 'string', desc: 'ascend or descend.' },
        ],
        response:
          '{\n  "success": true,\n  "obj": {\n    "items": [\n      {\n        "email": "alice@example.com",\n        "subId": "abcd1234",\n        "enable": true,\n        "totalGB": 53687091200,\n        "expiryTime": 1735689600000,\n        "limitIp": 0,\n        "reset": 0,\n        "inboundIds": [3, 5],\n        "traffic": { "up": 1024, "down": 4096, "enable": true },\n        "createdAt": 1735000000000,\n        "updatedAt": 1735100000000\n      }\n    ],\n    "total": 2000,\n    "filtered": 47,\n    "page": 1,\n    "pageSize": 25,\n    "summary": {\n      "total": 2000,\n      "active": 1850,\n      "online": ["alice@example.com"],\n      "depleted": [],\n      "expiring": [],\n      "deactive": []\n    }\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/get/:email',
        summary: 'Fetch one client by email, including the inbound IDs it is attached to.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique identifier).' },
        ],
        response:
          '{\n  "success": true,\n  "obj": {\n    "client": { "id": 1, "email": "alice@example.com", ... },\n    "inboundIds": [3, 5]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/add',
        summary: 'Create a new client and attach it to one or more inbounds in a single call. Body is JSON. Per-protocol secrets (UUID for VLESS/VMess, password for Trojan/Shadowsocks, auth for Hysteria) are generated server-side when omitted, so callers can send only the universal fields.',
        params: [
          { name: 'client', in: 'body (json)', type: 'object', desc: 'Client fields: email, subId, id (uuid), password, auth, flow, totalGB, expiryTime, limitIp, tgId (numeric Telegram user ID, 0 = none), comment, enable.' },
          { name: 'inboundIds', in: 'body (json)', type: 'integer[]', desc: 'Inbound IDs to attach the client to. At least one required.' },
        ],
        body: '{\n  "client": {\n    "email": "alice@example.com",\n    "totalGB": 53687091200,\n    "expiryTime": 1735689600000,\n    "tgId": 0,\n    "limitIp": 0,\n    "enable": true\n  },\n  "inboundIds": [3, 5]\n}',
        response: '{\n  "success": true,\n  "msg": "Client added"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/update/:email',
        summary: 'Update an existing client by email. Changes propagate to every attached inbound. Body is the JSON client payload — supply the full set of fields you want to keep (the server replaces the row, it does not patch).',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Current client email (unique identifier).' },
        ],
        body: '{\n  "email": "alice@example.com",\n  "totalGB": 107374182400,\n  "expiryTime": 1767225600000,\n  "tgId": 123456789,\n  "enable": true\n}',
        response: '{\n  "success": true,\n  "msg": "Client updated"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/del/:email',
        summary: 'Delete a client by email. Removes it from every attached inbound and drops its traffic record unless keepTraffic=1 is passed.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique identifier).' },
          { name: 'keepTraffic', in: 'query', type: 'integer', desc: 'Pass 1 to retain the xray_client_traffic row after deletion.' },
        ],
        response: '{\n  "success": true,\n  "msg": "Client deleted"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/:email/attach',
        summary: 'Attach an existing client to one or more additional inbounds. Body is JSON.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique identifier).' },
          { name: 'inboundIds', in: 'body (json)', type: 'integer[]', desc: 'Inbound IDs to attach.' },
        ],
        body: '{\n  "inboundIds": [7, 9]\n}',
        response: '{\n  "success": true\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/:email/detach',
        summary: 'Detach a client from one or more inbounds without deleting the client.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique identifier).' },
          { name: 'inboundIds', in: 'body (json)', type: 'integer[]', desc: 'Inbound IDs to detach.' },
        ],
        body: '{\n  "inboundIds": [5]\n}',
        response: '{\n  "success": true\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/resetAllTraffics',
        summary: 'Reset the up/down counters for every client globally. Quotas and expiry are not affected. Triggers an Xray restart if any counter actually moved.',
        response: '{\n  "success": true\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/delDepleted',
        summary: 'Delete every client whose traffic quota is exhausted (used >= total, when reset is disabled) or whose expiry has passed. Returns the deleted count and triggers an Xray restart when any client was on a running inbound.',
        response: '{\n  "success": true,\n  "obj": {\n    "deleted": 0\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkAdjust',
        summary: 'Shift expiry and/or traffic quota for many clients in one call. addDays/addBytes may be negative. Clients with unlimited expiry (expiryTime=0) or unlimited traffic (totalGB=0) are skipped for the corresponding field — bulk extend never converts unlimited to limited. Returns the adjusted count and per-email skip reasons.',
        body: '{\n  "emails": ["alice", "bob"],\n  "addDays": 30,\n  "addBytes": 53687091200\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "adjusted": 2,\n    "skipped": [\n      { "email": "carol", "reason": "unlimited expiry" }\n    ]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkDel',
        summary: 'Delete many clients in one call. The server processes the list sequentially so each delete sees the committed state of the previous one — avoids the race the per-email fan-out had on the panel side. Pass keepTraffic=true to retain the xray_client_traffic rows after deletion.',
        body: '{\n  "emails": ["alice", "bob"],\n  "keepTraffic": false\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "deleted": 2,\n    "skipped": [\n      { "email": "carol", "reason": "client not found" }\n    ]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkCreate',
        summary: 'Create many clients in one call. Body is a JSON array of {client, inboundIds} payloads — the same shape /add accepts. Items are processed sequentially; per-email skip reasons are returned for items that fail (e.g., duplicate email). Triggers a single Xray restart at the end if any inbound was running.',
        body: '[\n  {\n    "client": {\n      "email": "alice@example.com",\n      "totalGB": 53687091200,\n      "expiryTime": 0,\n      "enable": true\n    },\n    "inboundIds": [7]\n  },\n  {\n    "client": {\n      "email": "bob@example.com",\n      "totalGB": 53687091200,\n      "expiryTime": 0,\n      "enable": true\n    },\n    "inboundIds": [7, 9]\n  }\n]',
        response: '{\n  "success": true,\n  "obj": {\n    "created": 2,\n    "skipped": [\n      { "email": "alice@example.com", "reason": "email already in use" }\n    ]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/groups/bulkAdd',
        summary: 'Add many clients to a group in one call. Updates clients.group_name and patches the matching client entry inside every owning inbound\'s settings JSON in a single transaction. If the group name does not yet exist (in client_groups or as a derived label), it is auto-created as a persistent group. To clear the group label, use /groups/bulkRemove instead.',
        body: '{\n  "emails": ["alice", "bob"],\n  "group": "customer-a"\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "affected": 2\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/groups/bulkRemove',
        summary: 'Clear the group label on many clients in one call. Inverse of /groups/bulkAdd. Clients themselves are kept — only the group label is cleared from clients.group_name and from each owning inbound\'s settings JSON. Groups become empty if all their members are removed.',
        body: '{\n  "emails": ["alice", "bob"]\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "affected": 2\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkAttach',
        summary: 'Attach many existing clients to many inbounds in one call. Each client keeps its identity (email/UUID/password/subId) and a shared traffic row; all clients are added to a target inbound in a single AddInboundClient call. Clients already present on a target are reported under skipped. Returns per-email attached/skipped/errors lists and triggers a single Xray restart if any target inbound was running.',
        params: [
          { name: 'emails', in: 'body (json)', type: 'array', desc: 'Emails of existing clients to attach.' },
          { name: 'inboundIds', in: 'body (json)', type: 'integer[]', desc: 'Target inbound IDs to attach every client to.' },
        ],
        body: '{\n  "emails": ["alice", "bob"],\n  "inboundIds": [7, 9]\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "attached": ["alice", "bob"],\n    "skipped": ["bob"],\n    "errors": []\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkDetach',
        summary: 'Mirror of bulkAttach: detach many existing clients from many inbounds in one call. For each email, intersects the client\'s current inbounds with the requested set and detaches from those only; (email, inbound) pairs where the client is not currently attached are silently no-ops. Emails not attached to any of the requested inbounds are reported under skipped. Client records are kept even if they become orphaned — use bulkDel for full removal. Returns per-email detached/skipped/errors lists and triggers a single Xray restart if any target inbound was running.',
        params: [
          { name: 'emails', in: 'body (json)', type: 'array', desc: 'Emails of existing clients to detach.' },
          { name: 'inboundIds', in: 'body (json)', type: 'integer[]', desc: 'Inbound IDs to detach the clients from.' },
        ],
        body: '{\n  "emails": ["alice", "bob"],\n  "inboundIds": [7, 9]\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "detached": ["alice", "bob"],\n    "skipped": [],\n    "errors": []\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/bulkResetTraffic',
        summary: 'Zero up/down counters for many clients in one call. Loops the single-reset path so each client is re-enabled across its attached inbounds and pushed to Xray/remote nodes. Returns the count of successfully reset clients.',
        body: '{\n  "emails": ["alice", "bob"]\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "affected": 2\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/groups',
        summary: 'List all client groups with their member counts. Merges persisted groups (rows in client_groups, including empty placeholders) with the distinct group_name values currently set on clients. Sorted alphabetically (case-insensitive).',
        response: '{\n  "success": true,\n  "obj": [\n    { "name": "customer-a", "clientCount": 5 },\n    { "name": "internal", "clientCount": 0 }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/groups/:name/emails',
        summary: 'Return just the email list of clients that currently belong to the given group. Useful for fanning a single bulk action over an entire group without round-tripping the full client list.',
        params: [
          { name: 'name', in: 'path', type: 'string', desc: 'Group name (URL-encoded).' },
        ],
        response: '{\n  "success": true,\n  "obj": ["alice", "bob", "carol"]\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/groups/create',
        summary: 'Create a new empty (placeholder) group. The group becomes selectable in client forms and the filter drawer even before any client is added to it. Errors if a group with the same name already exists.',
        body: '{\n  "name": "customer-a"\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "name": "customer-a"\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/groups/rename',
        summary: 'Rename a group. The new name is applied to the client_groups row AND propagated to every matching client (both clients.group_name and the client entry inside every owning inbound\'s settings JSON) in a single transaction. Returns the number of clients whose label was updated.',
        body: '{\n  "oldName": "customer-a",\n  "newName": "tier-1"\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "affected": 5\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/groups/delete',
        summary: 'Remove a group. Deletes the client_groups row and clears the group label from every matching client (both clients.group_name and the inbound settings JSON). The clients themselves are NOT deleted — use /bulkDel after filtering by group for that. Returns the count of clients whose label was cleared.',
        body: '{\n  "name": "customer-a"\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "affected": 5\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/resetTraffic/:email',
        summary: 'Zero out a single client’s up/down counters. Re-enables the client across every attached inbound and pushes the change to Xray (or the remote node) so depleted users can connect again immediately.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/clients/updateTraffic/:email',
        summary: 'Manually adjust a client’s upload + download counters. Useful for migrations from external accounting systems.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
        body: '{\n  "upload": 1073741824,\n  "download": 5368709120\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/ips/:email',
        summary: 'List source IPs that have connected with the given client’s credentials. Returns an array of "ip (timestamp)" strings.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/clients/clearIps/:email',
        summary: 'Reset the recorded IP list for a client.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/clients/onlines',
        summary: 'List the emails of currently connected clients (last seen within the heartbeat window), deduped across every node.',
        response: '{\n  "success": true,\n  "obj": ["user1", "user2"]\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/onlinesByNode',
        summary: 'Online client emails grouped by the node that reported them. The local panel uses key "0"; each remote node uses its node id. Lets the inbounds page show online status per node instead of merging every node together.',
        response: '{\n  "success": true,\n  "obj": {\n    "0": ["user1"],\n    "3": ["user1", "user2"]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/activeInbounds',
        summary: 'Inbound tags that carried traffic within the heartbeat window, grouped by node (local panel uses key "0"). Pairs with onlinesByNode so the inbounds page only marks a multi-inbound client online on the inbounds it actually used. Nodes that do not report per-inbound activity are absent.',
        response: '{\n  "success": true,\n  "obj": {\n    "0": ["inbound-443", "inbound-8443"]\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/clients/lastOnline',
        summary: 'Map of client email → last-seen unix timestamp.',
        response: '{\n  "success": true,\n  "obj": {\n    "user1": 1700000000,\n    "user2": 1699999000\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/traffic/:email',
        summary: 'Traffic counters for a client identified by email.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique across the panel).' },
        ],
        response: '{\n  "success": true,\n  "obj": {\n    "email": "user1",\n    "up": 1048576,\n    "down": 2097152,\n    "total": 10737418240,\n    "expiryTime": 1735689600000\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/subLinks/:subId',
        summary:
          'Return every protocol URL (vless://, vmess://, trojan://, ss://, hysteria://, hy2://) for clients matching the subscription ID. Same result set as /sub/<subId>, but as a JSON array — no base64. When an inbound has streamSettings.externalProxy set, one URL is emitted per external proxy. Empty array when the subId has no enabled clients.',
        params: [
          { name: 'subId', in: 'path', type: 'string', desc: "Subscription ID, taken from the client's subId field." },
        ],
        response:
          '{\n  "success": true,\n  "obj": [\n    "vless://uuid@host:443?security=reality&...#user1",\n    "vmess://eyJ2IjoyLC..."\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/clients/links/:email',
        summary:
          "Return every URL for one client across all attached inbounds — the same strings the Copy URL button copies in the panel UI. Supported protocols: vmess, vless, trojan, shadowsocks, hysteria. If streamSettings.externalProxy is set, returns one URL per external proxy. Protocols without a URL form (socks, http, mixed, wireguard, dokodemo, tunnel) contribute nothing.",
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique identifier).' },
        ],
        response:
          '{\n  "success": true,\n  "obj": [\n    "vless://uuid@host:443?...#user1"\n  ]\n}',
      },
    ],
  },

  {
    id: 'nodes',
    title: 'Nodes',
    description:
      'Manage remote 3x-ui panels acting as nodes for a central panel. All endpoints under /panel/api/nodes.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/nodes/list',
        summary: 'List every configured node with its connection details, health, and last heartbeat patch.',
        response: '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "name": "de-fra-1",\n      "remark": "",\n      "scheme": "https",\n      "address": "node1.example.com",\n      "port": 2053,\n      "basePath": "/",\n      "apiToken": "abcdef...",\n      "enable": true,\n      "allowPrivateAddress": false,\n      "status": "online",\n      "lastHeartbeat": 1700000000,\n      "latencyMs": 42,\n      "xrayVersion": "25.x.x",\n      "panelVersion": "v3.x.x",\n      "cpuPct": 23.5,\n      "memPct": 45.1,\n      "uptimeSecs": 86400,\n      "lastError": "",\n      "inboundCount": 5,\n      "clientCount": 27,\n      "onlineCount": 3,\n      "depletedCount": 1,\n      "createdAt": 1700000000,\n      "updatedAt": 1700000000\n    }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/nodes/get/:id',
        summary: 'Fetch a single node by ID.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/nodes/webCert/:id',
        summary: 'Fetch a node\'s own web TLS certificate/key file paths (proxied to the node). Used by the inbound form\'s "Set Cert from Panel" so a node-assigned inbound gets paths that exist on the node, not the central panel.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
        response: '{\n  "success": true,\n  "obj": {\n    "webCertFile": "/root/cert/example.com/fullchain.pem",\n    "webKeyFile": "/root/cert/example.com/privkey.pem"\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/add',
        summary: 'Register a new remote node. Provide its URL, apiToken, and optional remark / allowPrivateAddress flag.',
        body:
          '{\n  "name": "de-fra-1",\n  "remark": "",\n  "scheme": "https",\n  "address": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef...",\n  "enable": true,\n  "allowPrivateAddress": false\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/update/:id',
        summary: 'Replace a node\u2019s connection details. Same body shape as /add.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
        body: '{\n  "name": "de-fra-1",\n  "remark": "",\n  "scheme": "https",\n  "address": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef...",\n  "enable": true,\n  "allowPrivateAddress": false\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/del/:id',
        summary: 'Delete a node. Inbounds bound to it are not auto-migrated.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/setEnable/:id',
        summary: 'Pause or resume traffic sync with this node.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
        body: '{\n  "enable": true\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/test',
        summary: 'Probe a node without saving it. Uses the body as connection details and returns the same heartbeat snapshot a registered node would have.',
        body: '{\n  "scheme": "https",\n  "address": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef..."\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "status": "online",\n    "latencyMs": 42,\n    "xrayVersion": "25.x.x",\n    "panelVersion": "v3.x.x",\n    "cpuPct": 12.5,\n    "memPct": 45.2,\n    "uptimeSecs": 86400,\n    "error": ""\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/certFingerprint',
        summary: "Connect to the node over HTTPS without verifying its certificate and return the leaf certificate's SHA-256 (base64). Used by the Add/Edit Node dialog to fetch and pin a self-signed certificate. Uses the same body as /test.",
        body: '{\n  "scheme": "https",\n  "address": "node1.example.com",\n  "port": 2053,\n  "basePath": "/"\n}',
        response: '{\n  "success": true,\n  "obj": "k3b1...base64-sha256...="\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/probe/:id',
        summary: 'Probe an existing node, updating its cached health state.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/updatePanel',
        summary: 'Trigger the official panel self-updater on each given node (downloads the latest release and restarts). Only enabled, online nodes are updated; offline/disabled ones are reported as skipped. Returns a per-node result list.',
        body: '{\n  "ids": [1, 2, 3]\n}',
        response: '{\n  "success": true,\n  "obj": [\n    { "id": 1, "name": "de-1", "ok": true },\n    { "id": 2, "name": "fr-1", "ok": false, "error": "node is offline" }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/nodes/history/:id/:metric/:bucket',
        summary: 'Aggregated metric history for a node — same shape as /server/history, scoped to one node.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
          { name: 'metric', in: 'path', type: 'string', desc: 'cpu | mem.' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
      },
    ],
  },

  {
    id: 'custom-geo',
    title: 'Custom Geo',
    description:
      'Manage user-supplied GeoIP / GeoSite source files. All endpoints under /panel/api/custom-geo.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/custom-geo/list',
        summary: 'List configured custom geo sources with their type, alias, URL, status, and last-download timestamp.',
      },
      {
        method: 'GET',
        path: '/panel/api/custom-geo/aliases',
        summary: 'List geo aliases currently usable in routing rules — both built-in defaults and the user-configured ones.',
      },
      {
        method: 'POST',
        path: '/panel/api/custom-geo/add',
        summary: 'Register a custom geo source. Alias is auto-normalised; URL must point to a .dat / .json blob.',
        body:
          '{\n  "type": "geoip",\n  "alias": "myips",\n  "url": "https://example.com/geo/my.dat"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/custom-geo/update/:id',
        summary: 'Replace a custom geo source. Same body shape as /add.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Custom geo source ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/custom-geo/delete/:id',
        summary: 'Remove a custom geo source and its cached file.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Custom geo source ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/custom-geo/download/:id',
        summary: 'Re-download one custom geo source on demand.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Custom geo source ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/custom-geo/update-all',
        summary: 'Re-download every configured custom geo source. Errors are reported per-source in the response.',
      },
    ],
  },

  {
    id: 'backup',
    title: 'Backup',
    description: 'Operations that interact with the configured Telegram bot.',
    endpoints: [
      {
        method: 'POST',
        path: '/panel/api/backuptotgbot',
        summary: 'Send a fresh DB backup to every Telegram chat configured as an admin recipient. No body, no params.',
      },
    ],
  },

  {
    id: 'settings',
    title: 'Settings',
    description:
      'Panel configuration and user credentials. All endpoints live under /panel/setting and require a logged-in session or Bearer token.',
    endpoints: [
      {
        method: 'POST',
        path: '/panel/setting/all',
        summary: 'Return every panel setting: web server, Telegram bot, subscription, security, LDAP. The full JSON blob that the Settings page edits.',
        response: '{\n  "success": true,\n  "obj": {\n    "webPort": 2053,\n    "webCertFile": "",\n    "webKeyFile": "",\n    "webBasePath": "/",\n    "subPort": 10882,\n    "subPath": "/sub/",\n    "tgBotEnable": false,\n    "tgBotToken": "",\n    ...\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/defaultSettings',
        summary: 'Return the computed default settings based on the request host. Useful to preview what a fresh install would use.',
      },
      {
        method: 'POST',
        path: '/panel/setting/update',
        summary: 'Persist every setting at once. The body mirrors the shape returned by /all. Invalid values (bad ports, missing cert pairs, etc.) are rejected before write.',
        body: '{\n  "webPort": 2053,\n  "webBasePath": "/",\n  "subPort": 10882,\n  "subPath": "/sub/",\n  "tgBotEnable": false,\n  ...\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/updateUser',
        summary: 'Change the panel admin username and password. Requires the current credentials for verification. The session is refreshed with the new values on success.',
        params: [
          { name: 'oldUsername', in: 'body', type: 'string', desc: 'Current admin username.' },
          { name: 'oldPassword', in: 'body', type: 'string', desc: 'Current admin password.' },
          { name: 'newUsername', in: 'body', type: 'string', desc: 'Desired new username.' },
          { name: 'newPassword', in: 'body', type: 'string', desc: 'Desired new password.' },
        ],
        body: '{\n  "oldUsername": "admin",\n  "oldPassword": "admin",\n  "newUsername": "newadmin",\n  "newPassword": "newpass"\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/restartPanel',
        summary: 'Restart the entire 3x-ui process after a 3-second grace period. The connection drops immediately; the panel comes back online ~5-10 seconds later.',
      },
      {
        method: 'GET',
        path: '/panel/setting/getDefaultJsonConfig',
        summary: 'Return the built-in default Xray JSON config template that ships with this panel version.',
      },
    ],
  },

  {
    id: 'api-tokens',
    title: 'API Tokens',
    description:
      'Manage Bearer tokens used for programmatic auth (bots, central panels acting on this node, CI). Each token has a unique name and an enabled flag — disable to revoke without deleting, delete to revoke permanently. Tokens are stored plaintext so the SPA can show them on demand. Send one as <code>Authorization: Bearer &lt;token&gt;</code> on any /panel/api/* request.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/setting/apiTokens',
        summary: 'List every API token, enabled or not.',
        response: '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "name": "default",\n      "token": "abcdef-12345-...",\n      "enabled": true,\n      "createdAt": 1736000000\n    }\n  ]\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/apiTokens/create',
        summary: 'Mint a new API token. Name must be unique and 1-64 characters; the token string is server-generated.',
        params: [
          { name: 'name', in: 'body', type: 'string', desc: 'Human-readable label, e.g. "central-panel-a".' },
        ],
        body: '{\n  "name": "central-panel-a"\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "id": 2,\n    "name": "central-panel-a",\n    "token": "new-token-string",\n    "enabled": true,\n    "createdAt": 1736000000\n  }\n}',
        errorResponse: '{\n  "success": false,\n  "msg": "a token with that name already exists"\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/apiTokens/delete/:id',
        summary: 'Permanently delete a token. Any caller using it stops authenticating immediately.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Token row ID.' },
        ],
        response: '{\n  "success": true\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/apiTokens/setEnabled/:id',
        summary: 'Toggle a token enabled/disabled without deleting it. Disabled tokens are rejected by checkAPIAuth on the next request.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Token row ID.' },
          { name: 'enabled', in: 'body', type: 'boolean', desc: 'New enabled state.' },
        ],
        body: '{\n  "enabled": false\n}',
        response: '{\n  "success": true\n}',
      },
    ],
  },

  {
    id: 'xray-settings',
    title: 'Xray Settings',
    description:
      'Xray configuration template, outbound management, Warp/Nord integration, and config testing. All endpoints under /panel/xray.',
    endpoints: [
      {
        method: 'POST',
        path: '/panel/xray/',
        summary: 'Return the Xray config template (JSON string), available inbound tags, client reverse tags, and the configured outbound test URL in one response.',
        response: '{\n  "success": true,\n  "obj": {\n    "xraySetting": "{...raw xray config...}",\n    "inboundTags": "[\\"inbound-443\\"]",\n    "clientReverseTags": "[]",\n    "outboundTestUrl": "https://www.google.com/generate_204"\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/xray/getDefaultJsonConfig',
        summary: 'Return the built-in default Xray config shipped with the panel (identical to /panel/setting/getDefaultJsonConfig).',
      },
      {
        method: 'GET',
        path: '/panel/xray/getOutboundsTraffic',
        summary: 'Return traffic statistics for every outbound. Each outbound shows up/down/total counters.',
      },
      {
        method: 'GET',
        path: '/panel/xray/getXrayResult',
        summary: 'Return the most recent Xray process stdout/stderr output. Useful to check for startup errors or runtime warnings.',
      },
      {
        method: 'POST',
        path: '/panel/xray/update',
        summary: 'Save the Xray JSON config template and optionally the outbound test URL. Both are sent as form fields.',
        params: [
          { name: 'xraySetting', in: 'body (form)', type: 'string', desc: 'Full Xray JSON config template.' },
          { name: 'outboundTestUrl', in: 'body (form)', type: 'string', desc: 'URL used for outbound reachability tests. Defaults to https://www.google.com/generate_204.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/xray/warp/:action',
        summary: 'Manage Cloudflare Warp integration. The action parameter selects the operation.',
        params: [
          { name: 'action', in: 'path', type: 'string', desc: 'data — return Warp stats (quota, remaining). del — delete Warp data. config — return current Warp config. reg — register a new Warp endpoint (sends privateKey, publicKey). license — set a Warp+ license key (sends license).' },
          { name: 'privateKey', in: 'body (form)', type: 'string', desc: 'Required when action=reg.' },
          { name: 'publicKey', in: 'body (form)', type: 'string', desc: 'Required when action=reg.' },
          { name: 'license', in: 'body (form)', type: 'string', desc: 'Required when action=license.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/xray/nord/:action',
        summary: 'Manage NordVPN integration. The action parameter selects the operation.',
        params: [
          { name: 'action', in: 'path', type: 'string', desc: 'countries — list available countries. servers — list servers in a country (sends countryId). reg — get NordVPN credentials (sends token). setKey — store NordVPN API key (sends key). data — return current NordVPN connection data. del — delete NordVPN data.' },
          { name: 'countryId', in: 'body (form)', type: 'string', desc: 'Required when action=servers.' },
          { name: 'token', in: 'body (form)', type: 'string', desc: 'Required when action=reg.' },
          { name: 'key', in: 'body (form)', type: 'string', desc: 'Required when action=setKey.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/xray/resetOutboundsTraffic',
        summary: 'Reset traffic counters for a specific outbound by tag.',
        params: [
          { name: 'tag', in: 'body (form)', type: 'string', desc: 'Outbound tag to reset (e.g. "proxy", "direct").' },
        ],
        body: 'tag=proxy',
      },
      {
        method: 'POST',
        path: '/panel/xray/testOutbound',
        summary: 'Test an outbound configuration. Sends the outbound JSON (required), optionally all outbounds (to resolve sockopt.dialerProxy dependencies), and a mode flag.',
        params: [
          { name: 'outbound', in: 'body (form)', type: 'string', desc: 'JSON-encoded single outbound to test (required).' },
          { name: 'allOutbounds', in: 'body (form)', type: 'string', desc: 'JSON array of all outbounds — used to resolve dialerProxy chains.' },
          { name: 'mode', in: 'body (form)', type: 'string', desc: '"tcp" for a fast dial-only probe (parallel-safe). Default/empty uses a full HTTP probe through a temp xray instance.' },
        ],
        body: 'outbound={"protocol":"freedom","settings":{}}&mode=tcp',
      },
    ],
  },

  {
    id: 'subscription',
    title: 'Subscription Server',
    description:
      'A separate HTTP/HTTPS server that serves proxy subscription links (standard, JSON, and Clash) to clients. The server listens on its own port (default 10882) and is configured in Settings → Subscription. Paths are configurable; defaults are shown below. All subscription endpoints set response headers for client apps to read traffic/expiry info.',
    subHeader: [
      { name: 'Subscription-Userinfo', desc: 'Traffic and expiry: <code>upload=N; download=N; total=N; expire=TS</code>' },
      { name: 'Profile-Title', desc: 'Base64-encoded subscription display name' },
      { name: 'Profile-Web-Page-Url', desc: 'Link to the subscription info page' },
      { name: 'Support-Url', desc: 'Support contact URL configured in settings' },
      { name: 'Profile-Update-Interval', desc: 'Suggested polling interval in minutes (e.g. <code>10</code>)' },
      { name: 'Announce', desc: 'Base64-encoded announcement string' },
      { name: 'Routing-Enable', desc: '<code>true</code> or <code>false</code> — whether routing rules are included' },
      { name: 'Routing', desc: 'Global routing rules for client apps that support them (e.g. Happ)' },
    ],
    endpoints: [
      {
        method: 'GET',
        path: '/{subPath}:subid',
        summary: 'Return base64-encoded subscription links for all enabled clients matching the subscription ID. When the request has an Accept: text/html header or ?html=1, renders a styled info page instead. Default path: /sub/:subid.',
        params: [
          { name: 'subid', in: 'path', type: 'string', desc: 'Client subscription ID.' },
        ],
      },
      {
        method: 'GET',
        path: '/{jsonPath}:subid',
        summary: 'Return subscription as a JSON array of proxy configs (one per enabled client). Only when JSON subscription is enabled in settings. Default path: /json/:subid.',
        params: [
          { name: 'subid', in: 'path', type: 'string', desc: 'Client subscription ID.' },
        ],
      },
      {
        method: 'GET',
        path: '/{clashPath}:subid',
        summary: 'Return subscription as a Clash/Mihomo-compatible YAML config. Only when Clash subscription is enabled in settings. Default path: /clash/:subid.',
        params: [
          { name: 'subid', in: 'path', type: 'string', desc: 'Client subscription ID.' },
        ],
      },
    ],
  },

  {
    id: 'websocket',
    title: 'WebSocket',
    description:
      'Real-time status updates via WebSocket. Connect once at <code>ws://<panel>/ws</code> to receive a stream of JSON messages without polling. Requires an authenticated session cookie (Bearer token auth is not supported). Each message has a <code>type</code> field that identifies the payload shape.',
    endpoints: [
      {
        method: 'GET',
        path: '/ws',
        summary: 'Upgrade an HTTP connection to a WebSocket. Requires an authenticated session cookie (Bearer token auth is not supported here). Returns 101 Switching Protocols on success. The server then pushes JSON messages described below.',
      },
      {
        method: 'WS',
        path: '→ type: status',
        summary: 'Server health snapshot pushed every 2 seconds. Contains CPU, memory, swap, disk, network IO, load, and Xray state — same shape as <code>GET /panel/api/server/status</code>.',
        response: '{\n  "type": "status",\n  "data": { "cpu": 12.5, "mem": { "current": 2147483648, "total": 8589934592 }, "xray": { "state": "running" } }\n}',
      },
      {
        method: 'WS',
        path: '→ type: xrayState',
        summary: 'Xray process state change. Fired when Xray starts, stops, or encounters an error.',
        response: '{\n  "type": "xrayState",\n  "data": "running"\n}',
      },
      {
        method: 'WS',
        path: '→ type: notification',
        summary: 'In-panel toast notification. Fired on Xray stop/restart, DB import, panel restart, etc.',
        response: '{\n  "type": "notification",\n  "title": "Xray service restarted",\n  "body": "Xray has been restarted successfully",\n  "severity": "success"\n}',
      },
      {
        method: 'WS',
        path: '→ type: invalidate',
        summary: 'Instructs the UI to re-fetch a resource. Fired when another admin session modifies data (e.g. toggling inbound enable).',
        response: '{\n  "type": "invalidate",\n  "resource": "inbounds"\n}',
      },
    ],
  },
];
