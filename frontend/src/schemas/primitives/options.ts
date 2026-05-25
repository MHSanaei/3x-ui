export const UTLS_FINGERPRINT = Object.freeze({
  UTLS_CHROME: 'chrome',
  UTLS_FIREFOX: 'firefox',
  UTLS_SAFARI: 'safari',
  UTLS_IOS: 'ios',
  UTLS_android: 'android',
  UTLS_EDGE: 'edge',
  UTLS_360: '360',
  UTLS_QQ: 'qq',
  UTLS_RANDOM: 'random',
  UTLS_RANDOMIZED: 'randomized',
  UTLS_RONDOMIZEDNOALPN: 'randomizednoalpn',
  UTLS_UNSAFE: 'unsafe',
});

export const ALPN_OPTION = Object.freeze({
  H3: 'h3',
  H2: 'h2',
  HTTP1: 'http/1.1',
});

export const SNIFFING_OPTION = Object.freeze({
  HTTP: 'http',
  TLS: 'tls',
  QUIC: 'quic',
  FAKEDNS: 'fakedns',
});

export const USERS_SECURITY = Object.freeze({
  AES_128_GCM: 'aes-128-gcm',
  CHACHA20_POLY1305: 'chacha20-poly1305',
  AUTO: 'auto',
  NONE: 'none',
  ZERO: 'zero',
});

export const MODE_OPTION = Object.freeze({
  AUTO: 'auto',
  PACKET_UP: 'packet-up',
  STREAM_UP: 'stream-up',
  STREAM_ONE: 'stream-one',
});

export const WireguardDomainStrategy = Object.freeze([
  'ForceIP',
  'ForceIPv4',
  'ForceIPv4v6',
  'ForceIPv6',
  'ForceIPv6v4',
] as const);

export const Address_Port_Strategy = Object.freeze({
  NONE: 'none',
  SrvPortOnly: 'srvportonly',
  SrvAddressOnly: 'srvaddressonly',
  SrvPortAndAddress: 'srvportandaddress',
  TxtPortOnly: 'txtportonly',
  TxtAddressOnly: 'txtaddressonly',
  TxtPortAndAddress: 'txtportandaddress',
});

export const DNSRuleActions = Object.freeze(['direct', 'drop', 'reject', 'hijack'] as const);
