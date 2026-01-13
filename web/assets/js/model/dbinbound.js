class DBInbound {

    constructor(data) {
        this.id = 0;
        this.userId = 0;
        this.up = 0;
        this.down = 0;
        this.total = 0;
        this.allTime = 0;
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
        this.nodeId = null; // Node ID for multi-node mode - DEPRECATED: kept only for backward compatibility, use nodeIds instead
        this.nodeIds = []; // Node IDs array for multi-node mode - use this for multi-node support
        if (data == null) {
            return;
        }
        ObjectUtil.cloneProps(this, data);
        // Ensure nodeIds is always an array (even if empty)
        // Priority: use nodeIds if available, otherwise convert from deprecated nodeId
        // First check if nodeIds exists and is an array (even if empty)
        // Handle nodeIds from API response - it should be an array
        if (this.nodeIds !== null && this.nodeIds !== undefined) {
            if (Array.isArray(this.nodeIds)) {
                // nodeIds is already an array - ensure all values are numbers
                if (this.nodeIds.length > 0) {
                    this.nodeIds = this.nodeIds.map(id => {
                        // Convert string to number if needed
                        const numId = typeof id === 'string' ? parseInt(id, 10) : id;
                        return numId;
                    }).filter(id => !isNaN(id) && id > 0);
                } else {
                    // Empty array is valid
                    this.nodeIds = [];
                }
            } else {
                // nodeIds exists but is not an array - try to convert
                // This shouldn't happen if API returns correct format, but handle it anyway
                const nodeId = typeof this.nodeIds === 'string' ? parseInt(this.nodeIds, 10) : this.nodeIds;
                this.nodeIds = !isNaN(nodeId) && nodeId > 0 ? [nodeId] : [];
            }
        } else if (this.nodeId !== null && this.nodeId !== undefined) {
            // Convert deprecated nodeId to nodeIds array (backward compatibility)
            const nodeId = typeof this.nodeId === 'string' ? parseInt(this.nodeId, 10) : this.nodeId;
            this.nodeIds = !isNaN(nodeId) && nodeId > 0 ? [nodeId] : [];
        } else {
            // No nodes assigned - ensure empty array
            this.nodeIds = [];
        }
        // Ensure nodeIds is never null or undefined - always an array
        if (!Array.isArray(this.nodeIds)) {
            this.nodeIds = [];
        }
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
        return moment(this.expiryTime);
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

    toInbound() {
        let settings = {};
        if (!ObjectUtil.isEmpty(this.settings)) {
            settings = JSON.parse(this.settings);
        }

        let streamSettings = {};
        if (!ObjectUtil.isEmpty(this.streamSettings)) {
            streamSettings = JSON.parse(this.streamSettings);
        }

        let sniffing = {};
        if (!ObjectUtil.isEmpty(this.sniffing)) {
            sniffing = JSON.parse(this.sniffing);
        }

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
        // Include nodeIds if available (for multi-node mode)
        if (this.nodeIds && Array.isArray(this.nodeIds) && this.nodeIds.length > 0) {
            config.nodeIds = this.nodeIds;
        } else if (this.nodeId !== null && this.nodeId !== undefined) {
            // Backward compatibility: convert single nodeId to nodeIds array
            config.nodeIds = [this.nodeId];
        }
        return Inbound.fromJson(config);
    }

    isMultiUser() {
        switch (this.protocol) {
            case Protocols.VMESS:
            case Protocols.VLESS:
            case Protocols.TROJAN:
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
                return true;
            default:
                return false;
        }
    }

    genInboundLinks(remarkModel) {
        const inbound = this.toInbound();
        return inbound.genInboundLinks(this.remark, remarkModel);
    }
}