class Msg {
    constructor(success = false, msg = "", obj = null) {
        this.success = success;
        this.msg = msg;
        this.obj = obj;
    }
}

class HttpUtil {
    static _handleMsg(msg) {
        if (!(msg instanceof Msg) || msg.msg === "") {
            return;
        }
        const messageType = msg.success ? 'success' : 'error';
        Vue.prototype.$message[messageType](msg.msg);
    }

    static _respToMsg(resp) {
        if (!resp || !resp.data) {
            return new Msg(false, 'No response data');
        }
        const { data } = resp;
        if (data == null) {
            return new Msg(true);
        }
        if (typeof data === 'object' && 'success' in data) {
            return new Msg(data.success, data.msg, data.obj);
        }
        return typeof data === 'object' ? data : new Msg(false, 'unknown data:', data);
    }

    static async get(url, params, options = {}) {
        try {
            const resp = await axios.get(url, { params, ...options });
            const msg = this._respToMsg(resp);
            this._handleMsg(msg);
            return msg;
        } catch (error) {
            console.error('GET request failed:', error);
            const errorMsg = new Msg(false, error.response?.data?.message || error.message || 'Request failed');
            this._handleMsg(errorMsg);
            return errorMsg;
        }
    }

    static async post(url, data, options = {}) {
        try {
            const resp = await axios.post(url, data, options);
            const msg = this._respToMsg(resp);
            this._handleMsg(msg);
            return msg;
        } catch (error) {
            console.error('POST request failed:', error);
            const errorMsg = new Msg(false, error.response?.data?.message || error.message || 'Request failed');
            this._handleMsg(errorMsg);
            return errorMsg;
        }
    }

    static async postWithModal(url, data, modal) {
        if (modal) {
            modal.loading(true);
        }
        const msg = await this.post(url, data);
        if (modal) {
            modal.loading(false);
            if (msg instanceof Msg && msg.success) {
                modal.close();
            }
        }
        return msg;
    }

    static async jsonPost(url, data) {
        let msg;
        try {
            const requestOptions = {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(data),
            };
            const resp = await fetch(basePath + url.replace(/^\/+|\/+$/g, ''), requestOptions);
            const response = await resp.json();

            msg = this._respToMsg({data : response});
        } catch (e) {
            msg = new Msg(false, e.toString());
        }
        this._handleMsg(msg);
        return msg;
    }

    static async postWithModalJson(url, data, modal) {
        if (modal) {
            modal.loading(true);
        }
        const msg = await this.jsonPost(url, data);
        if (modal) {
            modal.loading(false);
            if (msg instanceof Msg && msg.success) {
                modal.close();
            }
        }
        return msg;
    }
}

class PromiseUtil {
    static async sleep(timeout) {
        await new Promise(resolve => {
            setTimeout(resolve, timeout)
        });
    }
}

const seq = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('');

class RandomUtil {
    static randomIntRange(min, max) {
        return Math.floor(Math.random() * (max - min) + min);
    }

    static randomInt(n) {
        return this.randomIntRange(0, n);
    }

    static randomSeq(count) {
        let str = '';
        for (let i = 0; i < count; ++i) {
            str += seq[this.randomInt(62)];
        }
        return str;
    }

    static randomShortIds() {
        const lengths = [2, 4, 6, 8, 10, 12, 14, 16];
        for (let i = lengths.length - 1; i > 0; i--) {
            const j = Math.floor(Math.random() * (i + 1));
            [lengths[i], lengths[j]] = [lengths[j], lengths[i]];
        }

        let shortIds = [];
        for (let length of lengths) {
            let shortId = '';
            for (let i = 0; i < length; i++) {
                shortId += seq[this.randomInt(16)];
            }
            shortIds.push(shortId);
        }
        return shortIds.join(',');
    }

    static randomLowerAndNum(len) {
        let str = '';
        for (let i = 0; i < len; ++i) {
            str += seq[this.randomInt(36)];
        }
        return str;
    }

    static randomUUID() {
        const template = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx';
        return template.replace(/[xy]/g, function (c) {
            const randomValues = new Uint8Array(1);
            crypto.getRandomValues(randomValues);
            let randomValue = randomValues[0] % 16;
            let calculatedValue = (c === 'x') ? randomValue : (randomValue & 0x3 | 0x8);
            return calculatedValue.toString(16);
        });
    }

    static randomShadowsocksPassword() {
        let array = new Uint8Array(32);
        window.crypto.getRandomValues(array);
        return btoa(String.fromCharCode.apply(null, array));
    }
}

class ObjectUtil {
    static getPropIgnoreCase(obj, prop) {
        for (const name in obj) {
            if (!obj.hasOwnProperty(name)) {
                continue;
            }
            if (name.toLowerCase() === prop.toLowerCase()) {
                return obj[name];
            }
        }
        return undefined;
    }

    static deepSearch(obj, key) {
        if (obj instanceof Array) {
            for (let i = 0; i < obj.length; ++i) {
                if (this.deepSearch(obj[i], key)) {
                    return true;
                }
            }
        } else if (obj instanceof Object) {
            for (let name in obj) {
                if (!obj.hasOwnProperty(name)) {
                    continue;
                }
                if (this.deepSearch(obj[name], key)) {
                    return true;
                }
            }
        } else {
            return this.isEmpty(obj) ? false : obj.toString().toLowerCase().indexOf(key.toLowerCase()) >= 0;
        }
        return false;
    }

    static isEmpty(obj) {
        return obj === null || obj === undefined || obj === '';
    }

    static isArrEmpty(arr) {
        return !this.isEmpty(arr) && arr.length === 0;
    }

    static copyArr(dest, src) {
        dest.splice(0);
        for (const item of src) {
            dest.push(item);
        }
    }

    static clone(obj) {
        let newObj;
        if (obj instanceof Array) {
            newObj = [];
            this.copyArr(newObj, obj);
        } else if (obj instanceof Object) {
            newObj = {};
            for (const key of Object.keys(obj)) {
                newObj[key] = obj[key];
            }
        } else {
            newObj = obj;
        }
        return newObj;
    }

    static deepClone(obj) {
        let newObj;
        if (obj instanceof Array) {
            newObj = [];
            for (const item of obj) {
                newObj.push(this.deepClone(item));
            }
        } else if (obj instanceof Object) {
            newObj = {};
            for (const key of Object.keys(obj)) {
                newObj[key] = this.deepClone(obj[key]);
            }
        } else {
            newObj = obj;
        }
        return newObj;
    }

    static cloneProps(dest, src, ...ignoreProps) {
        if (dest == null || src == null) {
            return;
        }
        const ignoreEmpty = this.isArrEmpty(ignoreProps);
        for (const key of Object.keys(src)) {
            if (!src.hasOwnProperty(key)) {
                continue;
            } else if (!dest.hasOwnProperty(key)) {
                continue;
            } else if (src[key] === undefined) {
                continue;
            }
            if (ignoreEmpty) {
                dest[key] = src[key];
            } else {
                let ignore = false;
                for (let i = 0; i < ignoreProps.length; ++i) {
                    if (key === ignoreProps[i]) {
                        ignore = true;
                        break;
                    }
                }
                if (!ignore) {
                    dest[key] = src[key];
                }
            }
        }
    }

    static delProps(obj, ...props) {
        for (const prop of props) {
            if (prop in obj) {
                delete obj[prop];
            }
        }
    }

    static execute(func, ...args) {
        if (!this.isEmpty(func) && typeof func === 'function') {
            func(...args);
        }
    }

    static orDefault(obj, defaultValue) {
        if (obj == null) {
            return defaultValue;
        }
        return obj;
    }

    static equals(a, b) {
        for (const key in a) {
            if (!a.hasOwnProperty(key)) {
                continue;
            }
            if (!b.hasOwnProperty(key)) {
                return false;
            } else if (a[key] !== b[key]) {
                return false;
            }
        }
        return true;
    }
}

class Wireguard {
    static gf(init) {
        var r = new Float64Array(16);
        if (init) {
            for (var i = 0; i < init.length; ++i)
                r[i] = init[i];
        }
        return r;
    }

    static pack(o, n) {
        var b, m = this.gf(), t = this.gf();
        for (var i = 0; i < 16; ++i)
            t[i] = n[i];
        this.carry(t);
        this.carry(t);
        this.carry(t);
        for (var j = 0; j < 2; ++j) {
            m[0] = t[0] - 0xffed;
            for (var i = 1; i < 15; ++i) {
                m[i] = t[i] - 0xffff - ((m[i - 1] >> 16) & 1);
                m[i - 1] &= 0xffff;
            }
            m[15] = t[15] - 0x7fff - ((m[14] >> 16) & 1);
            b = (m[15] >> 16) & 1;
            m[14] &= 0xffff;
            this.cswap(t, m, 1 - b);
        }
        for (var i = 0; i < 16; ++i) {
            o[2 * i] = t[i] & 0xff;
            o[2 * i + 1] = t[i] >> 8;
        }
    }

    static carry(o) {
        var c;
        for (var i = 0; i < 16; ++i) {
            o[(i + 1) % 16] += (i < 15 ? 1 : 38) * Math.floor(o[i] / 65536);
            o[i] &= 0xffff;
        }
    }

    static cswap(p, q, b) {
        var t, c = ~(b - 1);
        for (var i = 0; i < 16; ++i) {
            t = c & (p[i] ^ q[i]);
            p[i] ^= t;
            q[i] ^= t;
        }
    }

    static add(o, a, b) {
        for (var i = 0; i < 16; ++i)
            o[i] = (a[i] + b[i]) | 0;
    }

    static subtract(o, a, b) {
        for (var i = 0; i < 16; ++i)
            o[i] = (a[i] - b[i]) | 0;
    }

    static multmod(o, a, b) {
        var t = new Float64Array(31);
        for (var i = 0; i < 16; ++i) {
            for (var j = 0; j < 16; ++j)
                t[i + j] += a[i] * b[j];
        }
        for (var i = 0; i < 15; ++i)
            t[i] += 38 * t[i + 16];
        for (var i = 0; i < 16; ++i)
            o[i] = t[i];
        this.carry(o);
        this.carry(o);
    }

    static invert(o, i) {
        var c = this.gf();
        for (var a = 0; a < 16; ++a)
            c[a] = i[a];
        for (var a = 253; a >= 0; --a) {
            this.multmod(c, c, c);
            if (a !== 2 && a !== 4)
                this.multmod(c, c, i);
        }
        for (var a = 0; a < 16; ++a)
            o[a] = c[a];
    }

    static clamp(z) {
        z[31] = (z[31] & 127) | 64;
        z[0] &= 248;
    }

    static generatePublicKey(privateKey) {
        var r, z = new Uint8Array(32);
        var a = this.gf([1]),
            b = this.gf([9]),
            c = this.gf(),
            d = this.gf([1]),
            e = this.gf(),
            f = this.gf(),
            _121665 = this.gf([0xdb41, 1]),
            _9 = this.gf([9]);
        for (var i = 0; i < 32; ++i)
            z[i] = privateKey[i];
        this.clamp(z);
        for (var i = 254; i >= 0; --i) {
            r = (z[i >>> 3] >>> (i & 7)) & 1;
            this.cswap(a, b, r);
            this.cswap(c, d, r);
            this.add(e, a, c);
            this.subtract(a, a, c);
            this.add(c, b, d);
            this.subtract(b, b, d);
            this.multmod(d, e, e);
            this.multmod(f, a, a);
            this.multmod(a, c, a);
            this.multmod(c, b, e);
            this.add(e, a, c);
            this.subtract(a, a, c);
            this.multmod(b, a, a);
            this.subtract(c, d, f);
            this.multmod(a, c, _121665);
            this.add(a, a, d);
            this.multmod(c, c, a);
            this.multmod(a, d, f);
            this.multmod(d, b, _9);
            this.multmod(b, e, e);
            this.cswap(a, b, r);
            this.cswap(c, d, r);
        }
        this.invert(c, c);
        this.multmod(a, a, c);
        this.pack(z, a);
        return z;
    }

    static generatePresharedKey() {
        var privateKey = new Uint8Array(32);
        window.crypto.getRandomValues(privateKey);
        return privateKey;
    }

    static generatePrivateKey() {
        var privateKey = this.generatePresharedKey();
        this.clamp(privateKey);
        return privateKey;
    }

    static encodeBase64(dest, src) {
        var input = Uint8Array.from([(src[0] >> 2) & 63, ((src[0] << 4) | (src[1] >> 4)) & 63, ((src[1] << 2) | (src[2] >> 6)) & 63, src[2] & 63]);
        for (var i = 0; i < 4; ++i)
            dest[i] = input[i] + 65 +
                (((25 - input[i]) >> 8) & 6) -
                (((51 - input[i]) >> 8) & 75) -
                (((61 - input[i]) >> 8) & 15) +
                (((62 - input[i]) >> 8) & 3);
    }

    static keyToBase64(key) {
        var i, base64 = new Uint8Array(44);
        for (i = 0; i < 32 / 3; ++i)
            this.encodeBase64(base64.subarray(i * 4), key.subarray(i * 3));
        this.encodeBase64(base64.subarray(i * 4), Uint8Array.from([key[i * 3 + 0], key[i * 3 + 1], 0]));
        base64[43] = 61;
        return String.fromCharCode.apply(null, base64);
    }

    static keyFromBase64(encoded) {
        const binaryStr = atob(encoded);
        const bytes = new Uint8Array(binaryStr.length);
        for (let i = 0; i < binaryStr.length; i++) {
            bytes[i] = binaryStr.charCodeAt(i);
        }
        return bytes;
    }

    static generateKeypair(secretKey = '') {
        var privateKey = secretKey.length > 0 ? this.keyFromBase64(secretKey) : this.generatePrivateKey();
        var publicKey = this.generatePublicKey(privateKey);
        return {
            publicKey: this.keyToBase64(publicKey),
            privateKey: secretKey.length > 0 ? secretKey : this.keyToBase64(privateKey)
        };
    }
}