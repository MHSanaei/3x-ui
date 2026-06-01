import type { SubSettings } from '../useInbounds';

export interface ClientStats {
  email: string;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  enable?: boolean;
}

export interface ClientSetting {
  email?: string;
  id?: string;
  security?: string;
  password?: string;
  flow?: string;
  subId?: string;
  totalGB?: number;
  expiryTime?: number;
  comment?: string;
  tgId?: string;
  enable?: boolean;
  limitIp?: number;
  created_at?: number;
  updated_at?: number;
}

export interface InboundInfo {
  protocol: string;
  clients: ClientSetting[];
  settings: Record<string, unknown>;
  isTcp: boolean;
  isWs: boolean;
  isHttpupgrade: boolean;
  isXHTTP: boolean;
  isGrpc: boolean;
  isSSMultiUser: boolean;
  isSS2022: boolean;
  isVlessTlsFlow: boolean;
  host: string | null;
  path: string | null;
  serviceName: string;
  serverName: string;
  stream: {
    network: string;
    security: string;
    xhttp?: { mode?: string };
    grpc?: { multiMode?: boolean };
  };
}

export interface DBInboundLike {
  id: number;
  address: string;
  port: number;
  listen: string;
  protocol: string;
  remark: string;
  enable?: boolean;
  isVMess?: boolean;
  isVLess?: boolean;
  isTrojan?: boolean;
  isSS?: boolean;
  isMixed?: boolean;
  isHTTP?: boolean;
  isWireguard?: boolean;
  settings: unknown;
  streamSettings: unknown;
  sniffing: unknown;
  clientStats?: ClientStats[];
}

export interface InboundInfoModalProps {
  open: boolean;
  onClose: () => void;
  dbInbound: DBInboundLike | null;
  clientIndex?: number;
  remarkModel?: string;
  expireDiff?: number;
  trafficDiff?: number;
  ipLimitEnable?: boolean;
  tgBotEnable?: boolean;
  nodeAddress?: string;
  subSettings?: SubSettings;
  lastOnlineMap?: Record<string, number>;
}
