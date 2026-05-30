export interface OutboundRow {
  key: number;
  tag?: string;
  protocol?: string;
  streamSettings?: { network?: string; security?: string };
  settings?: Record<string, unknown>;
}
