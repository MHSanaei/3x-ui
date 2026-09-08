export const ROUTING_DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];
export const LOG_LEVELS = ['none', 'debug', 'info', 'warning', 'error'];
export const ACCESS_LOG = ['none', './access.log'];
export const ERROR_LOG = ['none', './error.log'];
export const MASK_ADDRESS = ['quarter', 'half', 'full'];
export const BITTORRENT_PROTOCOLS = ['bittorrent'];

export const IPS_OPTIONS = [
  { label: 'Private IPs', value: 'geoip:private' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia (RuNetFreedom)', value: 'ext:geoip_runet.dat:ru' },
  { label: '🇻🇳 Vietnam', value: 'geoip:vn' },
  { label: '🇪🇸 Spain', value: 'geoip:es' },
  { label: '🇮🇩 Indonesia', value: 'geoip:id' },
  { label: '🇺🇦 Ukraine', value: 'geoip:ua' },
  { label: '🇹🇷 Türkiye', value: 'geoip:tr' },
  { label: '🇧🇷 Brazil', value: 'geoip:br' },
];
export const DOMAINS_OPTIONS = [
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia (RuNetFreedom)', value: 'ext:geosite_runet.dat:ru-available-only-inside' },
  { label: '🇷🇺 RoscomVPN Whitelist', value: 'ext:geosite_roscom.dat:whitelist' },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
export const BLOCK_DOMAINS_OPTIONS = [
  { label: 'Ads All', value: 'geosite:category-ads-all' },
  { label: 'Ads RU 🇷🇺', value: 'ext:geosite_runet.dat:category-ads-all' },
  { label: 'Geoblock RU 🇷🇺 (RoscomVPN)', value: 'ext:geosite_roscom.dat:category-geoblock-ru' },
  { label: 'Adult +18', value: 'geosite:category-porn' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia (RuNetFreedom)', value: 'ext:geosite_runet.dat:ru-available-only-inside' },
  { label: '🇷🇺 RoscomVPN Whitelist', value: 'ext:geosite_roscom.dat:whitelist' },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
export const SERVICES_OPTIONS = [
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Google', value: 'geosite:google' },
  { label: 'OpenAI', value: 'geosite:openai' },
  { label: 'Spotify', value: 'geosite:spotify' },
  { label: 'Netflix', value: 'geosite:netflix' },
  { label: 'Reddit', value: 'geosite:reddit' },
  { label: 'Speedtest', value: 'geosite:speedtest' },
];

export const directSettings = { tag: 'direct', protocol: 'freedom' };
export const blockedSettings = { tag: 'blocked', protocol: 'blackhole', settings: {} };
export const ipv4Settings = {
  tag: 'IPv4',
  protocol: 'freedom',
  settings: { domainStrategy: 'UseIPv4' },
};
