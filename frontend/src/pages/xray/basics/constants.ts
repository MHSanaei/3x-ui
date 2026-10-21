import i18next from 'i18next';
export const ROUTING_DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];
export const LOG_LEVELS = ['none', 'debug', 'info', 'warning', 'error'];
export const ACCESS_LOG = ['none', './access.log'];
export const ERROR_LOG = ['none', './error.log'];
export const MASK_ADDRESS = ['quarter', 'half', 'full'];
export const BITTORRENT_PROTOCOLS = ['bittorrent'];

export const IPS_OPTIONS = [
  {
    get label() {
      return i18next.t('ui.privateIPs');
    },
    value: 'geoip:private',
  },
  {
    get label() {
      return i18next.t('ui.iran');
    },
    value: 'ext:geoip_IR.dat:ir',
  },
  {
    get label() {
      return i18next.t('ui.china');
    },
    value: 'geoip:cn',
  },
  {
    get label() {
      return i18next.t('ui.russia');
    },
    value: 'ext:geoip_RU.dat:ru',
  },
  {
    get label() {
      return i18next.t('ui.vietnam');
    },
    value: 'geoip:vn',
  },
  {
    get label() {
      return i18next.t('ui.spain');
    },
    value: 'geoip:es',
  },
  {
    get label() {
      return i18next.t('ui.indonesia');
    },
    value: 'geoip:id',
  },
  {
    get label() {
      return i18next.t('ui.ukraine');
    },
    value: 'geoip:ua',
  },
  {
    get label() {
      return i18next.t('ui.tuRkiye');
    },
    value: 'geoip:tr',
  },
  {
    get label() {
      return i18next.t('ui.brazil');
    },
    value: 'geoip:br',
  },
];
export const DOMAINS_OPTIONS = [
  {
    get label() {
      return i18next.t('ui.iran');
    },
    value: 'ext:geosite_IR.dat:ir',
  },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  {
    get label() {
      return i18next.t('ui.china');
    },
    value: 'geosite:cn',
  },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  {
    get label() {
      return i18next.t('ui.russia');
    },
    value: 'ext:geosite_RU.dat:ru-available-only-inside',
  },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
export const BLOCK_DOMAINS_OPTIONS = [
  {
    get label() {
      return i18next.t('ui.adsAll');
    },
    value: 'geosite:category-ads-all',
  },
  {
    get label() {
      return i18next.t('ui.adsIR');
    },
    value: 'ext:geosite_IR.dat:category-ads-all',
  },
  {
    get label() {
      return i18next.t('ui.adsRU');
    },
    value: 'ext:geosite_RU.dat:category-ads-all',
  },
  {
    get label() {
      return i18next.t('ui.malware');
    },
    value: 'ext:geosite_IR.dat:malware',
  },
  {
    get label() {
      return i18next.t('ui.phishing');
    },
    value: 'ext:geosite_IR.dat:phishing',
  },
  {
    get label() {
      return i18next.t('ui.cryptominers');
    },
    value: 'ext:geosite_IR.dat:cryptominers',
  },
  {
    get label() {
      return i18next.t('ui.adult18');
    },
    value: 'geosite:category-porn',
  },
  {
    get label() {
      return i18next.t('ui.iran');
    },
    value: 'ext:geosite_IR.dat:ir',
  },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  {
    get label() {
      return i18next.t('ui.china');
    },
    value: 'geosite:cn',
  },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  {
    get label() {
      return i18next.t('ui.russia');
    },
    value: 'ext:geosite_RU.dat:ru-available-only-inside',
  },
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
// The strategy rides on sockopt: freedom resolves through the socket layer, and
// the settings-level alias makes the core warn on every config load.
export const ipv4Settings = {
  tag: 'IPv4',
  protocol: 'freedom',
  streamSettings: { sockopt: { domainStrategy: 'UseIPv4' } },
};
