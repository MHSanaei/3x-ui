export const OutboundProtocols = Object.freeze({
  Freedom: 'freedom',
  Blackhole: 'blackhole',
  DNS: 'dns',
  VMess: 'vmess',
  VLESS: 'vless',
  Trojan: 'trojan',
  Shadowsocks: 'shadowsocks',
  Wireguard: 'wireguard',
  Hysteria: 'hysteria',
  Socks: 'socks',
  HTTP: 'http',
  Loopback: 'loopback',
});

export const OutboundDomainStrategies = Object.freeze([
  'AsIs',
  'UseIP',
  'UseIPv4',
  'UseIPv6',
  'UseIPv6v4',
  'UseIPv4v6',
  'ForceIP',
  'ForceIPv6v4',
  'ForceIPv6',
  'ForceIPv4v6',
  'ForceIPv4',
] as const);

export type OutboundDomainStrategy = (typeof OutboundDomainStrategies)[number];
