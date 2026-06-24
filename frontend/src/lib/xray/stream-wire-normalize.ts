// Shapes the streamSettings subtree that 3x-ui persists to match what
// xray-core actually consumes. The panel's Zod defaults mirror the full
// SplitHTTPConfig / SockoptObject schema, but many fields are mode-specific
// (packet-up vs stream-one) or side-specific (inbound vs outbound). Emitting
// them anyway bloats configs and — for sockopt — can inject doc-example
// values like tcpWindowClamp: 600 that throttle throughput.

export type StreamWireSide = 'inbound' | 'outbound';

const PACKET_UP_FIELDS = [
  'scMaxEachPostBytes',
  'scMinPostsIntervalMs',
  'scMaxBufferedPosts',
] as const;

const STREAM_UP_SERVER_FIELDS = ['scStreamUpServerSecs'] as const;

const PLACEMENT_STRING_FIELDS = [
  'sessionIDPlacement',
  'sessionIDKey',
  'sessionIDTable',
  'sessionIDLength',
  'seqPlacement',
  'seqKey',
  'uplinkDataPlacement',
  'uplinkDataKey',
  'uplinkHTTPMethod',
  'xPaddingKey',
  'xPaddingHeader',
  'xPaddingPlacement',
  'xPaddingMethod',
] as const;

function isRecord(v: unknown): v is Record<string, unknown> {
  return v != null && typeof v === 'object' && !Array.isArray(v);
}

function nonEmptyString(v: unknown): v is string {
  return typeof v === 'string' && v.trim() !== '';
}

function hasMeaningfulHeaders(headers: unknown): boolean {
  return isRecord(headers) && Object.keys(headers).length > 0;
}

// Upper bound of an xray-core Int32Range value: "16-32" -> 32, "4" -> 4,
// 4 -> 4, "" / null -> 0. xmux fields are ranges, and xray-core keys its
// mutual-exclusivity check on the `.To` (upper) side.
function int32RangeUpper(v: unknown): number {
  if (typeof v === 'number') return Number.isFinite(v) ? v : 0;
  if (typeof v !== 'string') return 0;
  const trimmed = v.trim();
  if (trimmed === '') return 0;
  const parts = trimmed.split('-');
  const n = Number(parts[parts.length - 1]);
  return Number.isFinite(n) ? n : 0;
}

// xray-core's XmuxConfig rejects a config that sets BOTH maxConnections
// and maxConcurrency ("maxConnections cannot be specified together with
// maxConcurrency"). The panel pre-fills maxConcurrency ("16-32") whenever
// XMUX is enabled, so any explicit maxConnections would otherwise always
// collide and make xray refuse the config. maxConnections defaults to 0
// (off), so a positive value is an explicit opt-in to connection-pool
// mode — honor it and drop the leftover default maxConcurrency, matching
// core's "one strategy at a time" semantics.
function resolveXmuxExclusivity(xmux: Record<string, unknown>): Record<string, unknown> {
  if (int32RangeUpper(xmux.maxConnections) > 0 && int32RangeUpper(xmux.maxConcurrency) > 0) {
    const out = { ...xmux };
    delete out.maxConcurrency;
    return out;
  }
  return xmux;
}

/** Validates REALITY inbound `target` / `dest` (must include a port). */
export function validateRealityTarget(target: string): string | undefined {
  const trimmed = target.trim();
  if (!trimmed) {
    return 'pages.inbounds.form.realityTargetRequired';
  }

  // Unix socket destinations (rare, but valid in xray-core).
  if (trimmed.startsWith('/') || trimmed.startsWith('@')) {
    return undefined;
  }

  // Pure port → localhost:port in xray-core.
  if (/^\d+$/.test(trimmed)) {
    const port = Number(trimmed);
    if (port >= 1 && port <= 65535) return undefined;
    return 'pages.inbounds.form.realityTargetInvalidPort';
  }

  const lastColon = trimmed.lastIndexOf(':');
  if (lastColon <= 0 || lastColon === trimmed.length - 1) {
    return 'pages.inbounds.form.realityTargetNeedsPort';
  }

  const portPart = trimmed.slice(lastColon + 1);
  if (!/^\d+$/.test(portPart)) {
    return 'pages.inbounds.form.realityTargetInvalidPort';
  }
  const port = Number(portPart);
  if (port < 1 || port > 65535) {
    return 'pages.inbounds.form.realityTargetInvalidPort';
  }
  return undefined;
}

function dropEmptyStrings(obj: Record<string, unknown>, keys: readonly string[]): void {
  for (const key of keys) {
    const v = obj[key];
    if (v === '' || v == null) delete obj[key];
  }
}

function dropFalseFlags(obj: Record<string, unknown>, keys: readonly string[]): void {
  for (const key of keys) {
    if (obj[key] === false) delete obj[key];
  }
}

function dropZeroNumbers(obj: Record<string, unknown>, keys: readonly string[]): void {
  for (const key of keys) {
    if (obj[key] === 0) delete obj[key];
  }
}

function normalizeTlsForWire(raw: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...raw };
  if (out.fingerprint === '') delete out.fingerprint;

  // Empty server-side tuning fields mean "use xray-core's default" — never emit them.
  if (Array.isArray(out.curvePreferences) && out.curvePreferences.length === 0) {
    delete out.curvePreferences;
  }
  if (out.masterKeyLog === '' || out.masterKeyLog == null) delete out.masterKeyLog;
  if (isRecord(out.echSockopt)) {
    const echSock = normalizeSockoptForWire(out.echSockopt);
    if (echSock) {
      out.echSockopt = echSock;
    } else {
      delete out.echSockopt;
    }
  }

  const settings = out.settings;
  if (isRecord(settings)) {
    const settingsOut: Record<string, unknown> = { ...settings };
    if (settingsOut.fingerprint === '') delete settingsOut.fingerprint;
    out.settings = settingsOut;
  }

  return out;
}

export function normalizeXhttpForWire(
  raw: Record<string, unknown>,
  side: StreamWireSide,
): Record<string, unknown> {
  const out: Record<string, unknown> = { ...raw };
  const mode = typeof out.mode === 'string' && out.mode !== '' ? out.mode : 'auto';
  const enableXmux = out.enableXmux === true;
  delete out.enableXmux;

  if (side === 'inbound') {
    if (!enableXmux) delete out.xmux;
    // scMinPostsIntervalMs is a client-only tuning knob that subscriptions
    // must propagate to clients. Only strip the xray-core default ("30")
    // or empty values — the literal "30" is a known DPI fingerprint (#5141).
    if (out.scMinPostsIntervalMs === '' || out.scMinPostsIntervalMs === '30') {
      delete out.scMinPostsIntervalMs;
    }
    delete out.uplinkChunkSize;
  }

  if (isRecord(out.xmux)) {
    out.xmux = resolveXmuxExclusivity(out.xmux);
  }

  dropEmptyStrings(out, PLACEMENT_STRING_FIELDS);
  // Empty tuning fields mean "use xray-core's default" — never emit them.
  dropEmptyStrings(out, ['scMaxEachPostBytes', 'scMinPostsIntervalMs', 'scStreamUpServerSecs']);

  if (!hasMeaningfulHeaders(out.headers)) {
    delete out.headers;
  }

  if (out.xPaddingObfsMode !== true) {
    delete out.xPaddingObfsMode;
    dropEmptyStrings(out, [
      'xPaddingKey',
      'xPaddingHeader',
      'xPaddingPlacement',
      'xPaddingMethod',
    ]);
  }

  if (out.noGRPCHeader !== true) delete out.noGRPCHeader;
  if (out.noSSEHeader !== true) delete out.noSSEHeader;
  if (out.serverMaxHeaderBytes === 0) delete out.serverMaxHeaderBytes;
  if (out.uplinkChunkSize === 0) delete out.uplinkChunkSize;

  if (mode === 'stream-one') {
    for (const key of PACKET_UP_FIELDS) delete out[key];
    for (const key of STREAM_UP_SERVER_FIELDS) delete out[key];
  } else if (mode === 'stream-up') {
    for (const key of PACKET_UP_FIELDS) delete out[key];
    if (side === 'outbound') {
      delete out.scStreamUpServerSecs;
    }
  } else if (mode === 'packet-up') {
    delete out.scStreamUpServerSecs;
  }

  return out;
}

export function normalizeSockoptForWire(
  raw: Record<string, unknown>,
): Record<string, unknown> | undefined {
  const out: Record<string, unknown> = { ...raw };

  dropZeroNumbers(out, [
    'tcpWindowClamp',
    'tcpMaxSeg',
    'tcpUserTimeout',
    'tcpKeepAliveIdle',
    'tcpKeepAliveInterval',
    'mark',
  ]);

  dropFalseFlags(out, [
    'acceptProxyProtocol',
    'tcpFastOpen',
    'tcpMptcp',
    'penetrate',
    'V6Only',
  ]);

  if (out.tproxy === 'off') delete out.tproxy;
  if (out.domainStrategy === 'AsIs') delete out.domainStrategy;
  if (out.addressPortStrategy === 'none') delete out.addressPortStrategy;
  if (nonEmptyString(out.dialerProxy) === false) delete out.dialerProxy;
  if (nonEmptyString(out.interface) === false) delete out.interface;
  if (Array.isArray(out.trustedXForwardedFor) && out.trustedXForwardedFor.length === 0) {
    delete out.trustedXForwardedFor;
  }
  if (Array.isArray(out.customSockopt) && out.customSockopt.length === 0) {
    delete out.customSockopt;
  }

  const he = out.happyEyeballs;
  if (isRecord(he)) {
    const heOut: Record<string, unknown> = { ...he };
    if (heOut.tryDelayMs === 0) delete heOut.tryDelayMs;
    if (heOut.prioritizeIPv6 === false) delete heOut.prioritizeIPv6;
    if (heOut.interleave === 1) delete heOut.interleave;
    if (heOut.maxConcurrentTry === 4) delete heOut.maxConcurrentTry;
    if (Object.keys(heOut).length === 0) {
      delete out.happyEyeballs;
    } else {
      out.happyEyeballs = heOut;
    }
  }

  if (nonEmptyString(out.tcpcongestion) === false) delete out.tcpcongestion;

  if (Object.keys(out).length === 0) return undefined;
  return out;
}

export function normalizeStreamSettingsForWire(
  stream: Record<string, unknown>,
  opts: { side: StreamWireSide },
): Record<string, unknown> {
  const out: Record<string, unknown> = { ...stream };

  const xhttp = out.xhttpSettings;
  if (isRecord(xhttp)) {
    out.xhttpSettings = normalizeXhttpForWire(xhttp, opts.side);
  }

  const tls = out.tlsSettings;
  if (isRecord(tls)) {
    out.tlsSettings = normalizeTlsForWire(tls);
  }

  const sockopt = out.sockopt;
  if (isRecord(sockopt)) {
    const normalized = normalizeSockoptForWire(sockopt);
    if (normalized) {
      out.sockopt = normalized;
    } else {
      delete out.sockopt;
    }
  }

  return out;
}
