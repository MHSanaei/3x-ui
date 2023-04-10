class HttpUtil {
    static _handleMsg(msg) {
        if (!(msg instanceof Msg)) {
            return;
        }
        if (msg.msg === "") {
            return;
        }
        if (msg.success) {
            Vue.prototype.$message.success(msg.msg);
        } else {
            Vue.prototype.$message.error(msg.msg);
        }
    }

    static _respToMsg(resp) {
        const data = resp.data;
        if (data == null) {
            return new Msg(true);
        } else if (typeof data === 'object') {
            if (data.hasOwnProperty('success')) {
                return new Msg(data.success, data.msg, data.obj);
            } else {
                return data;
            }
        } else {
            return new Msg(false, 'unknown data:', data);
        }
    }

    static async get(url, data, options) {
        let msg;
        try {
            const resp = await axios.get(url, data, options);
            msg = this._respToMsg(resp);
        } catch (e) {
            msg = new Msg(false, e.toString());
        }
        this._handleMsg(msg);
        return msg;
    }

    static async post(url, data, options) {
        let msg;
        try {
            const resp = await axios.post(url, data, options);
            msg = this._respToMsg(resp);
        } catch (e) {
            msg = new Msg(false, e.toString());
        }
        this._handleMsg(msg);
        return msg;
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
}

class PromiseUtil {

    static async sleep(timeout) {
        await new Promise(resolve => {
            setTimeout(resolve, timeout)
        });
    }

}

const seq = [
    'a', 'b', 'c', 'd', 'e', 'f', 'g',
    'h', 'i', 'j', 'k', 'l', 'm', 'n',
    'o', 'p', 'q', 'r', 's', 't',
    'u', 'v', 'w', 'x', 'y', 'z',
    '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
    'A', 'B', 'C', 'D', 'E', 'F', 'G',
    'H', 'I', 'J', 'K', 'L', 'M', 'N',
    'O', 'P', 'Q', 'R', 'S', 'T',
    'U', 'V', 'W', 'X', 'Y', 'Z'
];

const shortIdSeq = [
    'a', 'b', 'c', 'd', 'e', 'f',
    '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
];

const x25519Map = new Map(
    [
        ['EH2FWe-Ij_FFAa2u9__-aiErLvVIneP601GOCdlyPWw', "goY3OtfaA4UYbiz7Hn0NysI5QJrK0VT_Chg6RLgUPQU"],
        ['cKI_6DoMSP1IepeWWXrG3G9nkehl94KYBhagU50g2U0', "VigpKFbSLnHLzBWobZaS1IBmw--giJ51w92y723ajnU"],
        ['qM2SNyK3NyHB6deWpEP3ITyCGKQFRTna_mlKP0w1QH0', "HYyIGuyNFslmcnNT7mrDdmuXwn4cm7smE_FZbYguKHQ"],
        ['qCWg5GMEDFd3n1nxDswlIpOHoPUXMLuMOIiLUVzubkI', "rJFC3dUjJxMnVZiUGzmf_LFsJUwFWY-CU5RQgFOHCWM"],
        ['4NOBxDrEsOhNI3Y3EnVIy_TN-uyBoAjQw6QM0YsOi0s', "CbcY9qc4YuMDJDyyL0OITlU824TBg1O84ClPy27e2RM"],
        ['eBvFb0M4HpSOwWjtXV8zliiEs_hg56zX4a2LpuuqpEI', "CjulQ2qVIky7ImIfysgQhNX7s_drGLheCGSkVHcLZhc"],
        ['yEpOzQV04NNcycWVeWtRNTzv5TS-ynTuKRacZCH-6U8', "O9RSr5gSdok2K_tobQnf_scyKVqnCx6C4Jrl7_rCZEQ"],
        ['CNt6TAUVCwqM6xIBHyni0K3Zqbn2htKQLvLb6XDgh0s', "d9cGLVBrDFS02L2OvkqyqwFZ1Ux3AHs28ehl4Rwiyl0"],
        ['EInKw-6Wr0rAHXlxxDuZU5mByIzcD3Z-_iWPzXlUL1k', "LlYD2nNVAvyjNvjZGZh4R8PkMIwkc6EycPTvR2LE0nQ"],
        ['GKIKo7rcXVyle-EUHtGIDtYnDsI6osQmOUl3DTJRAGc', "VcqHivYGGoBkcxOI6cSSjQmneltstkb2OhvO53dyhEM"],
        ['-FVDzv68IC17fJVlNDlhrrgX44WeBfbhwjWpCQVXGHE', "PGG2EYOvsFt2lAQTD7lqHeRxz2KxvllEDKcUrtizPBU"],
        ['0H3OJEYEu6XW7woqy7cKh2vzg6YHkbF_xSDTHKyrsn4', "mzevpYbS8kXengBY5p7tt56QE4tS3lwlwRemmkcQeyc"],
        ['8F8XywN6ci44ES6em2Z0fYYxyptB9uaXY9Hc1WSSPE4', "qCZUdWQZ2H33vWXnOkG8NpxBeq3qn5QWXlfCOWBNkkc"],
        ['IN0dqfkC10dj-ifRHrg2PmmOrzYs697ajGMwcLbu-1g', "2UW_EO3r7uczPGUUlpJBnMDpDmWUHE2yDzCmXS4sckE"],
        ['uIcmks5rAhvBe4dRaJOdeSqgxLGGMZhsGk4J4PEKL2s', "F9WJV_74IZp0Ide4hWjiJXk9FRtBUBkUr3mzU-q1lzk"],
    ]
);

class RandomUtil {

    static randomIntRange(min, max) {
        return parseInt(Math.random() * (max - min) + min, 10);
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

    static randomShortIdSeq(count) {
        let str = '';
        for (let i = 0; i < count; ++i) {
            str += shortIdSeq[this.randomInt(16)];
        }
        return str;
    }

    static randomLowerAndNum(count) {
        let str = '';
        for (let i = 0; i < count; ++i) {
            str += seq[this.randomInt(36)];
        }
        return str;
    }

    static randomMTSecret() {
        let str = '';
        for (let i = 0; i < 32; ++i) {
            let index = this.randomInt(16);
            if (index <= 9) {
                str += index;
            } else {
                str += seq[index - 10];
            }
        }
        return str;
    }

    static randomUUID() {
        let d = new Date().getTime();
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            let r = (d + Math.random() * 16) % 16 | 0;
            d = Math.floor(d / 16);
            return (c === 'x' ? r : (r & 0x7 | 0x8)).toString(16);
        });
    }

    static randowShortId() {
        let str = '';
        str += this.randomShortIdSeq(8)
        return str;
    }

    static randomX25519PrivateKey() {
        let num = x25519Map.size;
        let index = this.randomInt(num);
        let cntr = 0;
        for (let key of x25519Map.keys()) {
            if (cntr++ === index) {
                return key;
            }
        }
    }

    static randomX25519PublicKey(key) {
        return x25519Map.get(key)
    }
    static randomText() {
        var chars = 'abcdefghijklmnopqrstuvwxyz1234567890';
        var string = '';
        var len = 6 + Math.floor(Math.random() * 5)
        for(var ii=0; ii<len; ii++){
            string += chars[Math.floor(Math.random() * chars.length)];
        }
        return string;
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
            return obj.toString().toLowerCase().indexOf(key.toLowerCase()) >= 0;
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
