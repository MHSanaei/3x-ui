const Protocols = {
    VMESS: 'vmess',
    VLESS: 'vless',
    TROJAN: 'trojan',
    SHADOWSOCKS: 'shadowsocks',
    WIREGUARD: 'wireguard',
    HYSTERIA: 'hysteria',
    MIXED: 'mixed',
    HTTP: 'http',
    TUNNEL: 'tunnel',
    TUN: 'tun',
};

const SSMethods = {
    CHACHA20_POLY1305: 'chacha20-poly1305',
    CHACHA20_IETF_POLY1305: 'chacha20-ietf-poly1305',
    XCHACHA20_IETF_POLY1305: 'xchacha20-ietf-poly1305',
    BLAKE3_AES_128_GCM: '2022-blake3-aes-128-gcm',
    BLAKE3_AES_256_GCM: '2022-blake3-aes-256-gcm',
    BLAKE3_CHACHA20_POLY1305: '2022-blake3-chacha20-poly1305',
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
    UTLS_RONDOMIZEDNOALPN: "randomizednoalpn",
    UTLS_UNSAFE: "unsafe",
};

const ALPN_OPTION = {
    H3: "h3",
    H2: "h2",
    HTTP1: "http/1.1",
};

const SNIFFING_OPTION = {
    HTTP: "http",
    TLS: "tls",
    QUIC: "quic",
    FAKEDNS: "fakedns"
};

const USAGE_OPTION = {
    ENCIPHERMENT: "encipherment",
    VERIFY: "verify",
    ISSUE: "issue",
};

const DOMAIN_STRATEGY_OPTION = {
    AS_IS: "AsIs",
    USE_IP: "UseIP",
    USE_IPV6V4: "UseIPv6v4",
    USE_IPV6: "UseIPv6",
    USE_IPV4V6: "UseIPv4v6",
    USE_IPV4: "UseIPv4",
    FORCE_IP: "ForceIP",
    FORCE_IPV6V4: "ForceIPv6v4",
    FORCE_IPV6: "ForceIPv6",
    FORCE_IPV4V6: "ForceIPv4v6",
    FORCE_IPV4: "ForceIPv4",
};

const TCP_CONGESTION_OPTION = {
    BBR: "bbr",
    CUBIC: "cubic",
    RENO: "reno",
};

const USERS_SECURITY = {
    AES_128_GCM: "aes-128-gcm",
    CHACHA20_POLY1305: "chacha20-poly1305",
    AUTO: "auto",
    NONE: "none",
    ZERO: "zero",
};

const MODE_OPTION = {
    AUTO: "auto",
    PACKET_UP: "packet-up",
    STREAM_UP: "stream-up",
    STREAM_ONE: "stream-one",
};

Object.freeze(Protocols);
Object.freeze(SSMethods);
Object.freeze(TLS_FLOW_CONTROL);
Object.freeze(TLS_VERSION_OPTION);
Object.freeze(TLS_CIPHER_OPTION);
Object.freeze(UTLS_FINGERPRINT);
Object.freeze(ALPN_OPTION);
Object.freeze(SNIFFING_OPTION);
Object.freeze(USAGE_OPTION);
Object.freeze(DOMAIN_STRATEGY_OPTION);
Object.freeze(TCP_CONGESTION_OPTION);
Object.freeze(USERS_SECURITY);
Object.freeze(MODE_OPTION);

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

    toString(format = true) {
        return format ? JSON.stringify(this.toJson(), null, 2) : JSON.stringify(this.toJson());
    }

    static toHeaders(v2Headers) {
        let newHeaders = [];
        if (v2Headers) {
            Object.keys(v2Headers).forEach(key => {
                let values = v2Headers[key];
                if (typeof (values) === 'string') {
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

    static toV2Headers(headers, arr = true) {
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
    constructor(
        acceptProxyProtocol = false,
        type = 'none',
        request = new TcpStreamSettings.TcpRequest(),
        response = new TcpStreamSettings.TcpResponse(),
    ) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.type = type;
        this.request = request;
        this.response = response;
    }

    static fromJson(json = {}) {
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
    constructor(
        version = '1.1',
        method = 'GET',
        path = ['/'],
        headers = [],
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

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json = {}) {
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
    constructor(
        version = '1.1',
        status = '200',
        reason = 'OK',
        headers = [],
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

    static fromJson(json = {}) {
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
    constructor(
        mtu = 1350,
        tti = 20,
        uplinkCapacity = 5,
        downlinkCapacity = 20,
        cwndMultiplier = 1,
        maxSendingWindow = 2097152,
    ) {
        super();
        this.mtu = mtu;
        this.tti = tti;
        this.upCap = uplinkCapacity;
        this.downCap = downlinkCapacity;
        this.cwndMultiplier = cwndMultiplier;
        this.maxSendingWindow = maxSendingWindow;
    }

    static fromJson(json = {}) {
        return new KcpStreamSettings(
            json.mtu,
            json.tti,
            json.uplinkCapacity,
            json.downlinkCapacity,
            json.cwndMultiplier,
            json.maxSendingWindow,
        );
    }

    toJson() {
        return {
            mtu: this.mtu,
            tti: this.tti,
            uplinkCapacity: this.upCap,
            downlinkCapacity: this.downCap,
            cwndMultiplier: this.cwndMultiplier,
            maxSendingWindow: this.maxSendingWindow,
        };
    }
}

class WsStreamSettings extends XrayCommonClass {
    constructor(
        acceptProxyProtocol = false,
        path = '/',
        host = '',
        headers = [],
        heartbeatPeriod = 0,
    ) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.path = path;
        this.host = host;
        this.headers = headers;
        this.heartbeatPeriod = heartbeatPeriod;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new WsStreamSettings(
            json.acceptProxyProtocol,
            json.path,
            json.host,
            XrayCommonClass.toHeaders(json.headers),
            json.heartbeatPeriod,
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            path: this.path,
            host: this.host,
            headers: XrayCommonClass.toV2Headers(this.headers, false),
            heartbeatPeriod: this.heartbeatPeriod,
        };
    }
}

class GrpcStreamSettings extends XrayCommonClass {
    constructor(
        serviceName = "",
        authority = "",
        multiMode = false,
    ) {
        super();
        this.serviceName = serviceName;
        this.authority = authority;
        this.multiMode = multiMode;
    }

    static fromJson(json = {}) {
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

class HTTPUpgradeStreamSettings extends XrayCommonClass {
    constructor(
        acceptProxyProtocol = false,
        path = '/',
        host = '',
        headers = []
    ) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.path = path;
        this.host = host;
        this.headers = headers;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new HTTPUpgradeStreamSettings(
            json.acceptProxyProtocol,
            json.path,
            json.host,
            XrayCommonClass.toHeaders(json.headers),
        );
    }

    toJson() {
        return {
            acceptProxyProtocol: this.acceptProxyProtocol,
            path: this.path,
            host: this.host,
            headers: XrayCommonClass.toV2Headers(this.headers, false),
        };
    }
}

class xHTTPStreamSettings extends XrayCommonClass {
    constructor(
        path = '/',
        host = '',
        headers = [],
        scMaxBufferedPosts = 30,
        scMaxEachPostBytes = "1000000",
        scStreamUpServerSecs = "20-80",
        noSSEHeader = false,
        xPaddingBytes = "100-1000",
        mode = MODE_OPTION.AUTO,
        xPaddingObfsMode = false,
        xPaddingKey = '',
        xPaddingHeader = '',
        xPaddingPlacement = '',
        xPaddingMethod = '',
        uplinkHTTPMethod = '',
        sessionPlacement = '',
        sessionKey = '',
        seqPlacement = '',
        seqKey = '',
        uplinkDataPlacement = '',
        uplinkDataKey = '',
        uplinkChunkSize = 0,
    ) {
        super();
        this.path = path;
        this.host = host;
        this.headers = headers;
        this.scMaxBufferedPosts = scMaxBufferedPosts;
        this.scMaxEachPostBytes = scMaxEachPostBytes;
        this.scStreamUpServerSecs = scStreamUpServerSecs;
        this.noSSEHeader = noSSEHeader;
        this.xPaddingBytes = xPaddingBytes;
        this.mode = mode;
        this.xPaddingObfsMode = xPaddingObfsMode;
        this.xPaddingKey = xPaddingKey;
        this.xPaddingHeader = xPaddingHeader;
        this.xPaddingPlacement = xPaddingPlacement;
        this.xPaddingMethod = xPaddingMethod;
        this.uplinkHTTPMethod = uplinkHTTPMethod;
        this.sessionPlacement = sessionPlacement;
        this.sessionKey = sessionKey;
        this.seqPlacement = seqPlacement;
        this.seqKey = seqKey;
        this.uplinkDataPlacement = uplinkDataPlacement;
        this.uplinkDataKey = uplinkDataKey;
        this.uplinkChunkSize = uplinkChunkSize;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new xHTTPStreamSettings(
            json.path,
            json.host,
            XrayCommonClass.toHeaders(json.headers),
            json.scMaxBufferedPosts,
            json.scMaxEachPostBytes,
            json.scStreamUpServerSecs,
            json.noSSEHeader,
            json.xPaddingBytes,
            json.mode,
            json.xPaddingObfsMode,
            json.xPaddingKey,
            json.xPaddingHeader,
            json.xPaddingPlacement,
            json.xPaddingMethod,
            json.uplinkHTTPMethod,
            json.sessionPlacement,
            json.sessionKey,
            json.seqPlacement,
            json.seqKey,
            json.uplinkDataPlacement,
            json.uplinkDataKey,
            json.uplinkChunkSize,
        );
    }

    toJson() {
        return {
            path: this.path,
            host: this.host,
            headers: XrayCommonClass.toV2Headers(this.headers, false),
            scMaxBufferedPosts: this.scMaxBufferedPosts,
            scMaxEachPostBytes: this.scMaxEachPostBytes,
            scStreamUpServerSecs: this.scStreamUpServerSecs,
            noSSEHeader: this.noSSEHeader,
            xPaddingBytes: this.xPaddingBytes,
            mode: this.mode,
            xPaddingObfsMode: this.xPaddingObfsMode,
            xPaddingKey: this.xPaddingKey,
            xPaddingHeader: this.xPaddingHeader,
            xPaddingPlacement: this.xPaddingPlacement,
            xPaddingMethod: this.xPaddingMethod,
            uplinkHTTPMethod: this.uplinkHTTPMethod,
            sessionPlacement: this.sessionPlacement,
            sessionKey: this.sessionKey,
            seqPlacement: this.seqPlacement,
            seqKey: this.seqKey,
            uplinkDataPlacement: this.uplinkDataPlacement,
            uplinkDataKey: this.uplinkDataKey,
            uplinkChunkSize: this.uplinkChunkSize,
        };
    }
}

class HysteriaStreamSettings extends XrayCommonClass {
    constructor(
        protocol,
        version = 2,
        auth = '',
        udpIdleTimeout = 60,
        masquerade,
    ) {
        super(protocol);
        this.version = version;
        this.auth = auth;
        this.udpIdleTimeout = udpIdleTimeout;
        this.masquerade = masquerade;
    }

    static fromJson(json = {}) {
        return new HysteriaStreamSettings(
            json.protocol,
            json.version ?? 2,
            json.auth ?? '',
            json.udpIdleTimeout ?? 60,
            json.masquerade ? HysteriaMasquerade.fromJson(json.masquerade) : undefined,
        );
    }

    toJson() {
        return {
            protocol: this.protocol,
            version: this.version,
            auth: this.auth,
            udpIdleTimeout: this.udpIdleTimeout,
            masquerade: this.masqueradeSwitch ? this.masquerade.toJson() : undefined,
        };
    }

    get masqueradeSwitch() {
        return this.masquerade != undefined;
    }

    set masqueradeSwitch(value) {
        this.masquerade = value ? new HysteriaMasquerade() : undefined;
    }
};

class HysteriaMasquerade extends XrayCommonClass {
    constructor(
        type = 'proxy',
        dir = '',
        url = '',
        rewriteHost = false,
        insecure = false,
        content = '',
        headers = [],
        statusCode = 0,
    ) {
        super();
        this.type = type;
        this.dir = dir;
        this.url = url;
        this.rewriteHost = rewriteHost;
        this.insecure = insecure;
        this.content = content;
        this.headers = headers;
        this.statusCode = statusCode;
    }

    addHeader(name, value) {
        this.headers.push({ name: name, value: value });
    }

    removeHeader(index) {
        this.headers.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new HysteriaMasquerade(
            json.type,
            json.dir,
            json.url,
            json.rewriteHost,
            json.insecure,
            json.content,
            XrayCommonClass.toHeaders(json.headers),
            json.statusCode,
        );
    }

    toJson() {
        return {
            type: this.type,
            dir: this.dir,
            url: this.url,
            rewriteHost: this.rewriteHost,
            insecure: this.insecure,
            content: this.content,
            headers: XrayCommonClass.toV2Headers(this.headers, false),
            statusCode: this.statusCode,
        };
    }
};
class TlsStreamSettings extends XrayCommonClass {
    constructor(
        serverName = '',
        minVersion = TLS_VERSION_OPTION.TLS12,
        maxVersion = TLS_VERSION_OPTION.TLS13,
        cipherSuites = '',
        rejectUnknownSni = false,
        disableSystemRoot = false,
        enableSessionResumption = false,
        certificates = [new TlsStreamSettings.Cert()],
        alpn = [ALPN_OPTION.H2, ALPN_OPTION.HTTP1],
        echServerKeys = '',
        settings = new TlsStreamSettings.Settings()
    ) {
        super();
        this.sni = serverName;
        this.minVersion = minVersion;
        this.maxVersion = maxVersion;
        this.cipherSuites = cipherSuites;
        this.rejectUnknownSni = rejectUnknownSni;
        this.disableSystemRoot = disableSystemRoot;
        this.enableSessionResumption = enableSessionResumption;
        this.certs = certificates;
        this.alpn = alpn;
        this.echServerKeys = echServerKeys;
        this.settings = settings;
    }

    addCert() {
        this.certs.push(new TlsStreamSettings.Cert());
    }

    removeCert(index) {
        this.certs.splice(index, 1);
    }

    static fromJson(json = {}) {
        let certs;
        let settings;
        if (!ObjectUtil.isEmpty(json.certificates)) {
            certs = json.certificates.map(cert => TlsStreamSettings.Cert.fromJson(cert));
        }

        if (!ObjectUtil.isEmpty(json.settings)) {
            settings = new TlsStreamSettings.Settings(json.settings.fingerprint, json.settings.echConfigList);
        }
        return new TlsStreamSettings(
            json.serverName,
            json.minVersion,
            json.maxVersion,
            json.cipherSuites,
            json.rejectUnknownSni,
            json.disableSystemRoot,
            json.enableSessionResumption,
            certs,
            json.alpn,
            json.echServerKeys,
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
            disableSystemRoot: this.disableSystemRoot,
            enableSessionResumption: this.enableSessionResumption,
            certificates: TlsStreamSettings.toJsonArray(this.certs),
            alpn: this.alpn,
            echServerKeys: this.echServerKeys,
            settings: this.settings,
        };
    }
}

TlsStreamSettings.Cert = class extends XrayCommonClass {
    constructor(
        useFile = true,
        certificateFile = '',
        keyFile = '',
        certificate = '',
        key = '',
        oneTimeLoading = false,
        usage = USAGE_OPTION.ENCIPHERMENT,
        buildChain = false,
    ) {
        super();
        this.useFile = useFile;
        this.certFile = certificateFile;
        this.keyFile = keyFile;
        this.cert = Array.isArray(certificate) ? certificate.join('\n') : certificate;
        this.key = Array.isArray(key) ? key.join('\n') : key;
        this.oneTimeLoading = oneTimeLoading;
        this.usage = usage;
        this.buildChain = buildChain
    }

    static fromJson(json = {}) {
        if ('certificateFile' in json && 'keyFile' in json) {
            return new TlsStreamSettings.Cert(
                true,
                json.certificateFile,
                json.keyFile, '', '',
                json.oneTimeLoading,
                json.usage,
                json.buildChain,
            );
        } else {
            return new TlsStreamSettings.Cert(
                false, '', '',
                json.certificate.join('\n'),
                json.key.join('\n'),
                json.oneTimeLoading,
                json.usage,
                json.buildChain,
            );
        }
    }

    toJson() {
        if (this.useFile) {
            return {
                certificateFile: this.certFile,
                keyFile: this.keyFile,
                oneTimeLoading: this.oneTimeLoading,
                usage: this.usage,
                buildChain: this.buildChain,
            };
        } else {
            return {
                certificate: this.cert.split('\n'),
                key: this.key.split('\n'),
                oneTimeLoading: this.oneTimeLoading,
                usage: this.usage,
                buildChain: this.buildChain,
            };
        }
    }
};

TlsStreamSettings.Settings = class extends XrayCommonClass {
    constructor(
        fingerprint = UTLS_FINGERPRINT.UTLS_CHROME,
        echConfigList = '',
    ) {
        super();
        this.fingerprint = fingerprint;
        this.echConfigList = echConfigList;
    }
    static fromJson(json = {}) {
        return new TlsStreamSettings.Settings(
            json.fingerprint,
            json.echConfigList,
        );
    }
    toJson() {
        return {
            fingerprint: this.fingerprint,
            echConfigList: this.echConfigList
        };
    }
};


class RealityStreamSettings extends XrayCommonClass {
    constructor(
        show = false,
        xver = 0,
        target = '',
        serverNames = '',
        privateKey = '',
        minClientVer = '',
        maxClientVer = '',
        maxTimediff = 0,
        shortIds = RandomUtil.randomShortIds(),
        mldsa65Seed = '',
        settings = new RealityStreamSettings.Settings()
    ) {
        super();
        // If target/serverNames are not provided, use random values
        if (!target && !serverNames) {
            const randomTarget = typeof getRandomRealityTarget !== 'undefined'
                ? getRandomRealityTarget()
                : { target: 'www.amazon.com:443', sni: 'www.amazon.com,amazon.com' };
            target = randomTarget.target;
            serverNames = randomTarget.sni;
        }
        this.show = show;
        this.xver = xver;
        this.target = target;
        this.serverNames = Array.isArray(serverNames) ? serverNames.join(",") : serverNames;
        this.privateKey = privateKey;
        this.minClientVer = minClientVer;
        this.maxClientVer = maxClientVer;
        this.maxTimediff = maxTimediff;
        this.shortIds = Array.isArray(shortIds) ? shortIds.join(",") : shortIds;
        this.mldsa65Seed = mldsa65Seed;
        this.settings = settings;
    }

    static fromJson(json = {}) {
        let settings;
        if (!ObjectUtil.isEmpty(json.settings)) {
            settings = new RealityStreamSettings.Settings(
                json.settings.publicKey,
                json.settings.fingerprint,
                json.settings.serverName,
                json.settings.spiderX,
                json.settings.mldsa65Verify,
            );
        }
        return new RealityStreamSettings(
            json.show,
            json.xver,
            json.target,
            json.serverNames,
            json.privateKey,
            json.minClientVer,
            json.maxClientVer,
            json.maxTimediff,
            json.shortIds,
            json.mldsa65Seed,
            settings,
        );
    }

    toJson() {
        return {
            show: this.show,
            xver: this.xver,
            target: this.target,
            serverNames: this.serverNames.split(","),
            privateKey: this.privateKey,
            minClientVer: this.minClientVer,
            maxClientVer: this.maxClientVer,
            maxTimediff: this.maxTimediff,
            shortIds: this.shortIds.split(","),
            mldsa65Seed: this.mldsa65Seed,
            settings: this.settings,
        };
    }
}

RealityStreamSettings.Settings = class extends XrayCommonClass {
    constructor(
        publicKey = '',
        fingerprint = UTLS_FINGERPRINT.UTLS_CHROME,
        serverName = '',
        spiderX = '/',
        mldsa65Verify = ''
    ) {
        super();
        this.publicKey = publicKey;
        this.fingerprint = fingerprint;
        this.serverName = serverName;
        this.spiderX = spiderX;
        this.mldsa65Verify = mldsa65Verify;
    }
    static fromJson(json = {}) {
        return new RealityStreamSettings.Settings(
            json.publicKey,
            json.fingerprint,
            json.serverName,
            json.spiderX,
            json.mldsa65Verify
        );
    }
    toJson() {
        return {
            publicKey: this.publicKey,
            fingerprint: this.fingerprint,
            serverName: this.serverName,
            spiderX: this.spiderX,
            mldsa65Verify: this.mldsa65Verify
        };
    }
};

class SockoptStreamSettings extends XrayCommonClass {
    constructor(
        acceptProxyProtocol = false,
        tcpFastOpen = false,
        mark = 0,
        tproxy = "off",
        tcpMptcp = false,
        penetrate = false,
        domainStrategy = DOMAIN_STRATEGY_OPTION.USE_IP,
        tcpMaxSeg = 1440,
        dialerProxy = "",
        tcpKeepAliveInterval = 0,
        tcpKeepAliveIdle = 300,
        tcpUserTimeout = 10000,
        tcpcongestion = TCP_CONGESTION_OPTION.BBR,
        V6Only = false,
        tcpWindowClamp = 600,
        interfaceName = "",
        trustedXForwardedFor = [],
    ) {
        super();
        this.acceptProxyProtocol = acceptProxyProtocol;
        this.tcpFastOpen = tcpFastOpen;
        this.mark = mark;
        this.tproxy = tproxy;
        this.tcpMptcp = tcpMptcp;
        this.penetrate = penetrate;
        this.domainStrategy = domainStrategy;
        this.tcpMaxSeg = tcpMaxSeg;
        this.dialerProxy = dialerProxy;
        this.tcpKeepAliveInterval = tcpKeepAliveInterval;
        this.tcpKeepAliveIdle = tcpKeepAliveIdle;
        this.tcpUserTimeout = tcpUserTimeout;
        this.tcpcongestion = tcpcongestion;
        this.V6Only = V6Only;
        this.tcpWindowClamp = tcpWindowClamp;
        this.interfaceName = interfaceName;
        this.trustedXForwardedFor = trustedXForwardedFor;
    }

    static fromJson(json = {}) {
        if (Object.keys(json).length === 0) return undefined;
        return new SockoptStreamSettings(
            json.acceptProxyProtocol,
            json.tcpFastOpen,
            json.mark,
            json.tproxy,
            json.tcpMptcp,
            json.penetrate,
            json.domainStrategy,
            json.tcpMaxSeg,
            json.dialerProxy,
            json.tcpKeepAliveInterval,
            json.tcpKeepAliveIdle,
            json.tcpUserTimeout,
            json.tcpcongestion,
            json.V6Only,
            json.tcpWindowClamp,
            json.interface,
            json.trustedXForwardedFor || [],
        );
    }

    toJson() {
        const result = {
            acceptProxyProtocol: this.acceptProxyProtocol,
            tcpFastOpen: this.tcpFastOpen,
            mark: this.mark,
            tproxy: this.tproxy,
            tcpMptcp: this.tcpMptcp,
            penetrate: this.penetrate,
            domainStrategy: this.domainStrategy,
            tcpMaxSeg: this.tcpMaxSeg,
            dialerProxy: this.dialerProxy,
            tcpKeepAliveInterval: this.tcpKeepAliveInterval,
            tcpKeepAliveIdle: this.tcpKeepAliveIdle,
            tcpUserTimeout: this.tcpUserTimeout,
            tcpcongestion: this.tcpcongestion,
            V6Only: this.V6Only,
            tcpWindowClamp: this.tcpWindowClamp,
            interface: this.interfaceName,
        };
        if (this.trustedXForwardedFor && this.trustedXForwardedFor.length > 0) {
            result.trustedXForwardedFor = this.trustedXForwardedFor;
        }
        return result;
    }
}

class UdpMask extends XrayCommonClass {
    constructor(type = 'salamander', settings = {}) {
        super();
        this.type = type;
        this.settings = this._getDefaultSettings(type, settings);
    }

    _getDefaultSettings(type, settings = {}) {
        switch (type) {
            case 'salamander':
            case 'mkcp-aes128gcm':
                return { password: settings.password || '' };
            case 'header-dns':
                return { domain: settings.domain || '' };
            case 'xdns':
                return { domains: Array.isArray(settings.domains) ? settings.domains : [] };
            case 'xicmp':
                return { ip: settings.ip || '', id: settings.id ?? 0 };
            case 'mkcp-original':
            case 'header-dtls':
            case 'header-srtp':
            case 'header-utp':
            case 'header-wechat':
            case 'header-wireguard':
                return {};
            case 'header-custom':
                return {
                    client: Array.isArray(settings.client) ? settings.client : [],
                    server: Array.isArray(settings.server) ? settings.server : [],
                };
            case 'noise':
                return {
                    reset: settings.reset ?? 0,
                    noise: Array.isArray(settings.noise) ? settings.noise : [],
                };
            default:
                return settings;
        }
    }

    static fromJson(json = {}) {
        return new UdpMask(
            json.type || 'salamander',
            json.settings || {}
        );
    }

    toJson() {
        const cleanItem = item => {
            const out = { ...item };
            if (out.type === 'array') {
                delete out.packet;
            } else {
                delete out.rand;
                delete out.randRange;
            }
            return out;
        };

        let settings = this.settings;
        if (this.type === 'noise' && settings && Array.isArray(settings.noise)) {
            settings = { ...settings, noise: settings.noise.map(cleanItem) };
        } else if (this.type === 'header-custom' && settings) {
            settings = {
                ...settings,
                client: Array.isArray(settings.client) ? settings.client.map(cleanItem) : settings.client,
                server: Array.isArray(settings.server) ? settings.server.map(cleanItem) : settings.server,
            };
        }

        return {
            type: this.type,
            settings: (settings && Object.keys(settings).length > 0) ? settings : undefined
        };
    }
}

class TcpMask extends XrayCommonClass {
    constructor(type = 'fragment', settings = {}) {
        super();
        this.type = type;
        this.settings = this._getDefaultSettings(type, settings);
    }

    _getDefaultSettings(type, settings = {}) {
        switch (type) {
            case 'fragment':
                return {
                    packets: settings.packets ?? 'tlshello',
                    length: settings.length ?? '',
                    delay: settings.delay ?? '',
                    maxSplit: settings.maxSplit ?? '',
                };
            case 'sudoku':
                return {
                    password: settings.password ?? '',
                    ascii: settings.ascii ?? '',
                    customTable: settings.customTable ?? '',
                    customTables: Array.isArray(settings.customTables) ? settings.customTables : [],
                    paddingMin: settings.paddingMin ?? 0,
                    paddingMax: settings.paddingMax ?? 0,
                };
            case 'header-custom':
                return {
                    clients: Array.isArray(settings.clients) ? settings.clients : [],
                    servers: Array.isArray(settings.servers) ? settings.servers : [],
                };
            default:
                return settings;
        }
    }

    static fromJson(json = {}) {
        return new TcpMask(
            json.type || 'fragment',
            json.settings || {}
        );
    }

    toJson() {
        const cleanItem = item => {
            const out = { ...item };
            if (out.type === 'array') {
                delete out.packet;
            } else {
                delete out.rand;
                delete out.randRange;
            }
            return out;
        };

        let settings = this.settings;
        if (this.type === 'header-custom' && settings) {
            const cleanGroup = group => Array.isArray(group) ? group.map(cleanItem) : group;
            settings = {
                ...settings,
                clients: Array.isArray(settings.clients) ? settings.clients.map(cleanGroup) : settings.clients,
                servers: Array.isArray(settings.servers) ? settings.servers.map(cleanGroup) : settings.servers,
            };
        }

        return {
            type: this.type,
            settings: (settings && Object.keys(settings).length > 0) ? settings : undefined
        };
    }
}

class QuicParams extends XrayCommonClass {
    constructor(
        congestion = 'bbr',
        debug = false,
        brutalUp = 65537,
        brutalDown = 65537,
        udpHop = undefined,
        initStreamReceiveWindow = 8388608,
        maxStreamReceiveWindow = 8388608,
        initConnectionReceiveWindow = 20971520,
        maxConnectionReceiveWindow = 20971520,
        maxIdleTimeout = 30,
        keepAlivePeriod = 5,
        disablePathMTUDiscovery = false,
        maxIncomingStreams = 1024,
    ) {
        super();
        this.congestion = congestion;
        this.debug = debug;
        this.brutalUp = brutalUp;
        this.brutalDown = brutalDown;
        this.udpHop = udpHop;
        this.initStreamReceiveWindow = initStreamReceiveWindow;
        this.maxStreamReceiveWindow = maxStreamReceiveWindow;
        this.initConnectionReceiveWindow = initConnectionReceiveWindow;
        this.maxConnectionReceiveWindow = maxConnectionReceiveWindow;
        this.maxIdleTimeout = maxIdleTimeout;
        this.keepAlivePeriod = keepAlivePeriod;
        this.disablePathMTUDiscovery = disablePathMTUDiscovery;
        this.maxIncomingStreams = maxIncomingStreams;
    }

    get hasUdpHop() {
        return this.udpHop != null;
    }

    set hasUdpHop(value) {
        this.udpHop = value ? (this.udpHop || { ports: '20000-50000', interval: '5-10' }) : undefined;
    }

    static fromJson(json = {}) {
        if (!json || Object.keys(json).length === 0) return undefined;
        return new QuicParams(
            json.congestion,
            json.debug,
            json.brutalUp,
            json.brutalDown,
            json.udpHop ? { ports: json.udpHop.ports, interval: json.udpHop.interval } : undefined,
            json.initStreamReceiveWindow,
            json.maxStreamReceiveWindow,
            json.initConnectionReceiveWindow,
            json.maxConnectionReceiveWindow,
            json.maxIdleTimeout,
            json.keepAlivePeriod,
            json.disablePathMTUDiscovery,
            json.maxIncomingStreams,
        );
    }

    toJson() {
        const result = { congestion: this.congestion };
        if (this.debug) result.debug = this.debug;
        if (['brutal', 'force-brutal'].includes(this.congestion)) {
            if (this.brutalUp) result.brutalUp = this.brutalUp;
            if (this.brutalDown) result.brutalDown = this.brutalDown;
        }
        if (this.udpHop) result.udpHop = { ports: this.udpHop.ports, interval: this.udpHop.interval };
        if (this.initStreamReceiveWindow > 0) result.initStreamReceiveWindow = this.initStreamReceiveWindow;
        if (this.maxStreamReceiveWindow > 0) result.maxStreamReceiveWindow = this.maxStreamReceiveWindow;
        if (this.initConnectionReceiveWindow > 0) result.initConnectionReceiveWindow = this.initConnectionReceiveWindow;
        if (this.maxConnectionReceiveWindow > 0) result.maxConnectionReceiveWindow = this.maxConnectionReceiveWindow;
        if (this.maxIdleTimeout !== 30 && this.maxIdleTimeout > 0) result.maxIdleTimeout = this.maxIdleTimeout;
        if (this.keepAlivePeriod > 0) result.keepAlivePeriod = this.keepAlivePeriod;
        if (this.disablePathMTUDiscovery) result.disablePathMTUDiscovery = this.disablePathMTUDiscovery;
        if (this.maxIncomingStreams > 0) result.maxIncomingStreams = this.maxIncomingStreams;
        return result;
    }
}

class FinalMaskStreamSettings extends XrayCommonClass {
    constructor(tcp = [], udp = [], quicParams = undefined) {
        super();
        this.tcp = Array.isArray(tcp) ? tcp.map(t => t instanceof TcpMask ? t : new TcpMask(t.type, t.settings)) : [];
        this.udp = Array.isArray(udp) ? udp.map(u => new UdpMask(u.type, u.settings)) : [new UdpMask(udp.type, udp.settings)];
        this.quicParams = quicParams instanceof QuicParams ? quicParams : (quicParams ? QuicParams.fromJson(quicParams) : undefined);
    }

    get enableQuicParams() {
        return this.quicParams != null;
    }

    set enableQuicParams(value) {
        this.quicParams = value ? (this.quicParams || new QuicParams()) : undefined;
    }

    static fromJson(json = {}) {
        return new FinalMaskStreamSettings(
            json.tcp || [],
            json.udp || [],
            json.quicParams ? QuicParams.fromJson(json.quicParams) : undefined,
        );
    }

    toJson() {
        const result = {};
        if (this.tcp && this.tcp.length > 0) {
            result.tcp = this.tcp.map(t => t.toJson());
        }
        if (this.udp && this.udp.length > 0) {
            result.udp = this.udp.map(udp => udp.toJson());
        }
        if (this.quicParams) {
            result.quicParams = this.quicParams.toJson();
        }
        return result;
    }
}

class StreamSettings extends XrayCommonClass {
    constructor(network = 'tcp',
        security = 'none',
        externalProxy = [],
        tlsSettings = new TlsStreamSettings(),
        realitySettings = new RealityStreamSettings(),
        tcpSettings = new TcpStreamSettings(),
        kcpSettings = new KcpStreamSettings(),
        wsSettings = new WsStreamSettings(),
        grpcSettings = new GrpcStreamSettings(),
        httpupgradeSettings = new HTTPUpgradeStreamSettings(),
        xhttpSettings = new xHTTPStreamSettings(),
        hysteriaSettings = new HysteriaStreamSettings(),
        finalmask = new FinalMaskStreamSettings(),
        sockopt = undefined,
    ) {
        super();
        this.network = network;
        this.security = security;
        this.externalProxy = externalProxy;
        this.tls = tlsSettings;
        this.reality = realitySettings;
        this.tcp = tcpSettings;
        this.kcp = kcpSettings;
        this.ws = wsSettings;
        this.grpc = grpcSettings;
        this.httpupgrade = httpupgradeSettings;
        this.xhttp = xhttpSettings;
        this.hysteria = hysteriaSettings;
        this.finalmask = finalmask;
        this.sockopt = sockopt;
    }

    addTcpMask(type = 'fragment') {
        this.finalmask.tcp.push(new TcpMask(type));
    }

    delTcpMask(index) {
        if (this.finalmask.tcp) {
            this.finalmask.tcp.splice(index, 1);
        }
    }

    addUdpMask(type = 'salamander') {
        this.finalmask.udp.push(new UdpMask(type));
    }

    delUdpMask(index) {
        if (this.finalmask.udp) {
            this.finalmask.udp.splice(index, 1);
        }
    }

    get hasFinalMask() {
        const hasTcp = this.finalmask.tcp && this.finalmask.tcp.length > 0;
        const hasUdp = this.finalmask.udp && this.finalmask.udp.length > 0;
        const hasQuicParams = this.finalmask.quicParams != null;
        return hasTcp || hasUdp || hasQuicParams;
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

    static fromJson(json = {}) {
        return new StreamSettings(
            json.network,
            json.security,
            json.externalProxy,
            TlsStreamSettings.fromJson(json.tlsSettings),
            RealityStreamSettings.fromJson(json.realitySettings),
            TcpStreamSettings.fromJson(json.tcpSettings),
            KcpStreamSettings.fromJson(json.kcpSettings),
            WsStreamSettings.fromJson(json.wsSettings),
            GrpcStreamSettings.fromJson(json.grpcSettings),
            HTTPUpgradeStreamSettings.fromJson(json.httpupgradeSettings),
            xHTTPStreamSettings.fromJson(json.xhttpSettings),
            HysteriaStreamSettings.fromJson(json.hysteriaSettings),
            FinalMaskStreamSettings.fromJson(json.finalmask),
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
            realitySettings: this.isReality ? this.reality.toJson() : undefined,
            tcpSettings: network === 'tcp' ? this.tcp.toJson() : undefined,
            kcpSettings: network === 'kcp' ? this.kcp.toJson() : undefined,
            wsSettings: network === 'ws' ? this.ws.toJson() : undefined,
            grpcSettings: network === 'grpc' ? this.grpc.toJson() : undefined,
            httpupgradeSettings: network === 'httpupgrade' ? this.httpupgrade.toJson() : undefined,
            xhttpSettings: network === 'xhttp' ? this.xhttp.toJson() : undefined,
            hysteriaSettings: network === 'hysteria' ? this.hysteria.toJson() : undefined,
            finalmask: this.hasFinalMask ? this.finalmask.toJson() : undefined,
            sockopt: this.sockopt != undefined ? this.sockopt.toJson() : undefined,
        };
    }
}

class Sniffing extends XrayCommonClass {
    constructor(
        enabled = false,
        destOverride = ['http', 'tls', 'quic', 'fakedns'],
        metadataOnly = false,
        routeOnly = false,
        ipsExcluded = [],
        domainsExcluded = []) {
        super();
        this.enabled = enabled;
        this.destOverride = Array.isArray(destOverride) && destOverride.length > 0 ? destOverride : ['http', 'tls', 'quic', 'fakedns'];
        this.metadataOnly = metadataOnly;
        this.routeOnly = routeOnly;
        this.ipsExcluded = Array.isArray(ipsExcluded) ? ipsExcluded : [];
        this.domainsExcluded = Array.isArray(domainsExcluded) ? domainsExcluded : [];
    }

    static fromJson(json = {}) {
        let destOverride = ObjectUtil.clone(json.destOverride);
        if (ObjectUtil.isEmpty(destOverride) || ObjectUtil.isArrEmpty(destOverride) || ObjectUtil.isEmpty(destOverride[0])) {
            destOverride = ['http', 'tls', 'quic', 'fakedns'];
        }
        return new Sniffing(
            !!json.enabled,
            destOverride,
            json.metadataOnly,
            json.routeOnly,
            json.ipsExcluded || [],
            json.domainsExcluded || [],
        );
    }

    toJson() {
        return {
            enabled: this.enabled,
            destOverride: this.destOverride,
            metadataOnly: this.metadataOnly,
            routeOnly: this.routeOnly,
            ipsExcluded: this.ipsExcluded.length > 0 ? this.ipsExcluded : undefined,
            domainsExcluded: this.domainsExcluded.length > 0 ? this.domainsExcluded : undefined,
        };
    }
}

class Inbound extends XrayCommonClass {
    constructor(
        port = RandomUtil.randomInteger(10000, 60000),
        listen = '',
        protocol = Protocols.VLESS,
        settings = null,
        streamSettings = new StreamSettings(),
        tag = '',
        sniffing = new Sniffing(),
        clientStats = '',
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

    // Copy the xPadding* settings into the query-string of a vless/trojan/ss
    // link. Without this, the admin's custom xPaddingBytes range and (in
    // obfs mode) the custom xPaddingKey / xPaddingHeader / placement /
    // method never reach the client — the client keeps xray / sing-box's
    // internal defaults and the server rejects every handshake with
    // `invalid padding (...) length: 0`.
    //
    // Two encodings are emitted so each client family can pick at least
    // one up:
    //   - x_padding_bytes=<range>       flat, for sing-box-family clients
    //   - extra=<url-encoded-json>       full blob, for xray-core clients
    //
    // Fields are only included when they actually have a value, so a
    // default inbound yields the same URL it did before this helper.
    static applyXhttpPaddingToParams(xhttp, params) {
        if (!xhttp) return;
        if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
            params.set("x_padding_bytes", xhttp.xPaddingBytes);
        }
        const extra = {};
        if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
            extra.xPaddingBytes = xhttp.xPaddingBytes;
        }
        if (xhttp.xPaddingObfsMode === true) {
            extra.xPaddingObfsMode = true;
            ["xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod"].forEach(k => {
                if (typeof xhttp[k] === 'string' && xhttp[k].length > 0) {
                    extra[k] = xhttp[k];
                }
            });
        }
        if (Object.keys(extra).length > 0) {
            params.set("extra", JSON.stringify(extra));
        }
    }

    // VMess variant: VMess links are a base64-encoded JSON object, so we
    // copy the padding fields directly into the JSON instead of building
    // a query string.
    static applyXhttpPaddingToObj(xhttp, obj) {
        if (!xhttp || !obj) return;
        if (typeof xhttp.xPaddingBytes === 'string' && xhttp.xPaddingBytes.length > 0) {
            obj.x_padding_bytes = xhttp.xPaddingBytes;
        }
        if (xhttp.xPaddingObfsMode === true) {
            obj.xPaddingObfsMode = true;
            ["xPaddingKey", "xPaddingHeader", "xPaddingPlacement", "xPaddingMethod"].forEach(k => {
                if (typeof xhttp[k] === 'string' && xhttp[k].length > 0) {
                    obj[k] = xhttp[k];
                }
            });
        }
    }

    static hasShareableFinalMaskValue(value) {
        if (value == null) {
            return false;
        }
        if (Array.isArray(value)) {
            return value.some(item => Inbound.hasShareableFinalMaskValue(item));
        }
        if (typeof value === 'object') {
            return Object.values(value).some(item => Inbound.hasShareableFinalMaskValue(item));
        }
        if (typeof value === 'string') {
            return value.length > 0;
        }
        return true;
    }

    static serializeFinalMask(finalmask) {
        if (!finalmask) {
            return '';
        }
        const value = typeof finalmask.toJson === 'function' ? finalmask.toJson() : finalmask;
        return Inbound.hasShareableFinalMaskValue(value) ? JSON.stringify(value) : '';
    }

    // Export finalmask with the same compact JSON payload shape that
    // v2rayN-compatible share links use: fm=<json>.
    static applyFinalMaskToParams(finalmask, params) {
        if (!params) return;
        const payload = Inbound.serializeFinalMask(finalmask);
        if (payload.length > 0) {
            params.set("fm", payload);
        }
    }

    // VMess links are a base64 JSON object, so keep the same fm payload
    // under a flat property instead of a URL query string.
    static applyFinalMaskToObj(finalmask, obj) {
        if (!obj) return;
        const payload = Inbound.serializeFinalMask(finalmask);
        if (payload.length > 0) {
            obj.fm = payload;
        }
    }

    get clients() {
        switch (this.protocol) {
            case Protocols.VMESS: return this.settings.vmesses;
            case Protocols.VLESS: return this.settings.vlesses;
            case Protocols.TROJAN: return this.settings.trojans;
            case Protocols.SHADOWSOCKS: return this.isSSMultiUser ? this.settings.shadowsockses : null;
            case Protocols.HYSTERIA: return this.settings.hysterias;
            default: return null;
        }
    }

    get protocol() {
        return this._protocol;
    }

    set protocol(protocol) {
        this._protocol = protocol;
        this.settings = Inbound.Settings.getSettings(protocol);
        this.stream = new StreamSettings();
        if (protocol === Protocols.TROJAN) {
            this.tls = false;
        }
        if (protocol === Protocols.HYSTERIA) {
            this.stream.network = 'hysteria';
            this.stream.security = 'tls';
            // Hysteria runs over QUIC and must not inherit TCP TLS ALPN defaults.
            this.stream.tls.alpn = [ALPN_OPTION.H3];
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

    get isGrpc() {
        return this.network === "grpc";
    }

    get isHttpupgrade() {
        return this.network === "httpupgrade";
    }

    get isXHTTP() {
        return this.network === "xhttp";
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
    get isSS2022() {
        return this.method.substring(0, 4) === "2022";
    }

    get serverName() {
        if (this.stream.isTls) return this.stream.tls.sni;
        if (this.stream.isReality) return this.stream.reality.serverNames;
        return "";
    }

    getHeader(obj, name) {
        for (const header of obj.headers) {
            if (header.name.toLowerCase() === name.toLowerCase()) {
                return header.value;
            }
        }
        return "";
    }

    get host() {
        if (this.isTcp) {
            return this.getHeader(this.stream.tcp.request, 'host');
        } else if (this.isWs) {
            return this.stream.ws.host?.length > 0 ? this.stream.ws.host : this.getHeader(this.stream.ws, 'host');
        } else if (this.isHttpupgrade) {
            return this.stream.httpupgrade.host?.length > 0 ? this.stream.httpupgrade.host : this.getHeader(this.stream.httpupgrade, 'host');
        } else if (this.isXHTTP) {
            return this.stream.xhttp.host?.length > 0 ? this.stream.xhttp.host : this.getHeader(this.stream.xhttp, 'host');
        }
        return null;
    }

    get path() {
        if (this.isTcp) {
            return this.stream.tcp.request.path[0];
        } else if (this.isWs) {
            return this.stream.ws.path;
        } else if (this.isHttpupgrade) {
            return this.stream.httpupgrade.path;
        } else if (this.isXHTTP) {
            return this.stream.xhttp.path;
        }
        return null;
    }

    get serviceName() {
        return this.stream.grpc.serviceName;
    }

    isExpiry(index) {
        let exp = this.clients[index].expiryTime;
        return exp > 0 ? exp < new Date().getTime() : false;
    }

    canEnableTls() {
        if (this.protocol === Protocols.HYSTERIA) return true;
        if (![Protocols.VMESS, Protocols.VLESS, Protocols.TROJAN, Protocols.SHADOWSOCKS].includes(this.protocol)) return false;
        return ["tcp", "ws", "http", "grpc", "httpupgrade", "xhttp"].includes(this.network);
    }

    //this is used for xtls-rprx-vision
    canEnableTlsFlow() {
        if (((this.stream.security === 'tls') || (this.stream.security === 'reality')) && (this.network === "tcp")) {
            return this.protocol === Protocols.VLESS;
        }
        return false;
    }

    // Vision seed applies only when the XTLS Vision (TCP/TLS) flow is selected.
    // Excludes the UDP variant per spec.
    canEnableVisionSeed() {
        if (!this.canEnableTlsFlow()) return false;
        const clients = this.settings?.vlesses;
        if (!Array.isArray(clients)) return false;
        return clients.some(c => c?.flow === TLS_FLOW_CONTROL.VISION);
    }

    canEnableReality() {
        if (![Protocols.VLESS, Protocols.TROJAN].includes(this.protocol)) return false;
        return ["tcp", "http", "grpc", "xhttp"].includes(this.network);
    }

    canEnableStream() {
        return [Protocols.VMESS, Protocols.VLESS, Protocols.TROJAN, Protocols.SHADOWSOCKS, Protocols.HYSTERIA].includes(this.protocol);
    }

    reset() {
        this.port = RandomUtil.randomInteger(10000, 60000);
        this.listen = '';
        this.protocol = Protocols.VMESS;
        this.settings = Inbound.Settings.getSettings(Protocols.VMESS);
        this.stream = new StreamSettings();
        this.tag = '';
        this.sniffing = new Sniffing();
    }

    genVmessLink(address = '', port = this.port, forceTls, remark = '', clientId, security) {
        if (this.protocol !== Protocols.VMESS) {
            return '';
        }
        const tls = forceTls == 'same' ? this.stream.security : forceTls;
        let obj = {
            v: '2',
            ps: remark,
            add: address,
            port: port,
            id: clientId,
            scy: security,
            net: this.stream.network,
            tls: tls,
        };
        const network = this.stream.network;
        if (network === 'tcp') {
            const tcp = this.stream.tcp;
            obj.type = tcp.type;
            if (tcp.type === 'http') {
                const request = tcp.request;
                obj.path = request.path.join(',');
                const host = this.getHeader(request, 'host');
                if (host) obj.host = host;
            }
        } else if (network === 'kcp') {
            const kcp = this.stream.kcp;
            obj.mtu = kcp.mtu;
            obj.tti = kcp.tti;
        } else if (network === 'ws') {
            const ws = this.stream.ws;
            obj.path = ws.path;
            obj.host = ws.host?.length > 0 ? ws.host : this.getHeader(ws, 'host');
        } else if (network === 'grpc') {
            obj.path = this.stream.grpc.serviceName;
            obj.authority = this.stream.grpc.authority;
            if (this.stream.grpc.multiMode) {
                obj.type = 'multi'
            }
        } else if (network === 'httpupgrade') {
            const httpupgrade = this.stream.httpupgrade;
            obj.path = httpupgrade.path;
            obj.host = httpupgrade.host?.length > 0 ? httpupgrade.host : this.getHeader(httpupgrade, 'host');
        } else if (network === 'xhttp') {
            const xhttp = this.stream.xhttp;
            obj.path = xhttp.path;
            obj.host = xhttp.host?.length > 0 ? xhttp.host : this.getHeader(xhttp, 'host');
            obj.type = xhttp.mode;
            Inbound.applyXhttpPaddingToObj(xhttp, obj);
        }

        Inbound.applyFinalMaskToObj(this.stream.finalmask, obj);

        if (tls === 'tls') {
            if (!ObjectUtil.isEmpty(this.stream.tls.sni)) {
                obj.sni = this.stream.tls.sni;
            }
            if (!ObjectUtil.isEmpty(this.stream.tls.settings.fingerprint)) {
                obj.fp = this.stream.tls.settings.fingerprint;
            }
            if (this.stream.tls.alpn.length > 0) {
                obj.alpn = this.stream.tls.alpn.join(',');
            }
        }

        return 'vmess://' + Base64.encode(JSON.stringify(obj, null, 2));
    }

    genVLESSLink(address = '', port = this.port, forceTls, remark = '', clientId, flow) {
        const uuid = clientId;
        const type = this.stream.network;
        const security = forceTls == 'same' ? this.stream.security : forceTls;
        const params = new Map();
        params.set("type", this.stream.network);
        params.set("encryption", this.settings.encryption);
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
                params.set("mtu", kcp.mtu);
                params.set("tti", kcp.tti);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                params.set("host", ws.host?.length > 0 ? ws.host : this.getHeader(ws, 'host'));
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if (grpc.multiMode) {
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                const httpupgrade = this.stream.httpupgrade;
                params.set("path", httpupgrade.path);
                params.set("host", httpupgrade.host?.length > 0 ? httpupgrade.host : this.getHeader(httpupgrade, 'host'));
                break;
            case "xhttp":
                const xhttp = this.stream.xhttp;
                params.set("path", xhttp.path);
                params.set("host", xhttp.host?.length > 0 ? xhttp.host : this.getHeader(xhttp, 'host'));
                params.set("mode", xhttp.mode);
                Inbound.applyXhttpPaddingToParams(xhttp, params);
                break;
        }

        Inbound.applyFinalMaskToParams(this.stream.finalmask, params);

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls) {
                params.set("fp", this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)) {
                    params.set("sni", this.stream.tls.sni);
                }
                if (this.stream.tls.settings.echConfigList?.length > 0) {
                    params.set("ech", this.stream.tls.settings.echConfigList);
                }
                if (type == "tcp" && !ObjectUtil.isEmpty(flow)) {
                    params.set("flow", flow);
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
            if (!ObjectUtil.isEmpty(this.stream.reality.settings.mldsa65Verify)) {
                params.set("pqv", this.stream.reality.settings.mldsa65Verify);
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

    genSSLink(address = '', port = this.port, forceTls, remark = '', clientPassword) {
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
                params.set("mtu", kcp.mtu);
                params.set("tti", kcp.tti);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                params.set("host", ws.host?.length > 0 ? ws.host : this.getHeader(ws, 'host'));
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if (grpc.multiMode) {
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                const httpupgrade = this.stream.httpupgrade;
                params.set("path", httpupgrade.path);
                params.set("host", httpupgrade.host?.length > 0 ? httpupgrade.host : this.getHeader(httpupgrade, 'host'));
                break;
            case "xhttp":
                const xhttp = this.stream.xhttp;
                params.set("path", xhttp.path);
                params.set("host", xhttp.host?.length > 0 ? xhttp.host : this.getHeader(xhttp, 'host'));
                params.set("mode", xhttp.mode);
                Inbound.applyXhttpPaddingToParams(xhttp, params);
                break;
        }

        Inbound.applyFinalMaskToParams(this.stream.finalmask, params);

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls) {
                params.set("fp", this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if (this.stream.tls.settings.echConfigList?.length > 0) {
                    params.set("ech", this.stream.tls.settings.echConfigList);
                }
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)) {
                    params.set("sni", this.stream.tls.sni);
                }
            }
        }


        let password = new Array();
        if (this.isSS2022) password.push(settings.password);
        if (this.isSSMultiUser) password.push(clientPassword);

        let link = `ss://${Base64.encode(`${settings.method}:${password.join(':')}`, true)}@${address}:${port}`;
        const url = new URL(link);
        for (const [key, value] of params) {
            url.searchParams.set(key, value)
        }
        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    genTrojanLink(address = '', port = this.port, forceTls, remark = '', clientPassword) {
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
                params.set("mtu", kcp.mtu);
                params.set("tti", kcp.tti);
                break;
            case "ws":
                const ws = this.stream.ws;
                params.set("path", ws.path);
                params.set("host", ws.host?.length > 0 ? ws.host : this.getHeader(ws, 'host'));
                break;
            case "grpc":
                const grpc = this.stream.grpc;
                params.set("serviceName", grpc.serviceName);
                params.set("authority", grpc.authority);
                if (grpc.multiMode) {
                    params.set("mode", "multi");
                }
                break;
            case "httpupgrade":
                const httpupgrade = this.stream.httpupgrade;
                params.set("path", httpupgrade.path);
                params.set("host", httpupgrade.host?.length > 0 ? httpupgrade.host : this.getHeader(httpupgrade, 'host'));
                break;
            case "xhttp":
                const xhttp = this.stream.xhttp;
                params.set("path", xhttp.path);
                params.set("host", xhttp.host?.length > 0 ? xhttp.host : this.getHeader(xhttp, 'host'));
                params.set("mode", xhttp.mode);
                Inbound.applyXhttpPaddingToParams(xhttp, params);
                break;
        }

        Inbound.applyFinalMaskToParams(this.stream.finalmask, params);

        if (security === 'tls') {
            params.set("security", "tls");
            if (this.stream.isTls) {
                params.set("fp", this.stream.tls.settings.fingerprint);
                params.set("alpn", this.stream.tls.alpn);
                if (this.stream.tls.settings.echConfigList?.length > 0) {
                    params.set("ech", this.stream.tls.settings.echConfigList);
                }
                if (!ObjectUtil.isEmpty(this.stream.tls.sni)) {
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
            if (!ObjectUtil.isEmpty(this.stream.reality.settings.mldsa65Verify)) {
                params.set("pqv", this.stream.reality.settings.mldsa65Verify);
            }
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

    genHysteriaLink(address = '', port = this.port, remark = '', clientAuth) {
        const protocol = this.settings.version == 2 ? "hysteria2" : "hysteria";
        const link = `${protocol}://${clientAuth}@${address}:${port}`;

        const params = new Map();
        params.set("security", "tls");
        if (this.stream.tls.settings.fingerprint?.length > 0) params.set("fp", this.stream.tls.settings.fingerprint);
        if (this.stream.tls.alpn?.length > 0) params.set("alpn", this.stream.tls.alpn);
        if (this.stream.tls.settings.allowInsecure) params.set("insecure", "1");
        if (this.stream.tls.settings.echConfigList?.length > 0) params.set("ech", this.stream.tls.settings.echConfigList);
        if (this.stream.tls.sni?.length > 0) params.set("sni", this.stream.tls.sni);

        const udpMasks = this.stream?.finalmask?.udp;
        if (Array.isArray(udpMasks)) {
            const salamanderMask = udpMasks.find(mask => mask?.type === 'salamander');
            const obfsPassword = salamanderMask?.settings?.password;
            if (typeof obfsPassword === 'string' && obfsPassword.length > 0) {
                params.set("obfs", "salamander");
                params.set("obfs-password", obfsPassword);
            }
        }

        Inbound.applyFinalMaskToParams(this.stream.finalmask, params);

        const url = new URL(link);
        for (const [key, value] of params) {
            url.searchParams.set(key, value);
        }
        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    getWireguardTxt(address, port, remark, peerId) {
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

    getWireguardLink(address, port, remark, peerId) {
        const peer = this.settings?.peers?.[peerId];
        if (!peer) return '';

        const link = `wireguard://${address}:${port}`;
        const url = new URL(link);
        url.username = peer.privateKey || '';

        if (this.settings?.pubKey) {
            url.searchParams.set("publickey", this.settings.pubKey);
        }
        if (Array.isArray(peer.allowedIPs) && peer.allowedIPs.length > 0 && peer.allowedIPs[0]) {
            url.searchParams.set("address", peer.allowedIPs[0]);
        }
        if (this.settings?.mtu) {
            url.searchParams.set("mtu", this.settings.mtu);
        }

        url.hash = encodeURIComponent(remark);
        return url.toString();
    }

    genWireguardLinks(remark = '', remarkModel = '-ieo') {
        const addr = !ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0" ? this.listen : location.hostname;
        const separationChar = remarkModel.charAt(0);
        let links = [];
        this.settings.peers.forEach((p, index) => {
            links.push(this.getWireguardLink(addr, this.port, remark + separationChar + (index + 1), index));
        });
        return links.join('\r\n');
    }

    genWireguardConfigs(remark = '', remarkModel = '-ieo') {
        const addr = !ObjectUtil.isEmpty(this.listen) && this.listen !== "0.0.0.0" ? this.listen : location.hostname;
        const separationChar = remarkModel.charAt(0);
        let links = [];
        this.settings.peers.forEach((p, index) => {
            links.push(this.getWireguardTxt(addr, this.port, remark + separationChar + (index + 1), index));
        });
        return links.join('\r\n');
    }

    genLink(address = '', port = this.port, forceTls = 'same', remark = '', client) {
        switch (this.protocol) {
            case Protocols.VMESS:
                return this.genVmessLink(address, port, forceTls, remark, client.id, client.security);
            case Protocols.VLESS:
                return this.genVLESSLink(address, port, forceTls, remark, client.id, client.flow);
            case Protocols.SHADOWSOCKS:
                return this.genSSLink(address, port, forceTls, remark, this.isSSMultiUser ? client.password : '');
            case Protocols.TROJAN:
                return this.genTrojanLink(address, port, forceTls, remark, client.password);
            case Protocols.HYSTERIA:
                return this.genHysteriaLink(address, port, remark, client.auth.length > 0 ? client.auth : this.stream.hysteria.auth);
            default: return '';
        }
    }

    genAllLinks(remark = '', remarkModel = '-ieo', client) {
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
        if (ObjectUtil.isArrEmpty(this.stream.externalProxy)) {
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
        if (this.clients) {
            let links = [];
            this.clients.forEach((client) => {
                this.genAllLinks(remark, remarkModel, client).forEach(l => {
                    links.push(l.link);
                })
            });
            return links.join('\r\n');
        } else {
            if (this.protocol == Protocols.SHADOWSOCKS && !this.isSSMultiUser) return this.genSSLink(addr, this.port, 'same', remark);
            if (this.protocol == Protocols.WIREGUARD) {
                return this.genWireguardConfigs(remark, remarkModel);
            }
            return '';
        }
    }

    static fromJson(json = {}) {
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
        if (this.canEnableStream() || this.stream?.sockopt) {
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
            case Protocols.TUNNEL: return new Inbound.TunnelSettings(protocol);
            case Protocols.MIXED: return new Inbound.MixedSettings(protocol);
            case Protocols.HTTP: return new Inbound.HttpSettings(protocol);
            case Protocols.WIREGUARD: return new Inbound.WireguardSettings(protocol);
            case Protocols.TUN: return new Inbound.TunSettings(protocol);
            case Protocols.HYSTERIA: return new Inbound.HysteriaSettings(protocol);
            default: return null;
        }
    }

    static fromJson(protocol, json) {
        switch (protocol) {
            case Protocols.VMESS: return Inbound.VmessSettings.fromJson(json);
            case Protocols.VLESS: return Inbound.VLESSSettings.fromJson(json);
            case Protocols.TROJAN: return Inbound.TrojanSettings.fromJson(json);
            case Protocols.SHADOWSOCKS: return Inbound.ShadowsocksSettings.fromJson(json);
            case Protocols.TUNNEL: return Inbound.TunnelSettings.fromJson(json);
            case Protocols.MIXED: return Inbound.MixedSettings.fromJson(json);
            case Protocols.HTTP: return Inbound.HttpSettings.fromJson(json);
            case Protocols.WIREGUARD: return Inbound.WireguardSettings.fromJson(json);
            case Protocols.TUN: return Inbound.TunSettings.fromJson(json);
            case Protocols.HYSTERIA: return Inbound.HysteriaSettings.fromJson(json);
            default: return null;
        }
    }

    toJson() {
        return {};
    }
};

/** Shared user-quota fields and UI helpers for multi-user protocol clients. */
Inbound.ClientBase = class extends XrayCommonClass {
    constructor(
        email = RandomUtil.randomLowerAndNum(8),
        limitIp = 0,
        totalGB = 0,
        expiryTime = 0,
        enable = true,
        tgId = '',
        subId = RandomUtil.randomLowerAndNum(16),
        comment = '',
        reset = 0,
        created_at = undefined,
        updated_at = undefined,
    ) {
        super();
        this.email = email;
        this.limitIp = limitIp;
        this.totalGB = totalGB;
        this.expiryTime = expiryTime;
        this.enable = enable;
        this.tgId = tgId;
        this.subId = subId;
        this.comment = comment;
        this.reset = reset;
        this.created_at = created_at;
        this.updated_at = updated_at;
    }

    static commonArgsFromJson(json = {}) {
        return [
            json.email,
            json.limitIp,
            json.totalGB,
            json.expiryTime,
            json.enable,
            json.tgId,
            json.subId,
            json.comment,
            json.reset,
            json.created_at,
            json.updated_at,
        ];
    }

    _clientBaseToJson() {
        return {
            email: this.email,
            limitIp: this.limitIp,
            totalGB: this.totalGB,
            expiryTime: this.expiryTime,
            enable: this.enable,
            tgId: this.tgId,
            subId: this.subId,
            comment: this.comment,
            reset: this.reset,
            created_at: this.created_at,
            updated_at: this.updated_at,
        };
    }

    get _expiryTime() {
        if (this.expiryTime === 0 || this.expiryTime === '') {
            return null;
        }
        if (this.expiryTime < 0) {
            return this.expiryTime / -86400000;
        }
        return moment(this.expiryTime);
    }

    set _expiryTime(t) {
        if (t == null || t === '') {
            this.expiryTime = 0;
        } else {
            this.expiryTime = t.valueOf();
        }
    }

    get _totalGB() {
        return NumberFormatter.toFixed(this.totalGB / SizeFormatter.ONE_GB, 2);
    }

    set _totalGB(gb) {
        this.totalGB = NumberFormatter.toFixed(gb * SizeFormatter.ONE_GB, 0);
    }
};

Inbound.VmessSettings = class extends Inbound.Settings {
    constructor(protocol,
        vmesses = [new Inbound.VmessSettings.VMESS()]) {
        super(protocol);
        this.vmesses = vmesses;
    }

    indexOfVmessById(id) {
        return this.vmesses.findIndex(VMESS => VMESS.id === id);
    }

    addVmess(VMESS) {
        if (this.indexOfVmessById(VMESS.id) >= 0) {
            return false;
        }
        this.vmesses.push(VMESS);
    }

    delVmess(VMESS) {
        const i = this.indexOfVmessById(VMESS.id);
        if (i >= 0) {
            this.vmesses.splice(i, 1);
        }
    }

    static fromJson(json = {}) {
        return new Inbound.VmessSettings(
            Protocols.VMESS,
            (json.clients || []).map(client => Inbound.VmessSettings.VMESS.fromJson(client)),
        );
    }

    toJson() {
        return {
            clients: Inbound.VmessSettings.toJsonArray(this.vmesses),
        };
    }
};

Inbound.VmessSettings.VMESS = class extends Inbound.ClientBase {
    constructor(
        id = RandomUtil.randomUUID(),
        security = USERS_SECURITY.AUTO,
        email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at,
    ) {
        super(email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at);
        this.id = id;
        this.security = security;
    }

    static fromJson(json = {}) {
        return new Inbound.VmessSettings.VMESS(
            json.id,
            json.security,
            ...Inbound.ClientBase.commonArgsFromJson(json),
        );
    }

    toJson() {
        return {
            id: this.id,
            security: this.security,
            ...this._clientBaseToJson(),
        };
    }
};

Inbound.VLESSSettings = class extends Inbound.Settings {
    constructor(
        protocol,
        vlesses = [new Inbound.VLESSSettings.VLESS()],
        decryption = "none",
        encryption = "none",
        fallbacks = [],
        testseed = [],
    ) {
        super(protocol);
        this.vlesses = vlesses;
        this.decryption = decryption;
        this.encryption = encryption;
        this.fallbacks = fallbacks;
        this.testseed = testseed;
    }

    addFallback() {
        this.fallbacks.push(new Inbound.VLESSSettings.Fallback());
    }

    delFallback(index) {
        this.fallbacks.splice(index, 1);
    }

    // Empty array means "use server defaults" (won't be sent).
    // Anything else must be exactly 4 positive integers.
    static isValidTestseed(arr) {
        if (!Array.isArray(arr) || arr.length === 0) return true;
        if (arr.length !== 4) return false;
        return arr.every(v => Number.isInteger(v) && v > 0);
    }

    static fromJson(json = {}) {
        // Preserve a saved testseed only if it's a valid 4-positive-int array; otherwise leave empty
        // so toJson omits it and the form falls back to placeholder defaults.
        const saved = json.testseed;
        const testseed = (Array.isArray(saved)
            && saved.length === 4
            && saved.every(v => Number.isInteger(v) && v > 0))
            ? saved
            : [];

        const obj = new Inbound.VLESSSettings(
            Protocols.VLESS,
            (json.clients || []).map(client => Inbound.VLESSSettings.VLESS.fromJson(client)),
            json.decryption,
            json.encryption,
            Inbound.VLESSSettings.Fallback.fromJson(json.fallbacks || []),
            testseed,
        );
        return obj;
    }


    toJson() {
        const json = {
            clients: Inbound.VLESSSettings.toJsonArray(this.vlesses),
        };

        if (this.decryption) {
            json.decryption = this.decryption;
        }

        if (this.encryption) {
            json.encryption = this.encryption;
        }

        if (this.fallbacks && this.fallbacks.length > 0) {
            json.fallbacks = Inbound.VLESSSettings.toJsonArray(this.fallbacks);
        }

        // testseed is only meaningful for the exact xtls-rprx-vision flow, and only when
        // the user supplied a complete 4-positive-int array. Otherwise omit and let the
        // backend fall back to its safe defaults.
        const hasVisionFlow = this.vlesses && this.vlesses.some(v => v.flow === TLS_FLOW_CONTROL.VISION);
        if (hasVisionFlow
            && Array.isArray(this.testseed)
            && this.testseed.length === 4
            && this.testseed.every(v => Number.isInteger(v) && v > 0)) {
            json.testseed = this.testseed;
        }

        return json;
    }
};

Inbound.VLESSSettings.VLESS = class extends Inbound.ClientBase {
    constructor(
        id = RandomUtil.randomUUID(),
        flow = '',
        reverseTag = '',
        reverseSniffing = new Sniffing(),
        email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at,
    ) {
        super(email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at);
        this.id = id;
        this.flow = flow;
        this.reverseTag = reverseTag;
        this.reverseSniffing = reverseSniffing;
    }

    static fromJson(json = {}) {
        return new Inbound.VLESSSettings.VLESS(
            json.id,
            json.flow,
            json.reverse?.tag ?? '',
            Sniffing.fromJson(json.reverse?.sniffing || {}),
            ...Inbound.ClientBase.commonArgsFromJson(json),
        );
    }

    toJson() {
        const json = {
            id: this.id,
            flow: this.flow,
            ...this._clientBaseToJson(),
        };
        if (this.reverseTag) {
            json.reverse = {
                tag: this.reverseTag,
            };
        }
        return json;
    }
};

Inbound.VLESSSettings.Fallback = class extends XrayCommonClass {
    constructor(name = "", alpn = '', path = '', dest = '', xver = 0) {
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

    static fromJson(json = []) {
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
        trojans = [new Inbound.TrojanSettings.Trojan()],
        fallbacks = [],) {
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

    static fromJson(json = {}) {
        return new Inbound.TrojanSettings(
            Protocols.TROJAN,
            (json.clients || []).map(client => Inbound.TrojanSettings.Trojan.fromJson(client)),
            Inbound.TrojanSettings.Fallback.fromJson(json.fallbacks),);
    }

    toJson() {
        return {
            clients: Inbound.TrojanSettings.toJsonArray(this.trojans),
            fallbacks: Inbound.TrojanSettings.toJsonArray(this.fallbacks)
        };
    }
};

Inbound.TrojanSettings.Trojan = class extends Inbound.ClientBase {
    constructor(
        password = RandomUtil.randomSeq(10),
        email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at,
    ) {
        super(email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at);
        this.password = password;
    }

    toJson() {
        return {
            password: this.password,
            ...this._clientBaseToJson(),
        };
    }

    static fromJson(json = {}) {
        return new Inbound.TrojanSettings.Trojan(
            json.password,
            ...Inbound.ClientBase.commonArgsFromJson(json),
        );
    }
};

Inbound.TrojanSettings.Fallback = class extends XrayCommonClass {
    constructor(name = "", alpn = '', path = '', dest = '', xver = 0) {
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

    static fromJson(json = []) {
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
        method = SSMethods.BLAKE3_AES_256_GCM,
        password = RandomUtil.randomShadowsocksPassword(),
        network = 'tcp,udp',
        shadowsockses = [new Inbound.ShadowsocksSettings.Shadowsocks()],
        ivCheck = false,
    ) {
        super(protocol);
        this.method = method;
        this.password = password;
        this.network = network;
        this.shadowsockses = shadowsockses;
        this.ivCheck = ivCheck;
    }

    static fromJson(json = {}) {
        return new Inbound.ShadowsocksSettings(
            Protocols.SHADOWSOCKS,
            json.method,
            json.password,
            json.network,
            (json.clients || []).map(client => Inbound.ShadowsocksSettings.Shadowsocks.fromJson(client)),
            json.ivCheck,
        );
    }

    toJson() {
        return {
            method: this.method,
            password: this.password,
            network: this.network,
            clients: Inbound.ShadowsocksSettings.toJsonArray(this.shadowsockses),
            ivCheck: this.ivCheck,
        };
    }
};

Inbound.ShadowsocksSettings.Shadowsocks = class extends Inbound.ClientBase {
    constructor(
        method = '',
        password = RandomUtil.randomShadowsocksPassword(),
        email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at,
    ) {
        super(email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at);
        this.method = method;
        this.password = password;
    }

    toJson() {
        return {
            method: this.method,
            password: this.password,
            ...this._clientBaseToJson(),
        };
    }

    static fromJson(json = {}) {
        return new Inbound.ShadowsocksSettings.Shadowsocks(
            json.method,
            json.password,
            ...Inbound.ClientBase.commonArgsFromJson(json),
        );
    }
};

Inbound.HysteriaSettings = class extends Inbound.Settings {
    constructor(protocol, version = 2, hysterias = [new Inbound.HysteriaSettings.Hysteria()]) {
        super(protocol);
        this.version = version;
        this.hysterias = hysterias;
    }

    static fromJson(json = {}) {
        return new Inbound.HysteriaSettings(
            Protocols.HYSTERIA,
            json.version ?? 2,
            (json.clients || []).map(client => Inbound.HysteriaSettings.Hysteria.fromJson(client)),
        );
    }

    toJson() {
        return {
            version: this.version,
            clients: Inbound.HysteriaSettings.toJsonArray(this.hysterias),
        };
    }
};

Inbound.HysteriaSettings.Hysteria = class extends Inbound.ClientBase {
    constructor(
        auth = RandomUtil.randomSeq(10),
        email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at,
    ) {
        super(email, limitIp, totalGB, expiryTime, enable, tgId, subId, comment, reset, created_at, updated_at);
        this.auth = auth;
    }

    toJson() {
        return {
            auth: this.auth,
            ...this._clientBaseToJson(),
        };
    }

    static fromJson(json = {}) {
        return new Inbound.HysteriaSettings.Hysteria(
            json.auth,
            ...Inbound.ClientBase.commonArgsFromJson(json),
        );
    }
};

Inbound.TunnelSettings = class extends Inbound.Settings {
    constructor(
        protocol,
        address,
        port,
        portMap = [],
        network = 'tcp,udp',
        followRedirect = false
    ) {
        super(protocol);
        this.address = address;
        this.port = port;
        this.portMap = portMap;
        this.network = network;
        this.followRedirect = followRedirect;
    }

    static fromJson(json = {}) {
        return new Inbound.TunnelSettings(
            Protocols.TUNNEL,
            json.address,
            json.port,
            XrayCommonClass.toHeaders(json.portMap),
            json.network,
            json.followRedirect,
        );
    }

    toJson() {
        return {
            address: this.address,
            port: this.port,
            portMap: XrayCommonClass.toV2Headers(this.portMap, false),
            network: this.network,
            followRedirect: this.followRedirect,
        };
    }
};

Inbound.MixedSettings = class extends Inbound.Settings {
    constructor(protocol, auth = 'password', accounts = [new Inbound.MixedSettings.SocksAccount()], udp = false, ip = '127.0.0.1') {
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

    static fromJson(json = {}) {
        let accounts;
        if (json.auth === 'password') {
            accounts = json.accounts.map(
                account => Inbound.MixedSettings.SocksAccount.fromJson(account)
            )
        }
        return new Inbound.MixedSettings(
            Protocols.MIXED,
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
Inbound.MixedSettings.SocksAccount = class extends XrayCommonClass {
    constructor(user = RandomUtil.randomSeq(10), pass = RandomUtil.randomSeq(10)) {
        super();
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json = {}) {
        return new Inbound.MixedSettings.SocksAccount(json.user, json.pass);
    }
};

Inbound.HttpSettings = class extends Inbound.Settings {
    constructor(
        protocol,
        accounts = [new Inbound.HttpSettings.HttpAccount()],
        allowTransparent = false,
    ) {
        super(protocol);
        this.accounts = accounts;
        this.allowTransparent = allowTransparent;
    }

    addAccount(account) {
        this.accounts.push(account);
    }

    delAccount(index) {
        this.accounts.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new Inbound.HttpSettings(
            Protocols.HTTP,
            json.accounts.map(account => Inbound.HttpSettings.HttpAccount.fromJson(account)),
            json.allowTransparent,
        );
    }

    toJson() {
        return {
            accounts: Inbound.HttpSettings.toJsonArray(this.accounts),
            allowTransparent: this.allowTransparent,
        };
    }
};

Inbound.HttpSettings.HttpAccount = class extends XrayCommonClass {
    constructor(user = RandomUtil.randomSeq(10), pass = RandomUtil.randomSeq(10)) {
        super();
        this.user = user;
        this.pass = pass;
    }

    static fromJson(json = {}) {
        return new Inbound.HttpSettings.HttpAccount(json.user, json.pass);
    }
};

Inbound.WireguardSettings = class extends XrayCommonClass {
    constructor(
        protocol,
        mtu = 1420,
        secretKey = Wireguard.generateKeypair().privateKey,
        peers = [new Inbound.WireguardSettings.Peer()],
        noKernelTun = false
    ) {
        super(protocol);
        this.mtu = mtu;
        this.secretKey = secretKey;
        this.pubKey = secretKey.length > 0 ? Wireguard.generateKeypair(secretKey).publicKey : '';
        this.peers = peers;
        this.noKernelTun = noKernelTun;
    }

    addPeer() {
        this.peers.push(new Inbound.WireguardSettings.Peer(null, null, '', ['10.0.0.' + (this.peers.length + 2)]));
    }

    delPeer(index) {
        this.peers.splice(index, 1);
    }

    static fromJson(json = {}) {
        return new Inbound.WireguardSettings(
            Protocols.WIREGUARD,
            json.mtu,
            json.secretKey,
            json.peers.map(peer => Inbound.WireguardSettings.Peer.fromJson(peer)),
            json.noKernelTun,
        );
    }

    toJson() {
        return {
            mtu: this.mtu ?? undefined,
            secretKey: this.secretKey,
            peers: Inbound.WireguardSettings.Peer.toJsonArray(this.peers),
            noKernelTun: this.noKernelTun,
        };
    }
};

Inbound.WireguardSettings.Peer = class extends XrayCommonClass {
    constructor(privateKey, publicKey, psk = '', allowedIPs = ['10.0.0.2/32'], keepAlive = 0) {
        super();
        this.privateKey = privateKey
        this.publicKey = publicKey;
        if (!this.publicKey) {
            [this.publicKey, this.privateKey] = Object.values(Wireguard.generateKeypair())
        }
        this.psk = psk;
        allowedIPs.forEach((a, index) => {
            if (a.length > 0 && !a.includes('/')) allowedIPs[index] += '/32';
        })
        this.allowedIPs = allowedIPs;
        this.keepAlive = keepAlive;
    }

    static fromJson(json = {}) {
        return new Inbound.WireguardSettings.Peer(
            json.privateKey,
            json.publicKey,
            json.preSharedKey,
            json.allowedIPs,
            json.keepAlive
        );
    }

    toJson() {
        this.allowedIPs.forEach((a, index) => {
            if (a.length > 0 && !a.includes('/')) this.allowedIPs[index] += '/32';
        });
        return {
            privateKey: this.privateKey,
            publicKey: this.publicKey,
            preSharedKey: this.psk.length > 0 ? this.psk : undefined,
            allowedIPs: this.allowedIPs,
            keepAlive: this.keepAlive ?? undefined,
        };
    }
};

Inbound.TunSettings = class extends Inbound.Settings {
    constructor(
        protocol,
        name = 'xray0',
        mtu = [1500, 1280],
        gateway = [],
        dns = [],
        userLevel = 0,
        autoSystemRoutingTable = [],
        autoOutboundsInterface = 'auto'
    ) {
        super(protocol);
        this.name = name;
        this.mtu = this._normalizeMtu(mtu);
        this.gateway = Array.isArray(gateway) ? gateway : [];
        this.dns = Array.isArray(dns) ? dns : [];
        this.userLevel = userLevel;
        this.autoSystemRoutingTable = Array.isArray(autoSystemRoutingTable) ? autoSystemRoutingTable : [];
        this.autoOutboundsInterface = autoOutboundsInterface;
    }

    _normalizeMtu(mtu) {
        if (!Array.isArray(mtu)) {
            const single = Number(mtu) || 1500;
            return [single, single];
        }
        if (mtu.length === 0) {
            return [1500, 1280];
        }
        if (mtu.length === 1) {
            const single = Number(mtu[0]) || 1500;
            return [single, single];
        }
        return [Number(mtu[0]) || 1500, Number(mtu[1]) || 1280];
    }

    static fromJson(json = {}) {
        return new Inbound.TunSettings(
            Protocols.TUN,
            json.name ?? 'xray0',
            json.mtu ?? json.MTU ?? [1500, 1280],
            json.gateway ?? json.Gateway ?? [],
            json.dns ?? json.DNS ?? [],
            json.userLevel ?? 0,
            json.autoSystemRoutingTable ?? [],
            Object.prototype.hasOwnProperty.call(json, 'autoOutboundsInterface') ? json.autoOutboundsInterface : 'auto'
        );
    }

    toJson() {
        return {
            name: this.name || 'xray0',
            mtu: this._normalizeMtu(this.mtu),
            gateway: this.gateway,
            dns: this.dns,
            userLevel: this.userLevel || 0,
            autoSystemRoutingTable: this.autoSystemRoutingTable,
            autoOutboundsInterface: this.autoOutboundsInterface,
        };
    }
};
