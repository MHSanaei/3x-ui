// Based on database/model/model.go and xray/traffic.go

export interface ClientTraffic {
  id: number;
  inboundId: number;
  enable: boolean;
  email: string;
  up: number;
  down: number;
  expiryTime: number;
  total: number;
  reset?: number; // Optional, as it might not always be present
}

export type Protocol = "vmess" | "vless" | "trojan" | "shadowsocks" | "dokodemo-door" | "socks" | "http" | "wireguard";

export interface Inbound {
  id: number;
  userId: number; // Assuming not directly used in UI but good to have
  up: number;
  down: number;
  total: number; // Overall total for the inbound itself
  remark: string;
  enable: boolean;
  expiryTime: number; // Overall expiry for the inbound itself
  clientStats: ClientTraffic[]; // For clients directly associated with this inbound's settings

  // Config part
  listen: string;
  port: number;
  protocol: Protocol;
  settings: string; // JSON string, to be parsed for client details if needed
  streamSettings: string; // JSON string
  tag: string;
  sniffing: string; // JSON string
  // allocate: string; // JSON string - allocate seems to be missing in controller/model for direct form binding, might be part of settings
}

// For the list API, it seems clientStats are eagerly loaded.
export type InboundFromList = Inbound; // Alias for clarity

export interface InboundClientIP { // From model.InboundClientIps
  id: number;
  clientEmail: string;
  ips: string;
}

// For client details within Inbound.settings JSON string
export interface ClientSetting {
  id?: string; // UUID for vmess/vless
  password?: string; // for trojan
  email: string;
  flow?: string;
  encryption?: string; // for shadowsocks method
  totalGB?: number; // quota in GB for client
  expiryTime?: number; // client-specific expiry
  limitIp?: number;
  subId?: string;
  tgId?: string;
  enable?: boolean; // Client specific enable toggle
  comment?: string;
}
