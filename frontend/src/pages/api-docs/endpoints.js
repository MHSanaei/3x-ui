export function safeInlineHtml(input) {
  if (!input) return '';
  const escape = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  const open = '<code>';
  const close = '</code>';
  let out = '';
  let i = 0;
  while (i < input.length) {
    const oIdx = input.indexOf(open, i);
    if (oIdx === -1) {
      out += escape(input.slice(i));
      break;
    }
    out += escape(input.slice(i, oIdx));
    const cIdx = input.indexOf(close, oIdx + open.length);
    if (cIdx === -1) {
      out += escape(input.slice(oIdx));
      break;
    }
    out += '<code>' + escape(input.slice(oIdx + open.length, cIdx)) + '</code>';
    i = cIdx + close.length;
  }
  return out;
}

export const sections = [
  {
    id: 'auth',
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
        summary: 'List every inbound owned by the authenticated user, including each inbound’s clientStats traffic counters.',
        response:
          '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "userId": 1,\n      "up": 0,\n      "down": 0,\n      "total": 0,\n      "remark": "VLESS-443",\n      "enable": true,\n      "expiryTime": 0,\n      "listen": "",\n      "port": 443,\n      "protocol": "vless",\n      "settings": "{\\"clients\\":[...]}",\n      "streamSettings": "{...}",\n      "tag": "inbound-443",\n      "sniffing": "{...}",\n      "clientStats": [...]\n    }\n  ]\n}',
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
        method: 'GET',
        path: '/panel/api/inbounds/getClientTraffics/:email',
        summary: 'Traffic counters for a client identified by email.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email (unique across the panel).' },
        ],
        response: '{\n  "success": true,\n  "obj": {\n    "email": "user1",\n    "up": 1048576,\n    "down": 2097152,\n    "total": 10737418240,\n    "expiryTime": 1735689600000\n  }\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/getClientTrafficsById/:id',
        summary: 'Traffic counters for a client identified by its UUID/password.',
        params: [
          { name: 'id', in: 'path', type: 'string', desc: 'Client subId / UUID.' },
        ],
        response: '{\n  "success": true,\n  "obj": {\n    "email": "user1",\n    "up": 1048576,\n    "down": 2097152,\n    "total": 10737418240,\n    "expiryTime": 1735689600000\n  }\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/add',
        summary: 'Create a new inbound. Send the full inbound payload (protocol, port, settings JSON, streamSettings JSON, sniffing JSON, remark, expiryTime, total, enable).',
        body:
          '{\n  "enable": true,\n  "remark": "VLESS-443",\n  "listen": "",\n  "port": 443,\n  "protocol": "vless",\n  "expiryTime": 0,\n  "total": 0,\n  "settings": "{\\"clients\\":[{\\"id\\":\\"...\\",\\"email\\":\\"user1\\"}],\\"decryption\\":\\"none\\",\\"fallbacks\\":[]}",\n  "streamSettings": "{\\"network\\":\\"tcp\\",\\"security\\":\\"reality\\",\\"realitySettings\\":{...}}",\n  "sniffing": "{\\"enabled\\":true,\\"destOverride\\":[\\"http\\",\\"tls\\"]}"\n}',
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
        path: '/panel/api/inbounds/clientIps/:email',
        summary: 'List source IPs that have connected with the given client’s credentials. Returns an array of "ip (timestamp)" strings.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/clearClientIps/:email',
        summary: 'Reset the recorded IP list for a client.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/addClient',
        summary: 'Add one or more clients to an existing inbound. The settings field is the JSON-encoded settings.clients array of the target inbound.',
        body:
          '{\n  "id": 1,\n  "settings": "{\\"clients\\":[{\\"id\\":\\"uuid-here\\",\\"email\\":\\"newuser\\",\\"limitIp\\":0,\\"totalGB\\":0,\\"expiryTime\\":0,\\"enable\\":true,\\"flow\\":\\"\\"}]}"\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/copyClients',
        summary: 'Copy selected clients from one inbound into another. Useful for duplicating user lists across protocols.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Target inbound ID.' },
          { name: 'sourceInboundId', in: 'body', type: 'number', desc: 'Inbound ID to read clients from.' },
          { name: 'clientEmails', in: 'body', type: 'string[]', desc: 'Emails of clients to copy. Empty means all clients.' },
          { name: 'flow', in: 'body', type: 'string', desc: 'Override the flow field on copied clients (e.g. "xtls-rprx-vision"). Empty to keep source flow.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/delClient/:clientId',
        summary: 'Delete a client by its UUID/password from a specific inbound.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
          { name: 'clientId', in: 'path', type: 'string', desc: 'Client UUID / password.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/updateClient/:clientId',
        summary: 'Update a single client without rewriting the whole settings JSON. Send the target inbound payload with the new client values.',
        params: [
          { name: 'clientId', in: 'path', type: 'string', desc: 'Client UUID / password.' },
        ],
        body:
          '{\n  "id": 1,\n  "settings": "{\\"clients\\":[{\\"id\\":\\"uuid-here\\",\\"email\\":\\"user1\\",\\"limitIp\\":2,\\"totalGB\\":10737418240,\\"expiryTime\\":1735689600000,\\"enable\\":true}]}"\n}',
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
        path: '/panel/api/inbounds/:id/resetClientTraffic/:email',
        summary: 'Zero out upload + download counters for one client.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/resetAllTraffics',
        summary: 'Reset upload + download counters on every inbound. Destructive — accounting history is lost.',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/resetAllClientTraffics/:id',
        summary: 'Reset traffic for every client in one inbound.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/delDepletedClients/:id',
        summary: 'Delete clients in this inbound whose traffic cap or expiry has elapsed. Pass id=-1 to sweep every inbound.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID, or -1 for all inbounds.' },
        ],
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
        method: 'POST',
        path: '/panel/api/inbounds/onlines',
        summary: 'List the emails of currently connected clients (last seen within the heartbeat window).',
        response: '{\n  "success": true,\n  "obj": ["user1", "user2"]\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/lastOnline',
        summary: 'Map of client email → last-seen unix timestamp.',
        response: '{\n  "success": true,\n  "obj": [\n    { "email": "user1", "lastOnline": 1700000000 },\n    { "email": "user2", "lastOnline": 1699999000 }\n  ]\n}',
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/getSubLinks/:subId',
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
        path: '/panel/api/inbounds/getClientLinks/:id/:email',
        summary:
          "Return the URL(s) for one client on one inbound — the same string the Copy URL button copies in the panel UI. Supported protocols: vmess, vless, trojan, shadowsocks, hysteria, hysteria2. If streamSettings.externalProxy is set, returns one URL per external proxy. Protocols without a URL form (socks, http, mixed, wireguard, dokodemo, tunnel) return an empty array.",
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
        response:
          '{\n  "success": true,\n  "obj": [\n    "vless://uuid@host:443?...#user1"\n  ]\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/updateClientTraffic/:email',
        summary: 'Manually adjust a client’s upload + download counters. Useful for migrations from external accounting systems.',
        params: [
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
        body: '{\n  "upload": 1073741824,\n  "download": 5368709120\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/:id/delClientByEmail/:email',
        summary: 'Delete a client identified by email rather than UUID.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Inbound ID.' },
          { name: 'email', in: 'path', type: 'string', desc: 'Client email.' },
        ],
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
    id: 'nodes',
    title: 'Nodes',
    description:
      'Manage remote 3x-ui panels acting as nodes for a central panel. All endpoints under /panel/api/nodes.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/nodes/list',
        summary: 'List every configured node with its connection details, health, and last heartbeat patch.',
        response: '{\n  "success": true,\n  "obj": [\n    {\n      "id": 1,\n      "name": "de-fra-1",\n      "scheme": "https",\n      "host": "node1.example.com",\n      "port": 2053,\n      "status": "online",\n      "cpu": 23.5,\n      "mem": 45.1\n    }\n  ]\n}',
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
        method: 'POST',
        path: '/panel/api/nodes/add',
        summary: 'Register a new remote node. Provide its URL, apiToken, and optional label/notes.',
        body:
          '{\n  "name": "de-fra-1",\n  "scheme": "https",\n  "host": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef..."\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/nodes/update/:id',
        summary: 'Replace a node\u2019s connection details. Same body shape as /add.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
        body: '{\n  "name": "de-fra-1",\n  "scheme": "https",\n  "host": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef..."\n}',
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
        summary: 'Probe a node without saving it. Uses the body as connection details and returns whether the handshake succeeds.',
        body: '{\n  "scheme": "https",\n  "host": "node1.example.com",\n  "port": 2053,\n  "basePath": "/",\n  "apiToken": "abcdef..."\n}',
        response: '{\n  "success": true,\n  "obj": {\n    "status": "online",\n    "cpu": 12.5,\n    "mem": 45.2\n  }\n}',
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
    id: 'customGeo',
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
      'Panel configuration, user credentials, and API token management. All endpoints live under /panel/setting and require a logged-in session or Bearer token.',
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
      {
        method: 'GET',
        path: '/panel/setting/getApiToken',
        summary: 'Return the current API Bearer token. The token is auto-generated on first read so existing installs upgrade transparently.',
        response: '{\n  "success": true,\n  "obj": "abcdef-12345-..."\n}',
      },
      {
        method: 'POST',
        path: '/panel/setting/regenerateApiToken',
        summary: 'Rotate the API Bearer token. Any remote central panel that cached the old value will start failing heartbeats until updated with the new token.',
        response: '{\n  "success": true,\n  "obj": "new-token-string"\n}',
      },
    ],
  },

  {
    id: 'xraySettings',
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

export const methodColors = {
  GET: 'blue',
  POST: 'green',
  PUT: 'orange',
  PATCH: 'orange',
  DELETE: 'red',
  WS: 'purple',
};
