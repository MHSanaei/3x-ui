class Node {
    constructor(data) {
        this.id = 0;
        this.name = "";
        this.address = "";
        this.apiKey = "";
        this.status = "unknown";
        this.lastCheck = 0;
        this.createdAt = 0;
        this.updatedAt = 0;
        
        if (data == null) {
            return;
        }
        ObjectUtil.cloneProps(this, data);
    }

    get isOnline() {
        return this.status === "online";
    }

    get isOffline() {
        return this.status === "offline";
    }

    get isError() {
        return this.status === "error";
    }

    get isUnknown() {
        return this.status === "unknown" || !this.status;
    }

    get statusColor() {
        switch (this.status) {
            case 'online': return 'green';
            case 'offline': return 'red';
            case 'error': return 'red';
            default: return 'default';
        }
    }

    get statusIcon() {
        switch (this.status) {
            case 'online': return 'check-circle';
            case 'offline': return 'close-circle';
            case 'error': return 'exclamation-circle';
            default: return 'question-circle';
        }
    }

    get formattedLastCheck() {
        if (!this.lastCheck || this.lastCheck === 0) {
            return '-';
        }
        const date = new Date(this.lastCheck * 1000);
        const now = new Date();
        const diff = Math.floor((now - date) / 1000);
        
        if (diff < 60) return `${diff}s ago`;
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return `${Math.floor(diff / 86400)}d ago`;
    }

    toJson() {
        return {
            id: this.id,
            name: this.name,
            address: this.address,
            apiKey: this.apiKey,
            status: this.status,
            lastCheck: this.lastCheck,
            createdAt: this.createdAt,
            updatedAt: this.updatedAt
        };
    }

    static fromJson(json) {
        return new Node(json);
    }
}
