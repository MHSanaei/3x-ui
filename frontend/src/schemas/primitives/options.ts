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
  SRV_PORT_ONLY: 'SrvPortOnly',
  SRV_ADDRESS_ONLY: 'SrvAddressOnly',
  SRV_PORT_AND_ADDRESS: 'SrvPortAndAddress',
  TXT_PORT_ONLY: 'TxtPortOnly',
  TXT_ADDRESS_ONLY: 'TxtAddressOnly',
  TXT_PORT_AND_ADDRESS: 'TxtPortAndAddress',
});

export const DNSRuleActions = Object.freeze(['direct', 'drop', 'return', 'hijack'] as const);

export const TLS_VERSION_OPTION = Object.freeze({
  TLS10: '1.0',
  TLS11: '1.1',
  TLS12: '1.2',
  TLS13: '1.3',
});

export const TLS_CIPHER_OPTION = Object.freeze({
  AES_128_GCM: 'TLS_AES_128_GCM_SHA256',
  AES_256_GCM: 'TLS_AES_256_GCM_SHA384',
  CHACHA20_POLY1305: 'TLS_CHACHA20_POLY1305_SHA256',
  ECDHE_ECDSA_AES_128_CBC: 'TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA',
  ECDHE_ECDSA_AES_256_CBC: 'TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA',
  ECDHE_RSA_AES_128_CBC: 'TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA',
  ECDHE_RSA_AES_256_CBC: 'TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA',
  ECDHE_ECDSA_AES_128_GCM: 'TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256',
  ECDHE_ECDSA_AES_256_GCM: 'TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384',
  ECDHE_RSA_AES_128_GCM: 'TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256',
  ECDHE_RSA_AES_256_GCM: 'TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384',
  ECDHE_ECDSA_CHACHA20_POLY1305: 'TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256',
  ECDHE_RSA_CHACHA20_POLY1305: 'TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256',
});

export const USAGE_OPTION = Object.freeze({
  ENCIPHERMENT: 'encipherment',
  VERIFY: 'verify',
  ISSUE: 'issue',
});

export const DOMAIN_STRATEGY_OPTION = Object.freeze({
  AS_IS: 'AsIs',
  USE_IP: 'UseIP',
  USE_IPV6V4: 'UseIPv6v4',
  USE_IPV6: 'UseIPv6',
  USE_IPV4V6: 'UseIPv4v6',
  USE_IPV4: 'UseIPv4',
  FORCE_IP: 'ForceIP',
  FORCE_IPV6V4: 'ForceIPv6v4',
  FORCE_IPV6: 'ForceIPv6',
  FORCE_IPV4V6: 'ForceIPv4v6',
  FORCE_IPV4: 'ForceIPv4',
});

export const TCP_CONGESTION_OPTION = Object.freeze({
  BBR: 'bbr',
  CUBIC: 'cubic',
  RENO: 'reno',
});
