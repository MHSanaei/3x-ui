// Slim TS surface for what the React client pages need. The full
// inbound model (StreamSettings, RealityStreamSettings, etc.) still
// lives in inbound.js for the remaining vue entries; this file ports
// only the enum-like constants the React clients page consumes.

export const TLS_FLOW_CONTROL = {
  xtls_rprx_vision: 'xtls-rprx-vision',
  xtls_rprx_vision_udp443: 'xtls-rprx-vision-udp443',
} as const;
