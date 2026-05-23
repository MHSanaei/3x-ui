import { NumberFormatter } from '@/utils';

export class CurTotal {
  current: number;
  total: number;

  constructor(current: number, total: number) {
    this.current = current;
    this.total = total;
  }

  get percent(): number {
    if (this.total === 0) return 0;
    return NumberFormatter.toFixed((this.current / this.total) * 100, 2);
  }

  get color(): string {
    const p = this.percent;
    if (p < 80) return '#1677ff';
    if (p < 90) return '#faad14';
    return '#ff4d4f';
  }
}

const XRAY_STATE_COLORS: Record<string, string> = {
  running: 'green',
  stop: 'orange',
  error: 'red',
};

export interface NetIO {
  up: number;
  down: number;
}

export interface NetTraffic {
  sent: number;
  recv: number;
}

export interface PublicIP {
  ipv4: string | number;
  ipv6: string | number;
}

export interface AppStats {
  threads: number;
  mem: number;
  uptime: number;
}

export interface XrayInfo {
  state: 'running' | 'stop' | 'error' | string;
  errorMsg: string;
  version: string;
  color: string;
}

interface StatusInput {
  cpu?: number;
  cpuCores?: number;
  logicalPro?: number;
  cpuSpeedMhz?: number;
  disk?: { current?: number; total?: number };
  loads?: number[];
  mem?: { current?: number; total?: number };
  netIO?: NetIO;
  netTraffic?: NetTraffic;
  publicIP?: PublicIP;
  swap?: { current?: number; total?: number };
  tcpCount?: number;
  udpCount?: number;
  uptime?: number;
  appUptime?: number;
  appStats?: AppStats;
  xray?: Partial<XrayInfo>;
}

export class Status {
  cpu: CurTotal = new CurTotal(0, 0);
  cpuCores = 0;
  logicalPro = 0;
  cpuSpeedMhz = 0;
  disk: CurTotal = new CurTotal(0, 0);
  loads: number[] = [0, 0, 0];
  mem: CurTotal = new CurTotal(0, 0);
  netIO: NetIO = { up: 0, down: 0 };
  netTraffic: NetTraffic = { sent: 0, recv: 0 };
  publicIP: PublicIP = { ipv4: 0, ipv6: 0 };
  swap: CurTotal = new CurTotal(0, 0);
  tcpCount = 0;
  udpCount = 0;
  uptime = 0;
  appUptime = 0;
  appStats: AppStats = { threads: 0, mem: 0, uptime: 0 };
  xray: XrayInfo = { state: 'stop', errorMsg: '', version: '', color: '' };

  constructor(data?: StatusInput | null) {
    if (data == null) return;

    this.cpu = new CurTotal(data.cpu ?? 0, 100);
    this.cpuCores = data.cpuCores ?? 0;
    this.logicalPro = data.logicalPro ?? 0;
    this.cpuSpeedMhz = data.cpuSpeedMhz ?? 0;
    this.disk = new CurTotal(data.disk?.current ?? 0, data.disk?.total ?? 0);
    this.loads = (data.loads || [0, 0, 0]).map((v) => NumberFormatter.toFixed(v, 2));
    this.mem = new CurTotal(data.mem?.current ?? 0, data.mem?.total ?? 0);
    this.netIO = data.netIO ?? this.netIO;
    this.netTraffic = data.netTraffic ?? this.netTraffic;
    this.publicIP = data.publicIP ?? this.publicIP;
    this.swap = new CurTotal(data.swap?.current ?? 0, data.swap?.total ?? 0);
    this.tcpCount = data.tcpCount ?? 0;
    this.udpCount = data.udpCount ?? 0;
    this.uptime = data.uptime ?? 0;
    this.appUptime = data.appUptime ?? 0;
    this.appStats = data.appStats ?? this.appStats;
    this.xray = { ...this.xray, ...(data.xray || {}) };
    this.xray.color = XRAY_STATE_COLORS[this.xray.state] ?? 'gray';
  }
}
