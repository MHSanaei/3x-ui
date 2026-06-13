import type { NodeRecord } from '@/api/queries/useNodesQuery';

export interface StreamHints {
  network: string;
  isTls: boolean;
  isReality: boolean;
}

export type ProtocolFlags = {
  isVMess?: boolean;
  isVLess?: boolean;
  isTrojan?: boolean;
  isSS?: boolean;
  isHysteria?: boolean;
  isMixed?: boolean;
  isHTTP?: boolean;
  isWireguard?: boolean;
  isTunnel?: boolean;
};

export interface DBInboundRecord extends ProtocolFlags {
  id: number;
  enable: boolean;
  remark: string;
  subSortIndex: number;
  port: number;
  protocol: string;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  _expiryTime: { valueOf(): number } | null;
  nodeId?: number | null;
  settings: unknown;
  streamSettings: unknown;
}

export interface ClientCountEntry {
  clients: number;
  active: string[];
  deactive: string[];
  depleted: string[];
  expiring: string[];
  online: string[];
}

export interface InboundSpeedEntry {
  up: number;
  down: number;
}

export type RowAction =
  | 'edit'
  | 'showInfo'
  | 'qrcode'
  | 'export'
  | 'subs'
  | 'clipboard'
  | 'delete'
  | 'resetTraffic'
  | 'delAllClients'
  | 'clone';

export type GeneralAction = 'import' | 'export' | 'subs' | 'resetInbounds';

export interface InboundListProps {
  dbInbounds: DBInboundRecord[];
  clientCount: Record<number, ClientCountEntry>;
  onlineClients: string[];
  lastOnlineMap: Record<string, number>;
  inboundSpeed: Record<number, InboundSpeedEntry>;
  expireDiff: number;
  trafficDiff: number;
  pageSize: number;
  isMobile: boolean;
  subEnable: boolean;
  nodesById: Map<number, NodeRecord>;
  hasActiveNode: boolean;
  onAddInbound: () => void;
  onGeneralAction: (key: GeneralAction) => void;
  onRowAction: (action: { key: RowAction; dbInbound: DBInboundRecord }) => void;
  onBulkDelete: (ids: number[]) => Promise<boolean>;
}
