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
      },
      {
        method: 'GET',
        path: '/logout',
        summary: 'Clear the session cookie. Redirects back to the login page; not useful from non-browser clients.',
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
    title: 'Inbounds API',
    description:
      'Manage inbound configurations and their clients. All endpoints live under /panel/api/inbounds and require a logged-in session or Bearer token. Link-generating endpoints honour X-Forwarded-Host / X-Forwarded-Proto, so callers behind a reverse proxy get the correct external host in returned URLs.',
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
      },
      {
        method: 'GET',
        path: '/panel/api/inbounds/getClientTrafficsById/:id',
        summary: 'Traffic counters for a client identified by its UUID/password.',
        params: [
          { name: 'id', in: 'path', type: 'string', desc: 'Client subId / UUID.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/inbounds/add',
        summary: 'Create a new inbound. Send the full inbound payload (protocol, port, settings JSON, streamSettings JSON, sniffing JSON, remark, expiryTime, total, enable).',
        body:
          '{\n  "enable": true,\n  "remark": "VLESS-443",\n  "listen": "",\n  "port": 443,\n  "protocol": "vless",\n  "expiryTime": 0,\n  "total": 0,\n  "settings": "{\\"clients\\":[{\\"id\\":\\"...\\",\\"email\\":\\"user1\\"}],\\"decryption\\":\\"none\\",\\"fallbacks\\":[]}",\n  "streamSettings": "{\\"network\\":\\"tcp\\",\\"security\\":\\"reality\\",\\"realitySettings\\":{...}}",\n  "sniffing": "{\\"enabled\\":true,\\"destOverride\\":[\\"http\\",\\"tls\\"]}"\n}',
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
    title: 'Server API',
    description:
      'System status, log retrieval, certificate generators, Xray binary management, and backup/restore. All under /panel/api/server.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/server/status',
        summary: 'Real-time machine snapshot: CPU, memory, swap, disk, network IO, load averages, open connections, Xray state. Cached and refreshed every 2 seconds in the background.',
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
          { name: 'metric', in: 'path', type: 'string', desc: 'cpu | mem | swap | netIn | netOut | tcpCount | udpCount | load1 | online.' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds. Allowed: 2, 30, 60, 120, 180, 300.' },
        ],
      },
      {
        method: 'GET',
        path: '/panel/api/server/getXrayVersion',
        summary: 'List Xray binary versions available for install on this host.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getPanelUpdateInfo',
        summary: 'Check whether a newer 3x-ui release is available on GitHub.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getConfigJson',
        summary: 'Return the assembled Xray config that’s currently running on this host.',
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
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewX25519Cert',
        summary: 'Generate a new X25519 keypair for Reality.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewmldsa65',
        summary: 'Generate a new ML-DSA-65 keypair (post-quantum signature). Returns {privateKey, publicKey, seed}.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewmlkem768',
        summary: 'Generate a new ML-KEM-768 keypair (post-quantum KEM). Returns {clientKey, serverKey}.',
      },
      {
        method: 'GET',
        path: '/panel/api/server/getNewVlessEnc',
        summary: 'Generate VLESS encryption auth options. Returns auths with id, label, decryption, and encryption.',
      },
      {
        method: 'POST',
        path: '/panel/api/server/stopXrayService',
        summary: 'Stop the Xray binary. All proxies go offline immediately.',
      },
      {
        method: 'POST',
        path: '/panel/api/server/restartXrayService',
        summary: 'Reload Xray with the current config. Typically required after structural inbound or routing changes.',
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
        summary: 'Return the last N lines of the panel’s own log.',
        params: [
          { name: 'count', in: 'path', type: 'number', desc: 'Number of trailing log lines.' },
        ],
        body: '{\n  "level": "info",\n  "syslog": false\n}',
      },
      {
        method: 'POST',
        path: '/panel/api/server/xraylogs/:count',
        summary: 'Return the last N lines of the Xray process log.',
        params: [
          { name: 'count', in: 'path', type: 'number', desc: 'Number of trailing log lines.' },
        ],
      },
      {
        method: 'POST',
        path: '/panel/api/server/importDB',
        summary: 'Restore the panel DB from an uploaded SQLite file (multipart form, field name "db"). The panel restarts after restore. Destructive.',
      },
      {
        method: 'POST',
        path: '/panel/api/server/getNewEchCert',
        summary: 'Generate a new ECH (Encrypted Client Hello) keypair. Body picks the algorithm.',
      },
    ],
  },

  {
    id: 'nodes',
    title: 'Nodes API',
    description:
      'Manage remote 3x-ui panels acting as nodes for a central panel. All endpoints under /panel/api/nodes.',
    endpoints: [
      {
        method: 'GET',
        path: '/panel/api/nodes/list',
        summary: 'List every configured node with its connection details, health, and last heartbeat patch.',
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
        summary: 'Replace a node’s connection details. Same body shape as /add.',
        params: [
          { name: 'id', in: 'path', type: 'number', desc: 'Node ID.' },
        ],
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
          { name: 'metric', in: 'path', type: 'string', desc: 'Metric key (cpu, mem, netIn, …).' },
          { name: 'bucket', in: 'path', type: 'number', desc: 'Bucket size in seconds.' },
        ],
      },
    ],
  },

  {
    id: 'customGeo',
    title: 'Custom Geo API',
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
        method: 'GET',
        path: '/panel/api/backuptotgbot',
        summary: 'Send a fresh DB backup to every Telegram chat configured as an admin recipient. No body, no params.',
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
};
