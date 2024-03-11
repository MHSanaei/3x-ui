const Protocols = {
    VMESS: 'vmess',
    VLESS: 'vless',
    TROJAN: 'trojan',
    SHADOWSOCKS: 'shadowsocks',
    DOKODEMO: 'dokodemo-door',
    SOCKS: 'socks',
    HTTP: 'http',
    WIREGUARD: 'wireguard',
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

const XTLS_FLOW_CONTROL = {
    ORIGIN: "xtls-rprx-origin",
    DIRECT: "xtls-rprx-direct",
};

const TLS_FLOW_CONTROL = {
    VISION: "xtls-rprx-vision",
    VISION_UDP443: "xtls-rprx-vision-udp443",
};

const TLS_VERSION_OPTION = {
    TLS10: "1.0",
    TLS11: "1.1",
    TLS12: "1.2",
    TLS13: "1.3",
};

const TLS_CIPHER_OPTION = {
    RSA_AES_128_CBC: "TLS_RSA_WITH_AES_128_CBC_SHA",
    RSA_AES_256_CBC: "TLS_RSA_WITH_AES_256_CBC_SHA",
    RSA_AES_128_GCM: "TLS_RSA_WITH_AES_128_GCM_SHA256",
    RSA_AES_256_GCM: "TLS_RSA_WITH_AES_256_GCM_SHA384",
    AES_128_GCM: "TLS_AES_128_GCM_SHA256",
    AES_256_GCM: "TLS_AES_256_GCM_SHA384",
    CHACHA20_POLY1305: "TLS_CHACHA20_POLY1305_SHA256",
    ECDHE_ECDSA_AES_128_CBC: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
    ECDHE_ECDSA_AES_256_CBC: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
    ECDHE_RSA_AES_128_CBC: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
    ECDHE_RSA_AES_256_CBC: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
    ECDHE_ECDSA_AES_128_GCM: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
    ECDHE_ECDSA_AES_256_GCM: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
    ECDHE_RSA_AES_128_GCM: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
    ECDHE_RSA_AES_256_GCM: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
    ECDHE_ECDSA_CHACHA20_POLY1305: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
    ECDHE_RSA_CHACHA20_POLY1305: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
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

const SNIFFING_OPTION = {
    HTTP:    "http",
    TLS:     "tls",
    QUIC:    "quic",
    FAKEDNS: "fakedns"
};

Object.freeze(Protocols);
Object.freeze(SSMethods);
Object.freeze(XTLS_FLOW_CONTROL);
Object.freeze(TLS_FLOW_CONTROL);
Object.freeze(TLS_VERSION_OPTION);
Object.freeze(TLS_CIPHER_OPTION);
Object.freeze(ALPN_OPTION);
Object.freeze(SNIFFING_OPTION);

class XrayCommonClass {

    static toJsonArray(arr) {
        return arr.map(obj => obj.toJson());
    }

    static fromJson() {
        return new XrayCommonClass();
    }

    toJson() {
        return this;
    }

    toString(format=true) {
        return format ? JSON.stringify(this.toJson(), null, 2) : JSON.stringify(this.toJson());
    }

    static toHeaders(v2Headers) {
        let newHeaders = [];
        if (v2Headers) {
            Object.keys(v2Headers).forEach(key => {
                let values = v2Headers[key];
                if (typeof(values) === 'string') {
                    newHeaders.push({ name: key, value: values });
                } else {
                    for (let i = 0; i < values.length; ++i) {
                        newHeaders.push({ name: key, value: values[i] });
                    }
                }
            });
        }
        return newHeaders;
    }

    static toV2Headers(headers, arr=true) {
        let v2Headers = {};
        for (let i = 0; i < headers.length; ++i) {
            let name = headers[i].name;
            let value = headers[i].value;
            if (ObjectUtil.isEmpty(name) || ObjectUtil.isEmpty(value)) {
                continue;
            }
            if (!(name in v2Headers)) {
                v2Headers[name] = arr ? [value] : value;
            } else {
                if (arr) {
                    v2Headers[name].push(value);
                } else {
                    v2Headers[name] = value;
                }
            }
        }
        return v2Headers;
    }
}

class TcpStreamSettings extends XrayCommonClass {
    constructor(acceptProxyProtocol=false,
                type='none',
                request=new TcpStreamSettings.TcpRequest(),
                response=new TcpStreamSettings.TcpResponse(),
                ) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.type = type;
        this.request = request;
        this.response = response;
    }

    static fromJson(json={}) {
        let header = json.header;
        if (!header) {
            header = {};
        }
        return new TcpStreamSettings(json.acceptProxyProtocol,
            header.type,
            TcpStreamSettings.TcpRequest.fromJson(header.request),
            TcpStreamSettings.TcpResponse.fromJson(header.response),
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            header: {
                type: this.type,
                request: this.type === 'http' ? this.request.toJson() : undefined,
                response: this.type === 'http' ? this.response.toJson() : undefined,
            },
        };
    }
}

TcpStreamSettings.TcpRequest = class extends XrayCommonClass {
    constructor(version='1.1',
                method='GET',
                path=['/'],
                headers=[],
    ) {
        super();
        this.version = version;
        this.method = method;
        this.path = path.length === 0 ? ['/'] : path;
        this.headers = headers;
    }

    addPath(path) {
        this.path.push(path);
    }

    removePath(index) {
        this.path.splice(index, 1);
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    getHeader(name) {
        for (const header of this.headers) {
            if (header.name.toLowerCase() === name.toLowerCase()) {
                return header.value;
            }
        }
        return null;
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json={}) {
        return new TcpStreamSettings.TcpRequest(
            json.version,
            json.method,
            json.path,
            XrayCommonClass.toHeaders(json.headers),
        );
    }

    toJson() {
        return {
            version: this.version,
            method: this.method,
            path: ObjectUtil.clone(this.path),
            headers: XrayCommonClass.toV2Headers(this.headers),
        };
    }
};

TcpStreamSettings.TcpResponse = class extends XrayCommonClass {
    constructor(version='1.1',
                status='200',
                reason='OK',
                headers=[],
    ) {
        super();
        this.version = version;
        this.status = status;
        this.reason = reason;
        this.headers = headers;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json={}) {
        return new TcpStreamSettings.TcpResponse(
            json.version,
            json.status,
            json.reason,
            XrayCommonClass.toHeaders(json.headers),
        );
    }

    toJson() {
        return {
            version: this.version,
            status: this.status,
            reason: this.reason,
            headers: XrayCommonClass.toV2Headers(this.headers),
        };
    }
};

class KcpStreamSettings extends XrayCommonClass {
    constructor(mtu=1350, tti=20,
                uplinkCapacity=5,
                downlinkCapacity=20,
                congestion=false,
                readBufferSize=2,
                writeBufferSize=2,
                type='none',
                seed=RandomUtil.randomSeq(10),
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

class WsStreamSettings extends XrayCommonClass {
    constructor(acceptProxyProtocol=false, path='/', headers=[]) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.path = path;
        this.headers = headers;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    getHeader(name) {
        for (const header of this.headers) {
            if (header.name.toLowerCase() === name.toLowerCase()) {
                return header.value;
            }
        }
        return null;
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json={}) {
        return new WsStreamSettings(
            json.acceptProxyProtocol,
            json.path,
            XrayCommonClass.toHeaders(json.headers),
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            path: this.path,
            headers: XrayCommonClass.toV2Headers(this.headers, false),
        };
    }
}

class HttpStreamSettings extends XrayCommonClass {
    constructor(
        path='/',
        host=[''],
        ) {
        super();
        this.path = path;
        this.host = host.length === 0 ? [''] : host;
    }

    addHost(host) {
        this.host.push(host);
    }

    removeHost(index) {
        this.host.splice(index, 1);
    }

    static fromJson(json={}) {
        return new HttpStreamSettings(json.path, json.host);
    }

    toJson() {
        let host = [];
        for (let i = 0; i < this.host.length; ++i) {
            if (!ObjectUtil.isEmpty(this.host[i])) {
                host.push(this.host[i]);
            }
        }
        return {
            path: this.path,
            host: host,
        }
    }
}

class QuicStreamSettings extends XrayCommonClass {
    constructor(security='none',
                key=RandomUtil.randomSeq(10), type='none') {
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

class GrpcStreamSettings extends XrayCommonClass {
    constructor(
        serviceName="",
        authority="",
        multiMode=false,
        ) {
        super();
        this.serviceName = serviceName;
        this.authority = authority;
        this.multiMode = multiMode;
    }

    static fromJson(json={}) {
        return new GrpcStreamSettings(
            json.serviceName,
            json.authority,
            json.multiMode
            );
    }

    toJson() {
        return {
            serviceName: this.serviceName,
            authority: this.authority,
            multiMode: this.multiMode,
        }
    }
}

class HttpUpgradeStreamSettings extends XrayCommonClass {
    constructor(acceptProxyProtocol=false, path='/', host='') {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.path = path;
        this.host = host;
    }

    static fromJson(json={}) {
        return new HttpUpgradeStreamSettings(
            json.acceptProxyProtocol,
            json.path,
            json.host,
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            path: this.path,
            host: this.host,
        };
    }
}

class TlsStreamSettings extends XrayCommonClass {
    constructor(serverName='',
                minVersion = TLS_VERSION_OPTION.TLS12,
                maxVersion = TLS_VERSION_OPTION.TLS13,
                cipherSuites = '',
                rejectUnknownSni = false,
                certificates=[new TlsStreamSettings.Cert()],
                alpn=[ALPN_OPTION.H2,ALPN_OPTION.HTTP1],
                settings=new TlsStreamSettings.Settings()) {
        super();
        this.sni = serverName;
        this.minVersion = minVersion;
        this.maxVersion = maxVersion;
        this.cipherSuites = cipherSuites;
        this.rejectUnknownSni = rejectUnknownSni;
        this.certs = certificates;
        this.alpn = alpn;
        this.settings = settings;
    }

    addCert() {
        this.certs.push(new TlsStreamSettings.Cert());
    }

    removeCert(index) {
        this.certs.splice(index, 1);
    }

    static fromJson(json={}) {
        let certs;
        let settings;
        if (!ObjectUtil.isEmpty(json.certificates)) {
            certs = json.certificates.map(cert => TlsStreamSettings.Cert.fromJson(cert));
        }

		if (!ObjectUtil.isEmpty(json.settings)) {
            settings = new TlsStreamSettings.Settings(json.settings.allowInsecure , json.settings.fingerprint, json.settings.serverName, json.settings.domains);
        }
        return new TlsStreamSettings(
            json.serverName,
            json.minVersion,
            json.maxVersion,
            json.cipherSuites,
            json.rejectUnknownSni,
            certs,
            json.alpn,
            settings,
        );
    }

    toJson() {
        return {
            serverName: this.sni,
            minVersion: this.minVersion,
            maxVersion: this.maxVersion,
            cipherSuites: this.cipherSuites,
            rejectUnknownSni: this.rejectUnknownSni,
            certificates: TlsStreamSettings.toJsonArray(this.certs),
            alpn: this.alpn,
            settings: this.settings,
        };
    }
}

TlsStreamSettings.Cert = class extends XrayCommonClass {
    constructor(useFile=true, certificateFile='', keyFile='', certificate='', key='', ocspStapling=3600) {
        super();
        this.useFile = useFile;
        this.certFile = certificateFile;
        this.keyFile = keyFile;
        this.cert = certificate instanceof Array ? certificate.join('\n') : certificate;
        this.key = key instanceof Array ? key.join('\n') : key;
        this.ocspStapling = ocspStapling;
    }

    static fromJson(json={}) {
        if ('certificateFile' in json && 'keyFile' in json) {
            return new TlsStreamSettings.Cert(
                true,
                json.certificateFile,
                json.keyFile, '', '',
                json.ocspStapling,
            );
        } else {
            return new TlsStreamSettings.Cert(
                false, '', '',
                json.certificate.join('\n'),
                json.key.join('\n'),
                json.ocspStapling,
            );
        }
    }

    toJson() {
        if (this.useFile) {
            return {
                certificateFile: this.certFile,
                keyFile: this.keyFile,
                ocspStapling: this.ocspStapling,
            };
        } else {
            return {
                certificate: this.cert.split('\n'),
                key: this.key.split('\n'),
                ocspStapling: this.ocspStapling,
            };
        }
    }
};

TlsStreamSettings.Settings = class extends XrayCommonClass {
    constructor(allowInsecure = false, fingerprint = '') {
        super();
        this.allowInsecure = allowInsecure;
        this.fingerprint = fingerprint;
    }
    static fromJson(json = {}) {
        return new TlsStreamSettings.Settings(
            json.allowInsecure,
            json.fingerprint,
        );
    }
    toJson() {
        return {
            allowInsecure: this.allowInsecure,
            fingerprint: this.fingerprint,
        };
    }
};
class XtlsStreamSettings extends XrayCommonClass {
    constructor(serverName='',
                certificates=[new XtlsStreamSettings.Cert()],
                alpn=[ALPN_OPTION.H2,ALPN_OPTION.HTTP1],
                settings=new XtlsStreamSettings.Settings()) {
        super();
        this.sni = serverName;
        this.certs = certificates;
        this.alpn = alpn;
        this.settings = settings;
    }

    addCert() {
        this.certs.push(new XtlsStreamSettings.Cert());
    }

    removeCert(index) {
        this.certs.splice(index, 1);
    }

    static fromJson(json={}) {
        let certs;
        let settings;
        if (!ObjectUtil.isEmpty(json.certificates)) {
            certs = json.certificates.map(cert => XtlsStreamSettings.Cert.fromJson(cert));
        }

		if (!ObjectUtil.isEmpty(json.settings)) {
            settings = new XtlsStreamSettings.Settings(json.settings.allowInsecure , json.settings.serverName);
        }
        return new XtlsStreamSettings(
            json.serverName,
            certs,
            json.alpn,
            settings,
        );
    }

    toJson() {
        return {
            serverName: this.sni,
            certificates: XtlsStreamSettings.toJsonArray(this.certs),
            alpn: this.alpn,
            settings: this.settings,
        };
    }
}

XtlsStreamSettings.Cert = class extends XrayCommonClass {
    constructor(useFile=true, certificateFile='', keyFile='', certificate='', key='') {
        super();
        this.useFile = useFile;
        this.certFile = certificateFile;
        this.keyFile = keyFile;
        this.cert = certificate instanceof Array ? certificate.join('\n') : certificate;
        this.key = key instanceof Array ? key.join('\n') : key;
    }

    static fromJson(json={}) {
        if ('certificateFile' in json && 'keyFile' in json) {
            return new XtlsStreamSettings.Cert(
                true,
                json.certificateFile,
                json.keyFile,
            );
        } else {
            return new XtlsStreamSettings.Cert(
                false, '', '',
                json.certificate.join('\n'),
                json.key.join('\n'),
            );
        }
    }

    toJson() {
        if (this.useFile) {
            return {
                certificateFile: this.certFile,
                keyFile: this.keyFile,
            };
        } else {
            return {
                certificate: this.cert.split('\n'),
                key: this.key.split('\n'),
            };
        }
    }
};

XtlsStreamSettings.Settings = class extends XrayCommonClass {
    constructor(allowInsecure = false) {
        super();
        this.allowInsecure = allowInsecure;
    }
    static fromJson(json = {}) {
        return new XtlsStreamSettings.Settings(
            json.allowInsecure,
        );
    }
    toJson() {
        return {
            allowInsecure: this.allowInsecure,
        };
    }
};

class RealityStreamSettings extends XrayCommonClass {
    
    constructor(
        show = false,xver = 0,
        dest = 'yahoo.com:443',
        serverNames = 'yahoo.com,www.yahoo.com',
        privateKey = '',
        minClient = '',
        maxClient = '',
        maxTimediff = 0,
        shortIds = RandomUtil.randomShortId(),
        settings= new RealityStreamSettings.Settings()
        ){
        super();
        this.show = show;
        this.xver = xver;
        this.dest = dest;
        this.serverNames = serverNames instanceof Array ? serverNames.join(",") : serverNames;
        this.privateKey = privateKey;
        this.minClient = minClient;
        this.maxClient = maxClient;
        this.maxTimediff = maxTimediff;
        this.shortIds = shortIds instanceof Array ? shortIds.join(",") : shortIds; 
        this.settings = settings;
    }

    static fromJson(json = {}) {
        let settings;
		if (!ObjectUtil.isEmpty(json.settings)) {
            settings = new RealityStreamSettings.Settings(json.settings.publicKey , json.settings.fingerprint, json.settings.serverName, json.settings.spiderX);
        }
        return new RealityStreamSettings(
            json.show,
            json.xver,
            json.dest,
            json.serverNames,
            json.privateKey,
            json.minClient,
            json.maxClient,
            json.maxTimediff,
            json.shortIds,
            json.settings,
        );

    }
    toJson() {
        return {
            show: this.show,
            xver: this.xver,
            dest: this.dest,
            serverNames: this.serverNames.split(","),
            privateKey: this.privateKey,
            minClient: this.minClient,
            maxClient: this.maxClient,
            maxTimediff: this.maxTimediff,
            shortIds: this.shortIds.split(","),
            settings: this.settings,
        };
    }
}

RealityStreamSettings.Settings = class extends XrayCommonClass {
    constructor(publicKey = '', fingerprint = UTLS_FINGERPRINT.UTLS_FIREFOX, serverName = '', spiderX= '/') {
        super();
        this.publicKey = publicKey;
        this.fingerprint = fingerprint;
        this.serverName = serverName;
        this.spiderX = spiderX;
    }
    static fromJson(json = {}) {
        return new RealityStreamSettings.Settings(
            json.publicKey,
            json.fingerprint,
            json.serverName,
            json.spiderX,
        );
    }
    toJson() {
        return {
            publicKey: this.publicKey,
            fingerprint: this.fingerprint,
            serverName: this.serverName,
            spiderX: this.spiderX,
        };
    }
};

class SockoptStreamSettings extends XrayCommonClass {
    constructor(acceptProxyProtocol = false, tcpFastOpen = false, mark = 0, tproxy="off") {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.tcpFastOpen = tcpFastOpen;
        this.mark = mark;
        this.tproxy = tproxy;
    }
    
    static fromJson(json = {}) {
        if (Object.keys(json).length === 0) return undefined;
        return new SockoptStreamSettings(
            json.acceptProxyProtocol,
            json.tcpFastOpen,
            json.mark,
            json.tproxy,
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            tcpFastOpen: this.tcpFastOpen,
            mark: this.mark,
            tproxy: this.tproxy,
        };
    }
}

class StreamSettings extends XrayCommonClass {
    constructor(network='tcp',
        security='none',
        externalProxy = [],
        tlsSettings=new TlsStreamSettings(),
        xtlsSettings=new XtlsStreamSettings(),
        realitySettings = new RealityStreamSettings(),
        tcpSettings=new TcpStreamSettings(),
        kcpSettings=new KcpStreamSettings(),
        wsSettings=new WsStreamSettings(),
        httpSettings=new HttpStreamSettings(),
        quicSettings=new QuicStreamSettings(),
        grpcSettings=new GrpcStreamSettings(),
        httpupgradeSettings=new HttpUpgradeStreamSettings(),
        sockopt = undefined,
        ) {
        super();
        this.network = network;
        this.security = security;
        this.externalProxy = externalProxy;
        this.tls = tlsSettings;
        this.xtls = xtlsSettings;
        this.reality = realitySettings;
        this.tcp = tcpSettings;
        this.kcp = kcpSettings;
        this.ws = wsSettings;
        this.http = httpSettings;
        this.quic = quicSettings;
        this.grpc = grpcSettings;
        this.httpupgrade = httpupgradeSettings;
        this.sockopt = sockopt;
    }

    get isTls() {
        return this.security === "tls";
    }

    set isTls(isTls) {
        if (isTls) {
            this.security = 'tls';
        } else {
            this.security = 'none';
        }
    }

    get isXtls() {
        return this.security === "xtls";
    }

    set isXtls(isXtls) {
        if (isXtls) {
            this.security = 'xtls';
        } else {
            this.security = 'none';
        }
    }

    //for Reality
    get isReality() {
        return this.security === "reality";
    }

    set isReality(isReality) {
        if (isReality) {
            this.security = 'reality';
        } else {
            this.security = 'none';
        }
    }

    get sockoptSwitch() {
        return this.sockopt != undefined;
    }

    set sockoptSwitch(value) {
        this.sockopt = value ? new SockoptStreamSettings() : undefined;
    }

    static fromJson(json={}) {
        return new StreamSettings(
            json.network,
            json.security,
            json.externalProxy,
            TlsStreamSettings.fromJson(json.tlsSettings),
            XtlsStreamSettings.fromJson(json.xtlsSettings),
            RealityStreamSettings.fromJson(json.realitySettings),
            TcpStreamSettings.fromJson(json.tcpSettings),
            KcpStreamSettings.fromJson(json.kcpSettings),
            WsStreamSettings.fromJson(json.wsSettings),
            HttpStreamSettings.fromJson(json.httpSettings),
            QuicStreamSettings.fromJson(json.quicSettings),
            GrpcStreamSettings.fromJson(json.grpcSettings),
            HttpUpgradeStreamSettings.fromJson(json.httpupgradeSettings),
            SockoptStreamSettings.fromJson(json.sockopt),
        );
    }

    toJson() {
        const network = this.network;
        return {
            network: network,
            security: this.security,
            externalProxy: this.externalProxy,
            tlsSettings: this.isTls ? this.tls.toJson() : undefined,
            xtlsSettings: this.isXtls ? this.xtls.toJson() : undefined,
            realitySettings: this.isReality ? this.reality.toJson() : undefined,
            tcpSettings: network === 'tcp' ? this.tcp.toJson() : undefined,
            kcpSettings: network === 'kcp' ? this.kcp.toJson() : undefined,
            wsSettings: network === 'ws' ? this.ws.toJson() : undefined,
            httpSettings: network === 'http' ? this.http.toJson() : undefined,
            quicSettings: network === 'quic' ? this.quic.toJson() : undefined,
            grpcSettings: network === 'grpc' ? this.grpc.toJson() : undefined,
            httpupgradeSettings: network === 'httpupgrade' ? this.httpupgrade.toJson() : undefined,
            sockopt: this.sockopt != undefined ? this.sockopt.toJson() : undefined,
        };
    }
}

class Sniffing extends XrayCommonClass {
    constructor(enabled=true, destOverride=['http', 'tls', 'quic', 'fakedns']) {
        super();
        this.enabled = enabled;
        this.destOverride = destOverride;
    }

    static fromJson(json={}) {
        let destOverride = ObjectUtil.clone(json.destOverride);
        if (!ObjectUtil.isEmpty(destOverride) && !ObjectUtil.isArrEmpty(destOverride)) {
            if (ObjectUtil.isEmpty(destOverride[0])) {
                destOverride = ['http', 'tls', 'quic', 'fakedns'];
            }
        }
        return new Sniffing(
            !!json.enabled,
            destOverride,
        );
    }
}

class Inbound extends XrayCommonClass {
    constructor(port=RandomUtil.randomIntRange(10000, 60000),
                listen='',
                protocol=Protocols.VLESS,
                settings=null,
                streamSettings=new StreamSettings(),
                tag='',
                sniffing=new Sniffing(),
                clientStats='',
                ) {
        super();
        this.port = port;
        this.listen = listen;
        this._protocol = protocol;
        this.settings = ObjectUtil.isEmpty(settings) ? Inbound.Settings.getSettings(protocol) : settings;
        this.stream = streamSettings;
        this.tag = tag;
        this.sniffing = sniffing;
        this.clientStats = clientStats;
    }
    getClientStats() {
        return this.clientStats;
    }

    get clients() {
        switch (this.protocol) {
            case Protocols.VMESS: return this.settings.vmesses;
            case Protocols.VLESS: return this.settings.vlesses;
            case Protocols.TROJAN: return this.settings.trojans;
            case Protocols.SHADOWSOCKS: return this.isSSMultiUser ? this.settings.shadowsockses : null;
            default: return null;
        }
    }

    get protocol() {
        return this._protocol;
    }

    set protocol(protocol) {
        this._protocol = protocol;
        this.settings = Inbound.Settings.getSettings(protocol);
        if (protocol === Protocols.TROJAN) {
            this.tls = false;
        }
    }

    get xtls() {
        return this.stream.security === 'xtls';
    }

    set xtls(isXtls) {
        if (isXtls) {
            this.stream.security = 'xtls';
        } else {
            this.stream.security = 'none';
        }
    }
    
    get network() {
        return this.stream.network;
    }

    set network(network) {
        this.stream.network = network;
    }

    get isTcp() {
        return this.network === "tcp";
    }

    get isWs() {
        return this.network === "ws";
    }

    get isKcp() {
        return this.network === "kcp";
    }

    get isQuic() {
        return this.network === "quic"
    }

    get isGrpc() {
        return this.network === "grpc";
    }

    get isH2() {
        return this.network === "http";
    }

    get isHttpupgrade() {
        return this.network === "httpupgrade";
    }

    // Shadowsocks
    get method() {
        switch (this.protocol) {
            case Protocols.SHADOWSOCKS:
                return this.settings.method;
            default:
                return "";
        }
    }
    get isSSMultiUser() {
        return this.method != SSMethods.BLAKE3_CHACHA20_POLY1305;
    }
    get isSS2022(){
        return this.method.substring(0,4) === "2022";
    }

    get serverName() {
        if (this.stream.isTls) return this.stream.tls.sni;
        if (this.stream.isXtls) return this.stream.xtls.sni;
        if (this.stream.isReality) return this.stream.reality.serverNames;
        return "";
    }

    get host() {
        if (this.isTcp) {
            return this.stream.tcp.request.getHeader("Host");
        } else if (this.isWs) {
            return this.stream.ws.getHeader("Host");
        } else if (this.isH2) {
            return this.stream.http.host[0];
        } else if (this.isHttpupgrade) {
            return this.stream.httpupgrade.host;
        }
        return null;
    }

    get path() {
        if (this.isTcp) {
            return this.stream.tcp.request.path[0];
        } else if (this.isWs) {
            return this.stream.ws.path;
        } else if (this.isH2) {
            return this.stream.http.path;
        } else if (this.isHttpupgrade) {
            return this.stream.httpupgrade.path;
        }
        return null;
    }

    get quicSecurity() {
        return this.stream.quic.security;
    }

    get quicKey() {
        return this.stream.quic.key;
    }

    get quicType() {
        return this.stream.quic.type;
    }

    get kcpType() {
        return this.stream.kcp.type;
    }

    get kcpSeed() {
        return this.stream.kcp.seed;
    }

    get serviceName() {
        return this.stream.grpc.serviceName;
    }

    isExpiry(index) {
        let exp = this.clients[index].expiryTime;
        return exp > 0 ? exp < new Date().getTime() : false;
    }

    canEnableTls() {
        if(![Protocols.VMESS, Protocols.VLESS, Protocols.TROJAN, Protocols.SHADOWSOCKS].includes(this.protocol)) return false;
        return ["tcp", "ws", "http", "quic", "grpc", "httpupgrade"].includes(this.network);
    }

    //this is used for xtls-rprx-vision
    canEnableTlsFlow() {
        if (((this.stream.security === 'tls') || (this.stream.security === 'reality')) && (this.network === "tcp")) {
            return this.protocol === Protocols.VLESS;
        }
        return false;
    }

    canEnableReality() {
        if(![Protocols.VLESS, Protocols.TROJAN].includes(this.protocol)) return false;
        return ["tcp", "http", "grpc"].includes(this.network);
    }

    canEnableXtls() {
        if(![Protocols.VLESS, Protocols.TROJAN].includes(this.protocol)) return false;
        return this.network === "tcp";
    }

    canEnableStream() {
        return [Protocols.VMESS, Protocols.VLESS, Protocols.TROJAN, Protocols.SHADOWSOCKS].includes(this.protocol);
    }

    reset() {
        this.port = RandomUtil.randomIntRange(10000, 60000);
        this.listen = '';
        this.protocol = Protocols.VMESS;
        this.settings = Inbound.Settings.getSettings(Protocols.VMESS);
        this.stream = new StreamSettings();
        this.tag = '';
        this.sniffing = new Sniffing();
    }

    genVmessLink(address='', port=this.port, forceTls, remark='', clientId) {
        if (this.protocol !== Protocols.VMESS) {
            return '';
        }
        const security = forceTls == 'same' ? this.stream.security : forceTls;
        let obj = {
            v: '2',
            ps: remark,
            add: address,
            port: port,
            id: clientId,
            net: this.stream.network,
            type: 'none',
            tls: security,
        };
        let network = this.stream.network;
        if (network === 'tcp') {
            let tcp = this.stream.tcp;
            obj.type = tcp.type;
            if (tcp.type === 'http') {
                let request = tcp.request;
                obj.path = request.path.join(',');
                let index = request.headers.findIndex(header => header.name.toLowerCase() === 'host');
                if (index >= 0) {
                    obj.host = request.headers[index].value;
                }
            }
        } else if (network === 'kcp') {
            let kcp = this.stream.kcp;
            obj.type = kcp.type;
            obj.path = kcp.seed;
        } else if (network === 'ws') {
            let ws = this.stream.ws;
            obj.path = ws.path;
            let index = ws.headers.findIndex(header => header.name.toLowerCase() === 'host');
            if (index >= 0) {
                obj.host = ws.headers[index].value;
            }
        } else if (network === 'http') {
            obj.net = 'h2';
            obj.path = this.stream.http.path;
            obj.host = this.stream.http.host.join(',');
        } else if (network === 'quic') {
            obj.type = this.stream.quic.type;
            obj.host = this.stream.quic.security;
            obj.path = this.stream.quic.key;
        } else if (network === 'grpc') {
            obj.path = this.stream.grpc.serviceName;
            obj.authority = this.stream.grpc.authority;
            if (this.stream.grpc.multiMode){
                obj.type = 'multi'
            }
        } else if (network === 'httpupgrade') {
            let httpupgrade = this.stream.httpupgrade;
            obj.path = httpupgrade.path;
            obj.host = httpupgrade.host;
        }

        if (security === 'tls') {
            if (!ObjectUtil.isEmpty(this.stream.tls.sni)){
                obj.sni = this.stream.tls.sni;
            }
            if (!ObjectUtil.isEmpty(this.stream.tls.settings.fingerprint)){
                obj.fp = this.stream.tls.settings.fingerprint;
            }
            if (this.stream.tls.alpn.length>0){
                obj.alpn = this.stream.tls.alpn.join(',');
            }
            if (this.stream.tls.settings.allowInsecure){
                obj.allowInsecure = this.stream.tls.settings.allowInsecure;
            }
        }
        
        return 'vmess://' + base64(JSON.stringify(obj, null, 2));
    }

    genVLESSLink(address = '', port=this.port, forceTls, remark='', clientId, flow) {
        const uuid = clientId;
        const type = this.stream.network;
        const security = forceTls == 'same' ? this.stream.security : forceTls;
        const params = new Map();
        params.set("type", this.stream.network);
        switch (type) {
            case "tcp":
                const tcp = this.stream.tcp;
                if (tcp.type === 'http') {
                    const request = tcp.request;
                    params.set("path", request.path.join(','));
                    const index = request.headers.findIndex(header => header.name.toLowerCase() === 'host');
                    if (index >= 0) {
                        const host = request.headers[index].value;
                        params.set("host", host);
                    }
                    params.set("headerType", 'http');
                }
                break;
            case "kcp":
                const kcp = this.stream.kcp;
                params.set("headerType", kcp.type);
                params.set("seed", kcp.seed);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                const index = ws.headers.findIndex(header => header.name.toLowerCase() === 'host');
                if (index >= 0) {
                    const host = ws.headers[index].value;
                    params.set("host", host);
                }
                break;
            case "http":
                const http = this.stream.http;
                params.set("path", http.path);
                params.set("host", http.host);
                break;
            case "quic":
                const quic = this.stream.quic;
                params.set("quicSecurity", quic.security);
                params.set("key", quic.key);
                params.set("headerType", quic.type);
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if(grpc.multiMode){
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                    const httpupgrade = this.stream.httpupgrade;
                    params.set("path", httpupgrade.path);
                    params.set("host", httpupgrade.host);
                break;
        }

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls){
                params.set("fp" , this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if(this.stream.tls.settings.allowInsecure){
                    params.set("allowInsecure", "1");
                }
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)){
                    params.set("sni", this.stream.tls.sni);
                }
                if (type == "tcp" && !ObjectUtil.isEmpty(flow)) {
                    params.set("flow", flow);
                }
            }
        }

        else if (security === 'xtls') {
            params.set("security", "xtls");
            params.set("alpn", this.stream.xtls.alpn);
            if(this.stream.xtls.settings.allowInsecure){
                params.set("allowInsecure", "1");
            }
            if (!ObjectUtil.isEmpty(this.stream.xtls.sni)){
                params.set("sni", this.stream.xtls.sni);
			}
            params.set("flow", flow);
        }

        else if (security === 'reality') {
            params.set("security", "reality");
            params.set("pbk", this.stream.reality.settings.publicKey);
            params.set("fp", this.stream.reality.settings.fingerprint);
            if (!ObjectUtil.isArrEmpty(this.stream.reality.serverNames)) {
                params.set("sni", this.stream.reality.serverNames.split(",")[0]);
            }
            if (this.stream.reality.shortIds.length > 0) {
                params.set("sid", this.stream.reality.shortIds.split(",")[0]);
            }
            if (!ObjectUtil.isEmpty(this.stream.reality.settings.spiderX)) {
                params.set("spx", this.stream.reality.settings.spiderX);
            }
            if (type == 'tcp' && !ObjectUtil.isEmpty(flow)) {
                params.set("flow", flow);
            }
        }

        else {
            params.set("security", "none");
        }

        const link = `vless://${uuid}@${address}:${port}`;
        const url = new URL(link);
        for (const [key, value] of params) {
            url.searchParams.set(key, value)
        }
        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    genSSLink(address='', port=this.port, forceTls, remark='', clientPassword) {
        let settings = this.settings;
        const type = this.stream.network;
        const security = forceTls == 'same' ? this.stream.security : forceTls;
        const params = new Map();
        params.set("type", this.stream.network);
        switch (type) {
            case "tcp":
                const tcp = this.stream.tcp;
                if (tcp.type === 'http') {
                    const request = tcp.request;
                    params.set("path", request.path.join(','));
                    const index = request.headers.findIndex(header => header.name.toLowerCase() === 'host');
                    if (index >= 0) {
                        const host = request.headers[index].value;
                        params.set("host", host);
                    }
                    params.set("headerType", 'http');
                }
                break;
            case "kcp":
                const kcp = this.stream.kcp;
                params.set("headerType", kcp.type);
                params.set("seed", kcp.seed);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                const index = ws.headers.findIndex(header => header.name.toLowerCase() === 'host');
                if (index >= 0) {
                    const host = ws.headers[index].value;
                    params.set("host", host);
                }
                break;
            case "http":
                const http = this.stream.http;
                params.set("path", http.path);
                params.set("host", http.host);
                break;
            case "quic":
                const quic = this.stream.quic;
                params.set("quicSecurity", quic.security);
                params.set("key", quic.key);
                params.set("headerType", quic.type);
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if(grpc.multiMode){
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                    const httpupgrade = this.stream.httpupgrade;
                    params.set("path", httpupgrade.path);
                    params.set("host", httpupgrade.host);
                break;
        }

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls){
                params.set("fp" , this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if(this.stream.tls.settings.allowInsecure){
                    params.set("allowInsecure", "1");
                }
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)){
                    params.set("sni", this.stream.tls.sni);
                }
            }
        }


        let password = new Array();
        if (this.isSS2022) password.push(settings.password);
        if (this.isSSMultiUser) password.push(clientPassword);

        let link = `ss://${safeBase64(settings.method + ':' + password.join(':'))}@${address}:${port}`;
        const url = new URL(link);
        for (const [key, value] of params) {
            url.searchParams.set(key, value)
        }
        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    genTrojanLink(address = '', port=this.port, forceTls, remark = '', clientPassword) {
        const security = forceTls == 'same' ? this.stream.security : forceTls;
        const type = this.stream.network;
        const params = new Map();
        params.set("type", this.stream.network);
        switch (type) {
            case "tcp":
                const tcp = this.stream.tcp;
                if (tcp.type === 'http') {
                    const request = tcp.request;
                    params.set("path", request.path.join(','));
                    const index = request.headers.findIndex(header => header.name.toLowerCase() === 'host');
                    if (index >= 0) {
                        const host = request.headers[index].value;
                        params.set("host", host);
                    }
                    params.set("headerType", 'http');
                }
                break;
            case "kcp":
                const kcp = this.stream.kcp;
                params.set("headerType", kcp.type);
                params.set("seed", kcp.seed);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                const index = ws.headers.findIndex(header => header.name.toLowerCase() === 'host');
                if (index >= 0) {
                    const host = ws.headers[index].value;
                    params.set("host", host);
                }
                break;
            case "http":
                const http = this.stream.http;
                params.set("path", http.path);
                params.set("host", http.host);
                break;
            case "quic":
                const quic = this.stream.quic;
                params.set("quicSecurity", quic.security);
                params.set("key", quic.key);
                params.set("headerType", quic.type);
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if(grpc.multiMode){
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                    const httpupgrade = this.stream.httpupgrade;
                    params.set("path", httpupgrade.path);
                    params.set("host", httpupgrade.host);
                break;
        }

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls){
                params.set("fp" , this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if(this.stream.tls.settings.allowInsecure){
                    params.set("allowInsecure", "1");
                }
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)){
                    params.set("sni", this.stream.tls.sni);
                }
            }
        }

        else if (security === 'reality') {
            params.set("security", "reality");
            params.set("pbk", this.stream.reality.settings.publicKey);
            params.set("fp", this.stream.reality.settings.fingerprint);
            if (!ObjectUtil.isArrEmpty(this.stream.reality.serverNames)) {
                params.set("sni", this.stream.reality.serverNames.split(",")[0]);
            }
            if (this.stream.reality.shortIds.length > 0) {
                params.set("sid", this.stream.reality.shortIds.split(",")[0]);
            }
            if (!ObjectUtil.isEmpty(this.stream.reality.settings.spiderX)) {
                params.set("spx", this.stream.reality.settings.spiderX);
            }
        }

		else if (security === 'xtls') {
            params.set("security", "xtls");
            params.set("alpn", this.stream.xtls.alpn);
            if(this.stream.xtls.settings.allowInsecure){
                params.set("allowInsecure", "1");
            }
            if (!ObjectUtil.isEmpty(this.stream.xtls.sni)){
                params.set("sni", this.stream.xtls.sni);
			}
            params.set("flow", flow);
        }

        else {
            params.set("security", "none");
        }

        const link = `trojan://${clientPassword}@${address}:${port}`;
        const url = new URL(link);
        for (const [key, value] of params) {
            url.searchParams.set(key, value)
        }
        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    getWireguardLink(address, port, remark, peerId) {
        let txt = `[Interface]\n`
        txt += `PrivateKey = ${this.settings.peers[peerId].privateKey}\n`
        txt += `Address = ${this.settings.peers[peerId].allowedIPs[0]}\n`
        txt += `DNS = 1.1.1.1, 1.0.0.1\n`
        if (this.settings.mtu) {
            txt += `MTU = ${this.settings.mtu}\n`
        }
        txt += `\n# ${remark}\n`
        txt += `[Peer]\n`
        txt += `PublicKey = ${this.settings.pubKey}\n`
        txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`
        txt += `Endpoint = ${address}:${port}`
        if (this.settings.peers[peerId].psk) {
            txt += `\nPresharedKey = ${this.settings.peers[peerId].psk}`
        }
        if (this.settings.peers[peerId].keepAlive) {
            txt += `\nPersistentKeepalive = ${this.settings.peers[peerId].keepAlive}\n`
        }
        return txt;
    }

    genLink(address='', port=this.port, forceTls='same', remark='', client) {
        switch (this.protocol) {
            case Protocols.VMESS:
                return this.genVmessLink(address, port, forceTls, remark, client.id);
            case Protocols.VLESS:
                return this.genVLESSLink(address, port, forceTls, remark, client.id, client.flow);
            case Protocols.SHADOWSOCKS: 
                return this.genSSLink(address, port, forceTls, remark, this.isSSMultiUser ? client.password : '');
            case Protocols.TROJAN:
                return this.genTrojanLink(address, port, forceTls, remark, client.password);
            default: return '';
        }
    }

    genAllLinks(remark='', remarkModel = '-ieo', client){
        let result = [];
        let email = client ? client.email : '';
        let addr = !ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0" ? this.listen : location.hostname;
        let port = this.port;
        const separationChar = remarkModel.charAt(0);
        const orderChars = remarkModel.slice(1);
        let orders = {
            'i': remark,
            'e': email,
            'o': '',
          };
        if(ObjectUtil.isArrEmpty(this.stream.externalProxy)){
            let r = orderChars.split('').map(char => orders[char]).filter(x => x.length > 0).join(separationChar);
            result.push({
                remark: r,
                link: this.genLink(addr, port, 'same', r, client)
            });
        } else {
            this.stream.externalProxy.forEach((ep) => {
                orders['o'] = ep.remark;
                let r = orderChars.split('').map(char => orders[char]).filter(x => x.length > 0).join(separationChar);
                result.push({
                    remark: r,
                    link: this.genLink(ep.dest, ep.port, ep.forceTls, r, client)
                });
            });
        }
        return result;
    }

    genInboundLinks(remark = '', remarkModel = '-ieo') {
        let addr = !ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0" ? this.listen : location.hostname;
        if(this.clients){
           let links = [];
           this.clients.forEach((client) => {
                this.genAllLinks(remark,remarkModel,client).forEach(l => {
                    links.push(l.link);
                })
            });
            return links.join('\r\n');
        } else {
            if(this.protocol == Protocols.SHADOWSOCKS && !this.isSSMultiUser) return this.genSSLink(addr, this.port, 'same', remark);
            if(this.protocol == Protocols.WIREGUARD) {
                let links = [];
                this.settings.peers.forEach((p,index) => {
                    links.push(this.getWireguardLink(addr,this.port,remark + remarkModel.charAt(0) + (index+1),index));
                });
                return links.join('\r\n');
            }
            return '';
        }
    }

    static fromJson(json={}) {
        return new Inbound(
            json.port,
            json.listen,
            json.protocol,
            Inbound.Settings.fromJson(json.protocol, json.settings),
            StreamSettings.fromJson(json.streamSettings),
            json.tag,
            Sniffing.fromJson(json.sniffing),
            json.clientStats
        )
    }

    toJson() {
        let streamSettings;
        if (this.canEnableStream()) {
            streamSettings = this.stream.toJson();
        }
        return {
            port: this.port,
            listen: this.listen,
            protocol: this.protocol,
            settings: this.settings instanceof XrayCommonClass ? this.settings.toJson() : this.settings,
            streamSettings: streamSettings,
            tag: this.tag,
            sniffing: this.sniffing.toJson(),
            clientStats: this.clientStats
        };
    }
}

Inbound.Settings = class extends XrayCommonClass {
    constructor(protocol) {
        super();
        this.protocol = protocol;
    }

    static getSettings(protocol) {
        switch (protocol) {
            case Protocols.VMESS: return new Inbound.VmessSettings(protocol);
            case Protocols.VLESS: return new Inbound.VLESSSettings(protocol);
            case Protocols.TROJAN: return new Inbound.TrojanSettings(protocol);
            case Protocols.SHADOWSOCKS: return new Inbound.ShadowsocksSettings(protocol);
            case Protocols.DOKODEMO: return new Inbound.DokodemoSettings(protocol);
            case Protocols.SOCKS: return new Inbound.SocksSettings(protocol);
            case Protocols.HTTP: return new Inbound.HttpSettings(protocol);            
            case Protocols.WIREGUARD: return new Inbound.WireguardSettings(protocol);
            default: return null;
        }
    }

    static fromJson(protocol, json) {
        switch (protocol) {
            case Protocols.VMESS: return Inbound.VmessSettings.fromJson(json);
            case Protocols.VLESS: return Inbound.VLESSSettings.fromJson(json);
            case Protocols.TROJAN: return Inbound.TrojanSettings.fromJson(json);
            case Protocols.SHADOWSOCKS: return Inbound.ShadowsocksSettings.fromJson(json);
            case Protocols.DOKODEMO: return Inbound.DokodemoSettings.fromJson(json);
            case Protocols.SOCKS: return Inbound.SocksSettings.fromJson(json);
            case Protocols.HTTP: return Inbound.HttpSettings.fromJson(json);
            case Protocols.WIREGUARD: return Inbound.WireguardSettings.fromJson(json);
            default: return null;
        }
    }

    toJson() {
        return {};
    }
};

Inbound.VmessSettings = class extends Inbound.Settings {
    constructor(protocol,
        vmesses=[new Inbound.VmessSettings.Vmess()]) {
        super(protocol);
        this.vmesses = vmesses;
    }

    indexOfVmessById(id) {
        return this.vmesses.findIndex(vmess => vmess.id === id);
    }

    addVmess(vmess) {
        if (this.indexOfVmessById(vmess.id) >= 0) {
            return false;
        }
        this.vmesses.push(vmess);
    }

    delVmess(vmess) {
        const i = this.indexOfVmessById(vmess.id);
        if (i >= 0) {
            this.vmesses.splice(i, 1);
        }
    }

    static fromJson(json={}) {
        return new Inbound.VmessSettings(
            Protocols.VMESS,
            json.clients.map(client => Inbound.VmessSettings.Vmess.fromJson(client)),
        );
    }

    toJson() {
        return {
            clients: Inbound.VmessSettings.toJsonArray(this.vmesses),
        };
    }
};
Inbound.VmessSettings.Vmess = class extends XrayCommonClass {
    constructor(id=RandomUtil.randomUUID(), email=RandomUtil.randomLowerAndNum(8),limitIp=0, totalGB=0, expiryTime=0, enable=true, tgId='', subId=RandomUtil.randomLowerAndNum(16), reset=0) {
        super();
        this.id = id;
        this.email = email;
        this.limitIp = limitIp;
        this.totalGB = totalGB;
        this.expiryTime = expiryTime;
        this.enable = enable;
        this.tgId = tgId;
        this.subId = subId;
        this.reset = reset;
    }

    static fromJson(json={}) {
        return new Inbound.VmessSettings.Vmess(
            json.id,
            json.email,
            json.limitIp,
            json.totalGB,
            json.expiryTime,
            json.enable,
            json.tgId,
            json.subId,
            json.reset,
        );
    }
    get _expiryTime() {
        if (this.expiryTime === 0 || this.expiryTime === "") {
            return null;
        }
        if (this.expiryTime < 0){
            return this.expiryTime / -86400000;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null || t === "") {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }
    get _totalGB() {
        return toFixed(this.totalGB / ONE_GB, 2);
    }

    set _totalGB(gb) {
        this.totalGB = toFixed(gb * ONE_GB, 0);
    }

};

Inbound.VLESSSettings = class extends Inbound.Settings {
    constructor(protocol,
                vlesses=[new Inbound.VLESSSettings.VLESS()],
                decryption='none',
                fallbacks=[]) {
        super(protocol);
        this.vlesses = vlesses;
        this.decryption = decryption; 
        this.fallbacks = fallbacks;
    }

    addFallback() {
        this.fallbacks.push(new Inbound.VLESSSettings.Fallback());
    }

    delFallback(index) {
        this.fallbacks.splice(index, 1);
    }

    // decryption should be set to static value
    static fromJson(json={}) {
        return new Inbound.VLESSSettings(
            Protocols.VLESS,
            json.clients.map(client => Inbound.VLESSSettings.VLESS.fromJson(client)),
            json.decryption || 'none',
            Inbound.VLESSSettings.Fallback.fromJson(json.fallbacks),);
    }

    toJson() {
        return {
            clients: Inbound.VLESSSettings.toJsonArray(this.vlesses),
            decryption: this.decryption, 
            fallbacks: Inbound.VLESSSettings.toJsonArray(this.fallbacks),
        };
    }
};

Inbound.VLESSSettings.VLESS = class extends XrayCommonClass {
    constructor(id=RandomUtil.randomUUID(), flow='', email=RandomUtil.randomLowerAndNum(8),limitIp=0, totalGB=0, expiryTime=0, enable=true, tgId='', subId=RandomUtil.randomLowerAndNum(16), reset=0) {
        super();
        this.id = id;
        this.flow = flow;
        this.email = email;
        this.limitIp = limitIp;
        this.totalGB = totalGB;
        this.expiryTime = expiryTime;
        this.enable = enable;
        this.tgId = tgId;
        this.subId = subId;
        this.reset = reset;
    }

    static fromJson(json={}) {
        return new Inbound.VLESSSettings.VLESS(
            json.id,
            json.flow,
            json.email,
            json.limitIp,
            json.totalGB,
            json.expiryTime,
            json.enable,
            json.tgId,
            json.subId,
            json.reset,
        );
      }

    get _expiryTime() {
        if (this.expiryTime === 0 || this.expiryTime === "") {
            return null;
        }
        if (this.expiryTime < 0){
            return this.expiryTime / -86400000;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null || t === "") {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }
    get _totalGB() {
        return toFixed(this.totalGB / ONE_GB, 2);
    }

    set _totalGB(gb) {
        this.totalGB = toFixed(gb * ONE_GB, 0);
    }
};
Inbound.VLESSSettings.Fallback = class extends XrayCommonClass {
    constructor(name="", alpn='', path='', dest='', xver=0) {
        super();
        this.name = name;
        this.alpn = alpn;
        this.path = path;
        this.dest = dest;
        this.xver = xver;
    }

    toJson() {
        let xver = this.xver;
        if (!Number.isInteger(xver)) {
            xver = 0;
        }
        return {
            name: this.name,
            alpn: this.alpn,
            path: this.path,
            dest: this.dest,
            xver: xver,
        }
    }

    static fromJson(json=[]) {
        const fallbacks = [];
        for (let fallback of json) {
            fallbacks.push(new Inbound.VLESSSettings.Fallback(
                fallback.name,
                fallback.alpn,
                fallback.path,
                fallback.dest,
                fallback.xver,
            ))
        }
        return fallbacks;
    }
};

Inbound.TrojanSettings = class extends Inbound.Settings {
    constructor(protocol,
                trojans=[new Inbound.TrojanSettings.Trojan()],
                fallbacks=[],) {
        super(protocol);
        this.trojans = trojans;
        this.fallbacks = fallbacks;
    }

    addFallback() {
        this.fallbacks.push(new Inbound.TrojanSettings.Fallback());
    }

    delFallback(index) {
        this.fallbacks.splice(index, 1);
    }

    static fromJson(json={}) {
        return new Inbound.TrojanSettings(
            Protocols.TROJAN,
            json.clients.map(client => Inbound.TrojanSettings.Trojan.fromJson(client)),
            Inbound.TrojanSettings.Fallback.fromJson(json.fallbacks),);
        }

    toJson() {
        return {
            clients: Inbound.TrojanSettings.toJsonArray(this.trojans),
            fallbacks: Inbound.TrojanSettings.toJsonArray(this.fallbacks)
        };
    }
};
Inbound.TrojanSettings.Trojan = class extends XrayCommonClass {
    constructor(password=RandomUtil.randomSeq(10), flow='', email=RandomUtil.randomLowerAndNum(8),limitIp=0, totalGB=0, expiryTime=0, enable=true, tgId='', subId=RandomUtil.randomLowerAndNum(16), reset=0) {
        super();
        this.password = password;
        this.flow = flow;
        this.email = email;
        this.limitIp = limitIp;
        this.totalGB = totalGB;
        this.expiryTime = expiryTime;
        this.enable = enable;
        this.tgId = tgId;
        this.subId = subId;
        this.reset = reset;
    }

    toJson() {
        return {
            password: this.password,
            flow: this.flow,
            email: this.email,
            limitIp: this.limitIp,
            totalGB: this.totalGB,
            expiryTime: this.expiryTime,
            enable: this.enable,
            tgId: this.tgId,
            subId: this.subId,
            reset: this.reset,
        };
    }

    static fromJson(json = {}) {
        return new Inbound.TrojanSettings.Trojan(
            json.password,
            json.flow,
            json.email,
            json.limitIp,
            json.totalGB,
            json.expiryTime,
            json.enable,
            json.tgId,
            json.subId,
            json.reset,
        );
    }

    get _expiryTime() {
        if (this.expiryTime === 0 || this.expiryTime === "") {
            return null;
        }
        if (this.expiryTime < 0){
            return this.expiryTime / -86400000;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null || t === "") {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }
    get _totalGB() {
        return toFixed(this.totalGB / ONE_GB, 2);
    }

    set _totalGB(gb) {
        this.totalGB = toFixed(gb * ONE_GB, 0);
    }

};

Inbound.TrojanSettings.Fallback = class extends XrayCommonClass {
    constructor(name="", alpn='', path='', dest='', xver=0) {
        super();
        this.name = name;
        this.alpn = alpn;
        this.path = path;
        this.dest = dest;
        this.xver = xver;
    }

    toJson() {
        let xver = this.xver;
        if (!Number.isInteger(xver)) {
            xver = 0;
        }
        return {
            name: this.name,
            alpn: this.alpn,
            path: this.path,
            dest: this.dest,
            xver: xver,
        }
    }

    static fromJson(json=[]) {
        const fallbacks = [];
        for (let fallback of json) {
            fallbacks.push(new Inbound.TrojanSettings.Fallback(
                fallback.name,
                fallback.alpn,
                fallback.path,
                fallback.dest,
                fallback.xver,
            ))
        }
        return fallbacks;
    }
};

Inbound.ShadowsocksSettings = class extends Inbound.Settings {
    constructor(protocol,
                method=SSMethods.BLAKE3_AES_256_GCM,
                password=RandomUtil.randomShadowsocksPassword(),
                network='tcp,udp',
                shadowsockses=[new Inbound.ShadowsocksSettings.Shadowsocks()]
    ) {
        super(protocol);
        this.method = method;
        this.password = password;
        this.network = network;
        this.shadowsockses = shadowsockses;
    }

    static fromJson(json={}) {
        return new Inbound.ShadowsocksSettings(
            Protocols.SHADOWSOCKS,
            json.method,
            json.password,
            json.network,
            json.clients.map(client => Inbound.ShadowsocksSettings.Shadowsocks.fromJson(client)),
        );
    }

    toJson() {
        return {
            method: this.method,
            password: this.password,
            network: this.network,
            clients: Inbound.ShadowsocksSettings.toJsonArray(this.shadowsockses)
        };
    }
};

Inbound.ShadowsocksSettings.Shadowsocks = class extends XrayCommonClass {
    constructor(method='', password=RandomUtil.randomShadowsocksPassword(), email=RandomUtil.randomLowerAndNum(8),limitIp=0, totalGB=0, expiryTime=0, enable=true, tgId='', subId=RandomUtil.randomLowerAndNum(16), reset=0) {
        super();
        this.method = method;
        this.password = password;
        this.email = email;
        this.limitIp = limitIp;
        this.totalGB = totalGB;
        this.expiryTime = expiryTime;
        this.enable = enable;
        this.tgId = tgId;
        this.subId = subId;
        this.reset = reset;
    }

    toJson() {
        return {
            method: this.method,
            password: this.password,
            email: this.email,
            limitIp: this.limitIp,
            totalGB: this.totalGB,
            expiryTime: this.expiryTime,
            enable: this.enable,
            tgId: this.tgId,
            subId: this.subId,
            reset: this.reset,
        };
    }

    static fromJson(json = {}) {
        return new Inbound.ShadowsocksSettings.Shadowsocks(
            json.method,
            json.password,
            json.email,
            json.limitIp,
            json.totalGB,
            json.expiryTime,
            json.enable,
            json.tgId,
            json.subId,
            json.reset,
        );
    }

    get _expiryTime() {
        if (this.expiryTime === 0 || this.expiryTime === "") {
            return null;
        }
        if (this.expiryTime < 0){
            return this.expiryTime / -86400000;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null || t === "") {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }
    get _totalGB() {
        return toFixed(this.totalGB / ONE_GB, 2);
    }

    set _totalGB(gb) {
        this.totalGB = toFixed(gb * ONE_GB, 0);
    }

};

Inbound.DokodemoSettings = class extends Inbound.Settings {
    constructor(protocol, address, port, network='tcp,udp', followRedirect=false) {
        super(protocol);
        this.address = address;
        this.port = port;
        this.network = network;
        this.followRedirect = followRedirect;
    }

    static fromJson(json={}) {
        return new Inbound.DokodemoSettings(
            Protocols.DOKODEMO,
            json.address,
            json.port,
            json.network,
            json.followRedirect,
        );
    }

    toJson() {
        return {
            address: this.address,
            port: this.port,
            network: this.network,
            followRedirect: this.followRedirect,
        };
    }
};

Inbound.SocksSettings = class extends Inbound.Settings {
    constructor(protocol, auth='password', accounts=[new Inbound.SocksSettings.SocksAccount()], udp=false, ip='127.0.0.1') {
        super(protocol);
        this.auth = auth;
        this.accounts = accounts;
        this.udp = udp;
        this.ip = ip;
    }

    addAccount(account) {
        this.accounts.push(account);
    }

    delAccount(index) {
        this.accounts.splice(index, 1);
    }

    static fromJson(json={}) {
        let accounts;
        if (json.auth === 'password') {
            accounts = json.accounts.map(
                account => Inbound.SocksSettings.SocksAccount.fromJson(account)
            )
        }
        return new Inbound.SocksSettings(
            Protocols.SOCKS,
            json.auth,
            accounts,
            json.udp,
            json.ip,
        );
    }

    toJson() {
        return {
            auth: this.auth,
            accounts: this.auth === 'password' ? this.accounts.map(account => account.toJson()) : undefined,
            udp: this.udp,
            ip: this.ip,
        };
    }
};
Inbound.SocksSettings.SocksAccount = class extends XrayCommonClass {
    constructor(user=RandomUtil.randomSeq(10), pass=RandomUtil.randomSeq(10)) {
        super();
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json={}) {
        return new Inbound.SocksSettings.SocksAccount(json.user, json.pass);
    }
};

Inbound.HttpSettings = class extends Inbound.Settings {
    constructor(protocol, accounts=[new Inbound.HttpSettings.HttpAccount()]) {
        super(protocol);
        this.accounts = accounts;
    }

    addAccount(account) {
        this.accounts.push(account);
    }

    delAccount(index) {
        this.accounts.splice(index, 1);
    }

    static fromJson(json={}) {
        return new Inbound.HttpSettings(
            Protocols.HTTP,
            json.accounts.map(account => Inbound.HttpSettings.HttpAccount.fromJson(account)),
        );
    }

    toJson() {
        return {
            accounts: Inbound.HttpSettings.toJsonArray(this.accounts),
        };
    }
};

Inbound.HttpSettings.HttpAccount = class extends XrayCommonClass {
    constructor(user=RandomUtil.randomSeq(10), pass=RandomUtil.randomSeq(10)) {
        super();
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json={}) {
        return new Inbound.HttpSettings.HttpAccount(json.user, json.pass);
    }
};

Inbound.WireguardSettings = class extends XrayCommonClass {
    constructor(protocol, mtu=1420, secretKey=Wireguard.generateKeypair().privateKey, peers=[new Inbound.WireguardSettings.Peer()], kernelMode=false) {
        super(protocol);
        this.mtu = mtu;
        this.secretKey = secretKey;
        this.pubKey = secretKey.length>0 ? Wireguard.generateKeypair(secretKey).publicKey : '';
        this.peers = peers;
        this.kernelMode = kernelMode;
    }

    addPeer() {
        this.peers.push(new Inbound.WireguardSettings.Peer(null,null,'',['10.0.0.' + (this.peers.length+2)]));
    }

    delPeer(index) {
        this.peers.splice(index, 1);
    }

    static fromJson(json={}){
        return new Inbound.WireguardSettings(
            Protocols.WIREGUARD,
            json.mtu,
            json.secretKey,
            json.peers.map(peer => Inbound.WireguardSettings.Peer.fromJson(peer)),
            json.kernelMode,
        );
    }

    toJson() {
        return {
            mtu: this.mtu?? undefined,
            secretKey: this.secretKey,
            peers: Inbound.WireguardSettings.Peer.toJsonArray(this.peers),
            kernelMode: this.kernelMode,
        };
    }
};

Inbound.WireguardSettings.Peer = class extends XrayCommonClass {
    constructor(privateKey, publicKey, psk='', allowedIPs=['10.0.0.2/32'], keepAlive=0) {
        super();
        this.privateKey = privateKey
        this.publicKey = publicKey;
        if (!this.publicKey){
            [this.publicKey, this.privateKey] = Object.values(Wireguard.generateKeypair())
        }
        this.psk = psk;
        allowedIPs.forEach((a,index) => {
            if (a.length>0 && !a.includes('/')) allowedIPs[index] += '/32';
        })
        this.allowedIPs = allowedIPs;
        this.keepAlive = keepAlive;
    }

    static fromJson(json={}){
        return new Inbound.WireguardSettings.Peer(
            json.privateKey,
            json.publicKey,
            json.preSharedKey,
            json.allowedIPs,
            json.keepAlive
        );
    }

    toJson() {
        this.allowedIPs.forEach((a,index) => {
            if (a.length>0 && !a.includes('/')) this.allowedIPs[index] += '/32';
        });
        return {
            privateKey: this.privateKey,            
            publicKey: this.publicKey,
            preSharedKey: this.psk.length>0 ? this.psk : undefined,
            allowedIPs: this.allowedIPs,
            keepAlive: this.keepAlive?? undefined,
        };
    }
};