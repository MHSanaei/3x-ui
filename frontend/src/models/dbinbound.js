import dayjs from 'dayjs';
import { ObjectUtil, NumberFormatter, SizeFormatter } from '@/utils';
import { Inbound, Protocols } from './inbound.js';

export function coerceInboundJsonField(value) {
    if (value == null) return {};
    if (typeof value === 'object') return value;
    if (typeof value !== 'string') return {};
    const trimmed = value.trim();
    if (trimmed === '') return {};
    try {
        return JSON.parse(trimmed);
    } catch (_e) {
        return {};
    }
}

export class DBInbound {

    constructor(data) {
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
        this.clientStats = ""
        // Optional FK to web/runtime registered Node. null/undefined =
        // local panel; otherwise the inbound lives on the named node.
        this.nodeId = null;
        // Populated by the API when this inbound is a fallback child of
        // a VLESS/Trojan TCP-TLS master. Shape: { masterId, path }.
        this.fallbackParent = null;
        if (data == null) {
            return;
        }
        ObjectUtil.cloneProps(this, data);
    }

    get totalGB() {
        return NumberFormatter.toFixed(this.total / SizeFormatter.ONE_GB, 2);
    }

    set totalGB(gb) {
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

    get address() {
        let address = location.hostname;
        if (!ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0") {
            address = this.listen;
        }
        return address;
    }

    get _expiryTime() {
        if (this.expiryTime === 0) {
            return null;
        }
        return dayjs(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null) {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }

    get isExpiry() {
        return this.expiryTime < new Date().getTime();
    }

    invalidateCache() {
        this._cachedInbound = null;
        this._clientStatsMap = null;
    }

    toInbound() {
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

    getClientStats(email) {
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

    isMultiUser() {
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

    hasLink() {
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

    genInboundLinks(remarkModel, hostOverride = '') {
        const inbound = this.toInbound();
        return inbound.genInboundLinks(this.remark, remarkModel, hostOverride);
    }
}