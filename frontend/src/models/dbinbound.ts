/* eslint-disable @typescript-eslint/no-explicit-any */
import dayjs, { type Dayjs } from 'dayjs';
import { ObjectUtil, NumberFormatter, SizeFormatter } from '@/utils';
import { Inbound, Protocols } from './inbound';

export function coerceInboundJsonField(value: unknown): any {
    if (value == null) return {};
    if (typeof value === 'object') return value;
    if (typeof value !== 'string') return {};
    const trimmed = value.trim();
    if (trimmed === '') return {};
    try {
        return JSON.parse(trimmed);
    } catch {
        return {};
    }
}

export class DBInbound {
    id: number;
    userId: number;
    up: number;
    down: number;
    total: number;
    remark: string;
    enable: boolean;
    expiryTime: number;
    trafficReset: string;
    lastTrafficResetTime: number;

    listen: string;
    port: number;
    protocol: string;
    settings: any;
    streamSettings: any;
    tag: string;
    sniffing: any;
    clientStats: any;
    nodeId: number | null;
    fallbackParent: { masterId: number; path: string } | null;

    private _cachedInbound: any = null;
    private _clientStatsMap: Map<string, any> | null = null;

    constructor(data?: any) {
        this.id = 0;
        this.userId = 0;
        this.up = 0;
        this.down = 0;
        this.total = 0;
        this.remark = "";
        this.enable = true;
        this.expiryTime = 0;
        this.trafficReset = "never";
        this.lastTrafficResetTime = 0;

        this.listen = "";
        this.port = 0;
        this.protocol = "";
        this.settings = "";
        this.streamSettings = "";
        this.tag = "";
        this.sniffing = "";
        this.clientStats = "";
        this.nodeId = null;
        this.fallbackParent = null;
        if (data == null) {
            return;
        }
        ObjectUtil.cloneProps(this, data);
    }

    get totalGB(): number {
        return NumberFormatter.toFixed(this.total / SizeFormatter.ONE_GB, 2);
    }

    set totalGB(gb: number) {
        this.total = NumberFormatter.toFixed(gb * SizeFormatter.ONE_GB, 0);
    }

    get isVMess() {
        return this.protocol === Protocols.VMESS;
    }

    get isVLess() {
        return this.protocol === Protocols.VLESS;
    }

    get isTrojan() {
        return this.protocol === Protocols.TROJAN;
    }

    get isSS() {
        return this.protocol === Protocols.SHADOWSOCKS;
    }

    get isMixed() {
        return this.protocol === Protocols.MIXED;
    }

    get isHTTP() {
        return this.protocol === Protocols.HTTP;
    }

    get isWireguard() {
        return this.protocol === Protocols.WIREGUARD;
    }

    get isHysteria() {
        return this.protocol === Protocols.HYSTERIA;
    }

    get address(): string {
        let address = location.hostname;
        if (!ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0") {
            address = this.listen;
        }
        return address;
    }

    get _expiryTime(): Dayjs | null {
        if (this.expiryTime === 0) {
            return null;
        }
        return dayjs(this.expiryTime);
    }

    set _expiryTime(t: Dayjs | null | undefined) {
        if (t == null) {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }

    get isExpiry(): boolean {
        return this.expiryTime < new Date().getTime();
    }

    invalidateCache(): void {
        this._cachedInbound = null;
        this._clientStatsMap = null;
    }

    toInbound(): any {
        if (this._cachedInbound) {
            return this._cachedInbound;
        }

        const settings = coerceInboundJsonField(this.settings);
        const streamSettings = coerceInboundJsonField(this.streamSettings);
        const sniffing = coerceInboundJsonField(this.sniffing);

        const config = {
            port: this.port,
            listen: this.listen,
            protocol: this.protocol,
            settings: settings,
            streamSettings: streamSettings,
            tag: this.tag,
            sniffing: sniffing,
            clientStats: this.clientStats,
        };

        this._cachedInbound = Inbound.fromJson(config);
        return this._cachedInbound;
    }

    getClientStats(email: string): any {
        if (!this._clientStatsMap) {
            this._clientStatsMap = new Map();
            if (this.clientStats && Array.isArray(this.clientStats)) {
                for (const stats of this.clientStats) {
                    this._clientStatsMap.set(stats.email, stats);
                }
            }
        }
        return this._clientStatsMap.get(email);
    }

    isMultiUser(): boolean {
        switch (this.protocol) {
            case Protocols.VMESS:
            case Protocols.VLESS:
            case Protocols.TROJAN:
            case Protocols.HYSTERIA:
                return true;
            case Protocols.SHADOWSOCKS:
                return this.toInbound().isSSMultiUser;
            default:
                return false;
        }
    }

    hasLink(): boolean {
        switch (this.protocol) {
            case Protocols.VMESS:
            case Protocols.VLESS:
            case Protocols.TROJAN:
            case Protocols.SHADOWSOCKS:
            case Protocols.HYSTERIA:
                return true;
            default:
                return false;
        }
    }

    genInboundLinks(remarkModel: string, hostOverride: string = ''): any[] {
        const inbound = this.toInbound();
        return inbound.genInboundLinks(this.remark, remarkModel, hostOverride);
    }
}