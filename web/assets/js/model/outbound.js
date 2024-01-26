const Protocols = {
    Freedom: "freedom",
    Blackhole: "blackhole",
    DNS: "dns",
    VMess: "vmess",
    VLESS: "vless",
    Trojan: "trojan",
    Shadowsocks: "shadowsocks",
    Socks: "socks",
    HTTP: "http",
    Wireguard: "wireguard"
};

const SSMethods = {
    AES_256_GCM: 'aes-256-gcm',
    AES_128_GCM: 'aes-128-gcm',
    CHACHA20_POLY1305: 'chacha20-poly1305',
    CHACHA20_IETF_POLY1305: 'chacha20-ietf-poly1305',
    XCHACHA20_POLY1305: 'xchacha20-poly1305',
    XCHACHA20_IETF_POLY1305: 'xchacha20-ietf-poly1305',
    BLAKE3_AES_128_GCM: '2022-blake3-aes-128-gcm',
    BLAKE3_AES_256_GCM: '2022-blake3-aes-256-gcm',
    BLAKE3_CHACHA20_POLY1305: '2022-blake3-chacha20-poly1305',
};

const TLS_FLOW_CONTROL = {
    VISION: "xtls-rprx-vision",
    VISION_UDP443: "xtls-rprx-vision-udp443",
};

const UTLS_FINGERPRINT = {
    UTLS_CHROME: "chrome",
    UTLS_FIREFOX: "firefox",
    UTLS_SAFARI: "safari",
    UTLS_IOS: "ios",
    UTLS_android: "android",
    UTLS_EDGE: "edge",
    UTLS_360: "360",
    UTLS_QQ: "qq",
    UTLS_RANDOM: "random",
    UTLS_RANDOMIZED: "randomized",
};

const ALPN_OPTION = {
    H3: "h3",
    H2: "h2",
    HTTP1: "http/1.1",
};

const OutboundDomainStrategies = [
    "AsIs",
    "UseIP",
    "UseIPv4",
    "UseIPv6"
];

const WireguardDomainStrategy = [
    "ForceIP",
    "ForceIPv4",
    "ForceIPv4v6",
    "ForceIPv6",
    "ForceIPv6v4"
];

Object.freeze(Protocols);
Object.freeze(SSMethods);
Object.freeze(TLS_FLOW_CONTROL);
Object.freeze(ALPN_OPTION);
Object.freeze(OutboundDomainStrategies);
Object.freeze(WireguardDomainStrategy);

class CommonClass {

    static toJsonArray(arr) {
        return arr.map(obj => obj.toJson());
    }

    static fromJson() {
        return new CommonClass();
    }

    toJson() {
        return this;
    }

    toString(format=true) {
        return format ? JSON.stringify(this.toJson(), null, 2) : JSON.stringify(this.toJson());
    }
}

class TcpStreamSettings extends CommonClass {
    constructor(type='none', host, path) {
        super();
        this.type = type;
        this.host = host;
        this.path = path;
    }

    static fromJson(json={}) {
        let header = json.header;
        if (!header) return new TcpStreamSettings();
        if(header.type == 'http' && header.request){
            return new TcpStreamSettings(
                header.type,
                header.request.headers.Host.join(','),
                header.request.path.join(','),
            );
        }
        return new TcpStreamSettings(header.type,'','');
    }

    toJson() {
        return {
            header: {
                type: this.type,
                request: this.type === 'http' ? {
                    headers: {
                        Host: ObjectUtil.isEmpty(this.host) ? [] : this.host.split(',')
                    },
                    path: ObjectUtil.isEmpty(this.path) ? ["/"] : this.path.split(',')
                } : undefined,
            }
        };
    }
}

class KcpStreamSettings extends CommonClass {
    constructor(mtu=1350, tti=20,
                uplinkCapacity=5,
                downlinkCapacity=20,
                congestion=false,
                readBufferSize=2,
                writeBufferSize=2,
                type='none',
                seed='',
                ) {
        super();
        this.mtu = mtu;
        this.tti = tti;
        this.upCap = uplinkCapacity;
        this.downCap = downlinkCapacity;
        this.congestion = congestion;
        this.readBuffer = readBufferSize;
        this.writeBuffer = writeBufferSize;
        this.type = type;
        this.seed = seed;
    }

    static fromJson(json={}) {
        return new KcpStreamSettings(
            json.mtu,
            json.tti,
            json.uplinkCapacity,
            json.downlinkCapacity,
            json.congestion,
            json.readBufferSize,
            json.writeBufferSize,
            ObjectUtil.isEmpty(json.header) ? 'none' : json.header.type,
            json.seed,
        );
    }

    toJson() {
        return {
            mtu: this.mtu,
            tti: this.tti,
            uplinkCapacity: this.upCap,
            downlinkCapacity: this.downCap,
            congestion: this.congestion,
            readBufferSize: this.readBuffer,
            writeBufferSize: this.writeBuffer,
            header: {
                type: this.type,
            },
            seed: this.seed,
        };
    }
}

class WsStreamSettings extends CommonClass {
    constructor(path='/', host='') {
        super();
        this.path = path;
        this.host = host;
    }

    static fromJson(json={}) {
        return new WsStreamSettings(
            json.path,
            json.headers && !ObjectUtil.isEmpty(json.headers.Host) ? json.headers.Host : '',
        );
    }

    toJson() {
        return {
            path: this.path,
            headers: ObjectUtil.isEmpty(this.host) ? undefined : {Host: this.host},
        };
    }
}

class HttpStreamSettings extends CommonClass {
    constructor(path='/', host='') {
        super();
        this.path = path;
        this.host = host;
    }

    static fromJson(json={}) {
        return new HttpStreamSettings(
            json.path,
            json.host ? json.host.join(',') : '',
        );
    }

    toJson() {
        return {
            path: this.path,
            host: ObjectUtil.isEmpty(this.host) ? [''] : this.host.split(','),
        }
    }
}

class QuicStreamSettings extends CommonClass {
    constructor(security='none',
                key='', type='none') {
        super();
        this.security = security;
        this.key = key;
        this.type = type;
    }

    static fromJson(json={}) {
        return new QuicStreamSettings(
            json.security,
            json.key,
            json.header ? json.header.type : 'none',
        );
    }

    toJson() {
        return {
            security: this.security,
            key: this.key,
            header: {
                type: this.type,
            }
        }
    }
}

class GrpcStreamSettings extends CommonClass {
    constructor(serviceName="", multiMode=false) {
        super();
        this.serviceName = serviceName;
        this.multiMode = multiMode;
    }

    static fromJson(json={}) {
        return new GrpcStreamSettings(json.serviceName, json.multiMode);
    }

    toJson() {
        return {
            serviceName: this.serviceName,
            multiMode: this.multiMode,
        }
    }
}

class TlsStreamSettings extends CommonClass {
    constructor(serverName='',
                alpn=[],
                fingerprint = '',
                allowInsecure = false) {
        super();
        this.serverName = serverName;
        this.alpn = alpn;
        this.fingerprint = fingerprint;
        this.allowInsecure = allowInsecure;
    }

    static fromJson(json={}) {
        return new TlsStreamSettings(
            json.serverName,
            json.alpn,
            json.fingerprint,
            json.allowInsecure,
        );
    }

    toJson() {
        return {
            serverName: this.serverName,
            alpn: this.alpn,
            fingerprint: this.fingerprint,
            allowInsecure: this.allowInsecure,
        };
    }
}

class RealityStreamSettings extends CommonClass {
    constructor(publicKey = '', fingerprint = '', serverName = '', shortId = '', spiderX = '/') {
        super();
        this.publicKey = publicKey;
        this.fingerprint = fingerprint;
        this.serverName = serverName;
        this.shortId = shortId
        this.spiderX = spiderX;
    }
    static fromJson(json = {}) {
        return new RealityStreamSettings(
            json.publicKey,
            json.fingerprint,
            json.serverName,
            json.shortId,
            json.spiderX,
        );
    }
    toJson() {
        return {
            publicKey: this.publicKey,
            fingerprint: this.fingerprint,
            serverName: this.serverName,
            shortId: this.shortId,
            spiderX: this.spiderX,
        };
    }
};

class StreamSettings extends CommonClass {
    constructor(network='tcp',
                security='none',
                tlsSettings=new TlsStreamSettings(),
                realitySettings = new RealityStreamSettings(),
                tcpSettings=new TcpStreamSettings(),
                kcpSettings=new KcpStreamSettings(),
                wsSettings=new WsStreamSettings(),
                httpSettings=new HttpStreamSettings(),
                quicSettings=new QuicStreamSettings(),
                grpcSettings=new GrpcStreamSettings(),
                ) {
        super();
        this.network = network;
        this.security = security;
        this.tls = tlsSettings;
        this.reality = realitySettings;
        this.tcp = tcpSettings;
        this.kcp = kcpSettings;
        this.ws = wsSettings;
        this.http = httpSettings;
        this.quic = quicSettings;
        this.grpc = grpcSettings;
    }
    
    get isTls() {
        return this.security === 'tls';
    }

    get isReality() {
        return this.security === "reality";
    }

    static fromJson(json={}) {
        return new StreamSettings(
            json.network,
            json.security,
            TlsStreamSettings.fromJson(json.tlsSettings),
            RealityStreamSettings.fromJson(json.realitySettings),
            TcpStreamSettings.fromJson(json.tcpSettings),
            KcpStreamSettings.fromJson(json.kcpSettings),
            WsStreamSettings.fromJson(json.wsSettings),
            HttpStreamSettings.fromJson(json.httpSettings),
            QuicStreamSettings.fromJson(json.quicSettings),
            GrpcStreamSettings.fromJson(json.grpcSettings),
        );
    }

    toJson() {
        const network = this.network;
        return {
            network: network,
            security: this.security,
            tlsSettings: this.security == 'tls' ? this.tls.toJson() : undefined,
            realitySettings: this.security == 'reality' ? this.reality.toJson() : undefined,
            tcpSettings: network === 'tcp' ? this.tcp.toJson() : undefined,
            kcpSettings: network === 'kcp' ? this.kcp.toJson() : undefined,
            wsSettings: network === 'ws' ? this.ws.toJson() : undefined,
            httpSettings: network === 'http' ? this.http.toJson() : undefined,
            quicSettings: network === 'quic' ? this.quic.toJson() : undefined,
            grpcSettings: network === 'grpc' ? this.grpc.toJson() : undefined,
        };
    }
}

class Outbound extends CommonClass {
    constructor(
        tag='',
        protocol=Protocols.VMess,
        settings=null,
        streamSettings = new StreamSettings(),
    ) {
        super();
        this.tag = tag;
        this._protocol = protocol;
        this.settings = settings == null ? Outbound.Settings.getSettings(protocol) : settings;
        this.stream = streamSettings;
    }

    get protocol() {
        return this._protocol;
    }

    set protocol(protocol) {
        this._protocol = protocol;
        this.settings = Outbound.Settings.getSettings(protocol);
        this.stream = new StreamSettings();
    }

    canEnableTls() {
        if (![Protocols.VMess, Protocols.VLESS, Protocols.Trojan].includes(this.protocol)) return false;
        return ["tcp", "ws", "http", "quic", "grpc"].includes(this.stream.network);
    }

    //this is used for xtls-rprx-vision
    canEnableTlsFlow() {
        if ((this.stream.security != 'none') && (this.stream.network === "tcp")) {
            return this.protocol === Protocols.VLESS;
        }
        return false;
    }

    canEnableReality() {
        if (![Protocols.VLESS, Protocols.Trojan].includes(this.protocol)) return false;
        return ["tcp", "http", "grpc"].includes(this.stream.network);
    }

    canEnableStream() {
        return [Protocols.VMess, Protocols.VLESS, Protocols.Trojan, Protocols.Shadowsocks].includes(this.protocol);
    }

    hasVnext() {
        return [Protocols.VMess, Protocols.VLESS].includes(this.protocol);
    }

    hasServers() {
        return [Protocols.Trojan, Protocols.Shadowsocks, Protocols.Socks, Protocols.HTTP].includes(this.protocol);
    }

    hasAddressPort() {
        return [
            Protocols.DNS,
            Protocols.VMess,
            Protocols.VLESS,
            Protocols.Trojan,
            Protocols.Shadowsocks,
            Protocols.Socks,
            Protocols.HTTP
        ].includes(this.protocol);
    }

    hasUsername() {
        return [Protocols.Socks, Protocols.HTTP].includes(this.protocol);
    }

    static fromJson(json={}) {
        return new Outbound(
            json.tag,
            json.protocol,
            Outbound.Settings.fromJson(json.protocol, json.settings),
            StreamSettings.fromJson(json.streamSettings),
        )
    }

    toJson() {
        return {
            tag: this.tag == '' ? undefined : this.tag,
            protocol: this.protocol,
            settings: this.settings instanceof CommonClass ? this.settings.toJson() : this.settings,
            streamSettings: this.canEnableStream() ? this.stream.toJson() : undefined,
        };
    }

    static fromLink(link) {
        data = link.split('://');
        if(data.length !=2) return null;
        switch(data[0].toLowerCase()){
            case Protocols.VMess:
                return this.fromVmessLink(JSON.parse(Base64.decode(data[1])));
            case Protocols.VLESS:
            case Protocols.Trojan:
            case 'ss':
                return this.fromParamLink(link);
            default:
                return null;
        }
    }

    static fromVmessLink(json={}){
        let stream = new StreamSettings(json.net, json.tls);

        let network = json.net;
        if (network === 'tcp') {
            stream.tcp = new TcpStreamSettings(
                json.type,
                json.host ?? '',
                json.path ?? '');
        } else if (network === 'kcp') {
            stream.kcp = new KcpStreamSettings();
            stream.type = json.type;
            stream.seed = json.path;
        } else if (network === 'ws') {
            stream.ws = new WsStreamSettings(json.path,json.host);
        } else if (network === 'http' || network == 'h2') {
            stream.network = 'http'
            stream.http = new HttpStreamSettings(
                json.path,
                json.host);
        } else if (network === 'quic') {
            stream.quic = new QuicStreamSettings(
                json.host ? json.host : 'none',
                json.path,
                json.type ? json.type : 'none');
        } else if (network === 'grpc') {
            stream.grpc = new GrpcStreamSettings(json.path, json.type == 'multi');
        }

        if(json.tls && json.tls == 'tls'){
            stream.tls = new TlsStreamSettings(
                json.sni,
                json.alpn ? json.alpn.split(',') : [],
                json.fp,
                json.allowInsecure);
        }


        return new Outbound(json.ps, Protocols.VMess, new Outbound.VmessSettings(json.add, json.port, json.id), stream);
    }

    static fromParamLink(link){
        const url = new URL(link);
        let type = url.searchParams.get('type');
        let security = url.searchParams.get('security') ?? 'none';
        let stream = new StreamSettings(type, security);

        let headerType = url.searchParams.get('headerType');
        let host = url.searchParams.get('host');
        let path = url.searchParams.get('path');

        if (type === 'tcp') {
            stream.tcp = new TcpStreamSettings(headerType ?? 'none', host, path);
        } else if (type === 'kcp') {
            stream.kcp = new KcpStreamSettings();
            stream.kcp.type = headerType ?? 'none';
            stream.kcp.seed = path;
        } else if (type === 'ws') {
            stream.ws = new WsStreamSettings(path,host);
        } else if (type === 'http' || type == 'h2') {
            stream.http = new HttpStreamSettings(path,host);
        } else if (type === 'quic') {
            stream.quic = new QuicStreamSettings(
                url.searchParams.get('quicSecurity') ?? 'none',
                url.searchParams.get('key') ?? '',
                headerType ?? 'none');
        } else if (type === 'grpc') {
            stream.grpc = new GrpcStreamSettings(url.searchParams.get('serviceName') ?? '', url.searchParams.get('mode') == 'multi');
        }

        if(security == 'tls'){
            let fp=url.searchParams.get('fp') ?? 'none';
            let alpn=url.searchParams.get('alpn');
            let allowInsecure=url.searchParams.get('allowInsecure');
            let sni=url.searchParams.get('sni') ?? '';
            stream.tls = new TlsStreamSettings(sni, alpn ? alpn.split(',') : [], fp, allowInsecure == 1);
        }

        if(security == 'reality'){
            let pbk=url.searchParams.get('pbk');
            let fp=url.searchParams.get('fp');
            let sni=url.searchParams.get('sni') ?? '';
            let sid=url.searchParams.get('sid') ?? '';
            let spx=url.searchParams.get('spx') ?? '';
            stream.reality  = new RealityStreamSettings(pbk, fp, sni, sid, spx);
        }

        let data = link.split('?');
        if(data.length != 2) return null;

        const regex = /([^@]+):\/\/([^@]+)@([^:]+):(\d+)\?(.*)$/;
        const match = link.match(regex);

        if (!match) return null;
        let [, protocol, userData, address, port, ] = match;
        port *= 1;
        if(protocol == 'ss') {
            protocol = 'shadowsocks';
            userData = atob(userData).split(':');
        }
        var settings;
        switch(protocol){
            case Protocols.VLESS:
                settings = new Outbound.VLESSSettings(address, port, userData, url.searchParams.get('flow') ?? '');
                break;
            case Protocols.Trojan:
                settings = new Outbound.TrojanSettings(address, port, userData);
                break;
            case Protocols.Shadowsocks:
                let method = userData.splice(0,1)[0];
                settings = new Outbound.ShadowsocksSettings(address, port, userData.join(":"), method, true);
                break;
            default:
                return null;
        }
        let remark = decodeURIComponent(url.hash);
        // Remove '#' from url.hash
        remark = remark.length > 0 ? remark.substring(1) : 'out-' + protocol + '-' + port;
        return new Outbound(remark, protocol, settings, stream);
    }
}

Outbound.Settings = class extends CommonClass {
    constructor(protocol) {
        super();
        this.protocol = protocol;
    }

    static getSettings(protocol) {
        switch (protocol) {
            case Protocols.Freedom: return new Outbound.FreedomSettings();
            case Protocols.Blackhole: return new Outbound.BlackholeSettings();
            case Protocols.DNS: return new Outbound.DNSSettings();
            case Protocols.VMess: return new Outbound.VmessSettings();
            case Protocols.VLESS: return new Outbound.VLESSSettings();
            case Protocols.Trojan: return new Outbound.TrojanSettings();
            case Protocols.Shadowsocks: return new Outbound.ShadowsocksSettings();
            case Protocols.Socks: return new Outbound.SocksSettings();
            case Protocols.HTTP: return new Outbound.HttpSettings();
            case Protocols.Wireguard: return new Outbound.WireguardSettings();
            default: return null;
        }
    }

    static fromJson(protocol, json) {
        switch (protocol) {
            case Protocols.Freedom: return Outbound.FreedomSettings.fromJson(json);
            case Protocols.Blackhole: return Outbound.BlackholeSettings.fromJson(json);
            case Protocols.DNS: return Outbound.DNSSettings.fromJson(json);
            case Protocols.VMess: return Outbound.VmessSettings.fromJson(json);
            case Protocols.VLESS: return Outbound.VLESSSettings.fromJson(json);
            case Protocols.Trojan: return Outbound.TrojanSettings.fromJson(json);
            case Protocols.Shadowsocks: return Outbound.ShadowsocksSettings.fromJson(json);
            case Protocols.Socks: return Outbound.SocksSettings.fromJson(json);
            case Protocols.HTTP: return Outbound.HttpSettings.fromJson(json);
            case Protocols.Wireguard: return Outbound.WireguardSettings.fromJson(json);
            default: return null;
        }
    }

    toJson() {
        return {};
    }
};
Outbound.FreedomSettings = class extends CommonClass {
    constructor(domainStrategy='', fragment={}) {
        super();
        this.domainStrategy = domainStrategy;
        this.fragment = fragment;
    }

    static fromJson(json={}) {
        return new Outbound.FreedomSettings(
            json.domainStrategy,
            json.fragment ? Outbound.FreedomSettings.Fragment.fromJson(json.fragment) : undefined,
        );
    }

    toJson() {
        return {
            domainStrategy: ObjectUtil.isEmpty(this.domainStrategy) ? undefined : this.domainStrategy,
            fragment: Object.keys(this.fragment).length === 0 ? undefined : this.fragment,
        };
    }
};
Outbound.FreedomSettings.Fragment = class extends CommonClass {
    constructor(packets='1-3',length='',interval=''){
        super();
        this.packets = packets;
        this.length = length;
        this.interval = interval;
    }

    static fromJson(json={}) {
        return new Outbound.FreedomSettings.Fragment(
            json.packets,
            json.length,
            json.interval,
        );
    }
};
Outbound.BlackholeSettings = class extends CommonClass {
    constructor(type) {
        super();
        this.type;
    }

    static fromJson(json={}) {
        return new Outbound.BlackholeSettings(
            json.response ? json.response.type : undefined,
        );
    }

    toJson() {
        return {
            response: ObjectUtil.isEmpty(this.type) ? undefined : {type: this.type},
        };
    }
};
Outbound.DNSSettings = class extends CommonClass {
    constructor(network='udp', address='1.1.1.1', port=53) {
        super();
        this.network = network;
        this.address = address;
        this.port = port;
    }

    static fromJson(json={}){
        return new Outbound.DNSSettings(
            json.network,
            json.address,
            json.port,
        );
    }
};
Outbound.VmessSettings = class extends CommonClass {
    constructor(address, port, id) {
        super();
        this.address = address;
        this.port = port;
        this.id = id;
    }

    static fromJson(json={}) {
        if(ObjectUtil.isArrEmpty(json.vnext)) return new Outbound.VmessSettings();
        return new Outbound.VmessSettings(
            json.vnext[0].address,
            json.vnext[0].port,
            json.vnext[0].users[0].id,
        );
    }

    toJson() {
        return {
            vnext: [{
                address: this.address,
                port: this.port,
                users: [{id: this.id}],
            }],
        };
    }
};
Outbound.VLESSSettings = class extends CommonClass {
    constructor(address, port, id, flow, encryption='none') {
        super();
        this.address = address;
        this.port = port;
        this.id = id;
        this.flow = flow;
        this.encryption = encryption
    }

    static fromJson(json={}) {
        if(ObjectUtil.isArrEmpty(json.vnext)) return new Outbound.VLESSSettings();
        return new Outbound.VLESSSettings(
            json.vnext[0].address,
            json.vnext[0].port,
            json.vnext[0].users[0].id,
            json.vnext[0].users[0].flow,
            json.vnext[0].users[0].encryption,
        );
    }

    toJson() {
        return {
            vnext: [{
                address: this.address,
                port: this.port,
                users: [{id: this.id, flow: this.flow, encryption: 'none',}],
            }],
        };
    }
};
Outbound.TrojanSettings = class extends CommonClass {
    constructor(address, port, password) {
        super();
        this.address = address;
        this.port = port;
        this.password = password;
    }

    static fromJson(json={}) {
        if(ObjectUtil.isArrEmpty(json.servers)) return new Outbound.TrojanSettings();
        return new Outbound.TrojanSettings(
            json.servers[0].address,
            json.servers[0].port,
            json.servers[0].password,
        );
    }

    toJson() {
        return {
            servers: [{
                address: this.address,
                port: this.port,
                password: this.password,
            }],
        };
    }
};
Outbound.ShadowsocksSettings = class extends CommonClass {
    constructor(address, port, password, method, uot) {
        super();
        this.address = address;
        this.port = port;
        this.password = password;
        this.method = method;
        this.uot = uot;
    }

    static fromJson(json={}) {
        let servers = json.servers;
        if(ObjectUtil.isArrEmpty(servers)) servers=[{}];
        return new Outbound.ShadowsocksSettings(
            servers[0].address,
            servers[0].port,
            servers[0].password,
            servers[0].method,
            servers[0].uot,
        );
    }

    toJson() {
        return {
            servers: [{
                address: this.address,
                port: this.port,
                password: this.password,
                method: this.method,
                uot: this.uot,
            }],
        };
    }
};

Outbound.SocksSettings = class extends CommonClass {
    constructor(address, port, user, pass) {
        super();
        this.address = address;
        this.port = port;
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json={}) {
        servers = json.servers;
        if(ObjectUtil.isArrEmpty(servers)) servers=[{users: [{}]}];
        return new Outbound.SocksSettings(
            servers[0].address,
            servers[0].port,
            ObjectUtil.isArrEmpty(servers[0].users) ? '' : servers[0].users[0].user,
            ObjectUtil.isArrEmpty(servers[0].pass) ? '' : servers[0].users[0].pass,
        );
    }

    toJson() {
        return {
            servers: [{
                address: this.address,
                port: this.port,
                users: ObjectUtil.isEmpty(this.user) ? [] : [{user: this.user, pass: this.pass}],
            }],
        };
    }
};
Outbound.HttpSettings = class extends CommonClass {
    constructor(address, port, user, pass) {
        super();
        this.address = address;
        this.port = port;
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json={}) {
        servers = json.servers;
        if(ObjectUtil.isArrEmpty(servers)) servers=[{users: [{}]}];
        return new Outbound.HttpSettings(
            servers[0].address,
            servers[0].port,
            ObjectUtil.isArrEmpty(servers[0].users) ? '' : servers[0].users[0].user,
            ObjectUtil.isArrEmpty(servers[0].pass) ? '' : servers[0].users[0].pass,
        );
    }

    toJson() {
        return {
            servers: [{
                address: this.address,
                port: this.port,
                users: ObjectUtil.isEmpty(this.user) ? [] : [{user: this.user, pass: this.pass}],
            }],
        };
    }
};

Outbound.WireguardSettings = class extends CommonClass {
    constructor(
            mtu=1420, secretKey=Wireguard.generateKeypair().privateKey,
            address=[''], workers=2, domainStrategy='ForceIPv6v4', reserved='',
            peers=[new Outbound.WireguardSettings.Peer()], kernelMode=false) {
        super();
        this.mtu = mtu;
        this.secretKey = secretKey;
        this.pubKey = secretKey.length>0 ? Wireguard.generateKeypair(secretKey).publicKey : '';
        this.address = address instanceof Array ? address.join(',') : address;
        this.workers = workers;
        this.domainStrategy = domainStrategy;
        this.reserved = reserved instanceof Array ? reserved.join(',') : reserved;
        this.peers = peers;
        this.kernelMode = kernelMode;
    }

    addPeer() {
        this.peers.push(new Outbound.WireguardSettings.Peer());
    }

    delPeer(index) {
        this.peers.splice(index, 1);
    }

    static fromJson(json={}){
        return new Outbound.WireguardSettings(
            json.mtu,
            json.secretKey,
            json.address,
            json.workers,
            json.domainStrategy,
            json.reserved,
            json.peers.map(peer => Outbound.WireguardSettings.Peer.fromJson(peer)),
            json.kernelMode,
        );
    }

    toJson() {
        return {
            mtu: this.mtu?? undefined,
            secretKey: this.secretKey,
            address: this.address ? this.address.split(",") : [],
            workers: this.workers?? undefined,
            domainStrategy: WireguardDomainStrategy.includes(this.domainStrategy) ? this.domainStrategy : undefined,
            reserved: this.reserved ? this.reserved.split(",") : undefined,
            peers: Outbound.WireguardSettings.Peer.toJsonArray(this.peers),
            kernelMode: this.kernelMode,
        };
    }
};

Outbound.WireguardSettings.Peer = class extends CommonClass {
    constructor(publicKey=Wireguard.generateKeypair().publicKey, psk='', allowedIPs=['0.0.0.0/0','::/0'], endpoint='', keepAlive=0) {
        super();
        this.publicKey = publicKey;
        this.psk = psk;
        this.allowedIPs = allowedIPs;
        this.endpoint = endpoint;
        this.keepAlive = keepAlive;
    }

    static fromJson(json={}){
        return new Outbound.WireguardSettings.Peer(
            json.publicKey,
            json.preSharedKey,
            json.allowedIPs,
            json.endpoint,
            json.keepAlive
        );
    }

    toJson() {
        return {
            publicKey: this.publicKey,
            preSharedKey: this.psk.length>0 ? this.psk : undefined,
            allowedIPs: this.allowedIPs ? this.allowedIPs : undefined,
            endpoint: this.endpoint,
            keepAlive: this.keepAlive?? undefined,
        };
    }
};