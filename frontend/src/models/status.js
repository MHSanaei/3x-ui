import { NumberFormatter } from '@/utils';

export class CurTotal {
  constructor(current, total) {
    this.current = current;
    this.total = total;
  }

  get percent() {
    if (this.total === 0) return 0;
    return NumberFormatter.toFixed((this.current / this.total) * 100, 2);
  }

  get color() {
    // Match AD-Vue 4's semantic palette so the gauges fit the
    // global blue/gold/red theme instead of the legacy teal/orange.
    const p = this.percent;
    if (p < 80) return '#1677ff'; // primary
    if (p < 90) return '#faad14'; // warning
    return '#ff4d4f';             // danger
  }
}

const XRAY_STATE_COLORS = {
  running: 'green',
  stop: 'orange',
  error: 'red',
};

const XRAY_STATE_MESSAGES = {
  running: 'Xray is running',
  stop: 'Xray is stopped',
  error: 'Xray error',
};

export class Status {
  constructor(data) {
    this.cpu = new CurTotal(0, 0);
    this.cpuCores = 0;
    this.logicalPro = 0;
    this.cpuSpeedMhz = 0;
    this.disk = new CurTotal(0, 0);
    this.loads = [0, 0, 0];
    this.mem = new CurTotal(0, 0);
    this.netIO = { up: 0, down: 0 };
    this.netTraffic = { sent: 0, recv: 0 };
    this.publicIP = { ipv4: 0, ipv6: 0 };
    this.swap = new CurTotal(0, 0);
    this.tcpCount = 0;
    this.udpCount = 0;
    this.uptime = 0;
    this.appUptime = 0;
    this.appStats = { threads: 0, mem: 0, uptime: 0 };
    this.xray = { state: 'stop', stateMsg: '', errorMsg: '', version: '', color: '' };

    if (data == null) return;

    this.cpu = new CurTotal(data.cpu, 100);
    this.cpuCores = data.cpuCores;
    this.logicalPro = data.logicalPro;
    this.cpuSpeedMhz = data.cpuSpeedMhz;
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
    this.xray.stateMsg = XRAY_STATE_MESSAGES[this.xray.state] ?? 'Unknown';
  }
}
