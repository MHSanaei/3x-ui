import axios from 'axios';
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from 'axios';
import i18next from 'i18next';
import { getMessage } from './messageBus';

type RespEnvelope = { success?: unknown; msg?: unknown; obj?: unknown };

export class Msg<T = unknown> {
  success: boolean;
  msg: string;
  obj: T | null;

  constructor(success: boolean = false, msg: string = '', obj: T | null = null) {
    this.success = success;
    this.msg = msg;
    this.obj = obj;
  }
}

export interface HttpOptions extends AxiosRequestConfig {
  silent?: boolean;
}

export interface HttpModal {
  loading: (state: boolean) => void;
  close: () => void;
}

export class HttpUtil {
  static _handleMsg(msg: unknown): void {
    if (!(msg instanceof Msg) || msg.msg === '') {
      return;
    }
    const messageType = msg.success ? 'success' : 'error';
    getMessage()[messageType](msg.msg);
    if (
      msg.success &&
      msg.obj &&
      typeof msg.obj === 'object' &&
      (msg.obj as { nodePending?: unknown }).nodePending === true
    ) {
      getMessage().warning(i18next.t('pages.inbounds.toasts.savedNodeOfflineWillSync'));
    }
  }

  static _respToMsg(resp: AxiosResponse | undefined): Msg {
    if (!resp || !resp.data) {
      return new Msg(false, 'No response data');
    }
    const { data } = resp;
    if (data == null) {
      return new Msg(true);
    }
    if (typeof data === 'object' && 'success' in (data as object)) {
      const d = data as RespEnvelope;
      return new Msg(Boolean(d.success), typeof d.msg === 'string' ? d.msg : '', d.obj ?? null);
    }
    return typeof data === 'object' ? (data as Msg) : new Msg(false, 'unknown data:', data);
  }

  static async get<T = unknown>(url: string, params?: unknown, options: HttpOptions = {}): Promise<Msg<T>> {
    const { silent, ...axiosOpts } = options;
    try {
      const resp = await axios.get(url, { params, ...axiosOpts });
      const msg = this._respToMsg(resp) as Msg<T>;
      if (!silent) this._handleMsg(msg);
      return msg;
    } catch (error) {
      console.error('GET request failed:', error);
      const err = error as AxiosError<{ message?: string }>;
      const errorMsg = new Msg<T>(false, err.response?.data?.message || err.message || 'Request failed');
      if (!silent) this._handleMsg(errorMsg);
      return errorMsg;
    }
  }

  static async post<T = unknown>(url: string, data?: unknown, options: HttpOptions = {}): Promise<Msg<T>> {
    const { silent, ...axiosOpts } = options;
    try {
      const resp = await axios.post(url, data, axiosOpts);
      const msg = this._respToMsg(resp) as Msg<T>;
      if (!silent) this._handleMsg(msg);
      return msg;
    } catch (error) {
      console.error('POST request failed:', error);
      const err = error as AxiosError<{ message?: string }>;
      const errorMsg = new Msg<T>(false, err.response?.data?.message || err.message || 'Request failed');
      if (!silent) this._handleMsg(errorMsg);
      return errorMsg;
    }
  }

  static async postWithModal<T = unknown>(url: string, data?: unknown, modal?: HttpModal | null): Promise<Msg<T>> {
    if (modal) {
      modal.loading(true);
    }
    const msg = await this.post<T>(url, data);
    if (modal) {
      modal.loading(false);
      if (msg instanceof Msg && msg.success) {
        modal.close();
      }
    }
    return msg;
  }
}

export function applyDocumentTitle(): void {
  const host = window.location.hostname;
  if (!host) return;
  const current = document.title.trim();
  document.title = current ? `${host} - ${current}` : host;
}

export class PromiseUtil {
  static async sleep(timeout: number): Promise<void> {
    await new Promise<void>((resolve) => {
      setTimeout(resolve, timeout);
    });
  }
}

export interface RandomSeqOptions {
  type?: 'default' | 'hex';
  hasNumbers?: boolean;
  hasLowercase?: boolean;
  hasUppercase?: boolean;
}

export class RandomUtil {
  static getSeq({ type = 'default', hasNumbers = true, hasLowercase = true, hasUppercase = true }: RandomSeqOptions = {}): string {
    let seq = '';

    switch (type) {
      case 'hex':
        seq += '0123456789abcdef';
        break;
      default:
        if (hasNumbers) seq += '0123456789';
        if (hasLowercase) seq += 'abcdefghijklmnopqrstuvwxyz';
        if (hasUppercase) seq += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
        break;
    }

    return seq;
  }

  static randomInteger(min: number, max: number): number {
    const range = max - min + 1;
    const randomBuffer = new Uint32Array(1);
    window.crypto.getRandomValues(randomBuffer);
    return Math.floor((randomBuffer[0] / (0xFFFFFFFF + 1)) * range) + min;
  }

  static randomSeq(count: number, options: RandomSeqOptions = {}): string {
    const seq = this.getSeq(options);
    const seqLength = seq.length;
    const randomValues = new Uint32Array(count);
    window.crypto.getRandomValues(randomValues);
    return Array.from(randomValues, (v) => seq[v % seqLength]).join('');
  }

  static randomShortIds(): string {
    const lengths = [2, 4, 6, 8, 10, 12, 14, 16].sort(() => Math.random() - 0.5);
    return lengths.map((len) => this.randomSeq(len, { type: 'hex' })).join(',');
  }

  static randomLowerAndNum(len: number): string {
    return this.randomSeq(len, { hasUppercase: false });
  }

  static randomUUID(): string {
    if (window.location.protocol === 'https:') {
      return window.crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const randomValues = new Uint8Array(1);
      window.crypto.getRandomValues(randomValues);
      const randomValue = randomValues[0] % 16;
      const calculatedValue = c === 'x' ? randomValue : (randomValue & 0x3) | 0x8;
      return calculatedValue.toString(16);
    });
  }

  static randomShadowsocksPassword(method: string = '2022-blake3-aes-256-gcm'): string {
    const length = method === '2022-blake3-aes-128-gcm' ? 16 : 32;
    const array = new Uint8Array(length);
    window.crypto.getRandomValues(array);
    return Base64.alternativeEncode(String.fromCharCode(...array));
  }

  static isShadowsocks2022Password(password: string, method: string): boolean {
    if (!method || method.substring(0, 4) !== '2022') return true;
    const expected = method === '2022-blake3-aes-128-gcm' ? 16 : 32;
    try {
      return window.atob(password).length === expected;
    } catch {
      return false;
    }
  }

  static randomBase64(length: number = 16): string {
    const array = new Uint8Array(length);
    window.crypto.getRandomValues(array);
    return Base64.alternativeEncode(String.fromCharCode(...array));
  }

  static randomBase32String(length: number = 16): string {
    const array = new Uint8Array(length);
    window.crypto.getRandomValues(array);

    const base32Chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    let result = '';
    let bits = 0;
    let buffer = 0;

    for (let i = 0; i < array.length; i++) {
      buffer = (buffer << 8) | array[i];
      bits += 8;

      while (bits >= 5) {
        bits -= 5;
        result += base32Chars[(buffer >>> bits) & 0x1F];
      }
    }

    if (bits > 0) {
      result += base32Chars[(buffer << (5 - bits)) & 0x1F];
    }

    return result;
  }
}

type AnyRecord = Record<string, unknown>;

export class ObjectUtil {
  static getPropIgnoreCase(obj: AnyRecord, prop: string): unknown {
    for (const name in obj) {
      if (!Object.prototype.hasOwnProperty.call(obj, name)) continue;
      if (name.toLowerCase() === prop.toLowerCase()) {
        return obj[name];
      }
    }
    return undefined;
  }

  static deepSearch(obj: unknown, key: string): boolean {
    if (obj instanceof Array) {
      for (let i = 0; i < obj.length; ++i) {
        if (this.deepSearch(obj[i], key)) return true;
      }
    } else if (obj instanceof Object) {
      const rec = obj as AnyRecord;
      for (const name in rec) {
        if (!Object.prototype.hasOwnProperty.call(rec, name)) continue;
        if (this.deepSearch(rec[name], key)) return true;
      }
    } else {
      return this.isEmpty(obj) ? false : String(obj).toLowerCase().indexOf(key.toLowerCase()) >= 0;
    }
    return false;
  }

  static isEmpty(obj: unknown): boolean {
    return obj === null || obj === undefined || obj === '';
  }

  static isArrEmpty(arr: unknown): boolean {
    return !Array.isArray(arr) || arr.length === 0;
  }

  static copyArr<T>(dest: T[], src: T[]): void {
    dest.splice(0);
    for (const item of src) {
      dest.push(item);
    }
  }

  static clone<T>(obj: T): T {
    if (obj instanceof Array) {
      const newArr: unknown[] = [];
      this.copyArr(newArr, obj);
      return newArr as unknown as T;
    }
    if (obj instanceof Object) {
      const newObj: AnyRecord = {};
      const rec = obj as unknown as AnyRecord;
      for (const key of Object.keys(rec)) {
        newObj[key] = rec[key];
      }
      return newObj as unknown as T;
    }
    return obj;
  }

  static deepClone<T>(obj: T): T {
    if (obj instanceof Array) {
      const newArr: unknown[] = [];
      for (const item of obj) {
        newArr.push(this.deepClone(item));
      }
      return newArr as unknown as T;
    }
    if (obj instanceof Object) {
      const newObj: AnyRecord = {};
      const rec = obj as unknown as AnyRecord;
      for (const key of Object.keys(rec)) {
        newObj[key] = this.deepClone(rec[key]);
      }
      return newObj as unknown as T;
    }
    return obj;
  }

  static cloneProps(dest: object, src: object, ...ignoreProps: string[]): void {
    if (dest == null || src == null) return;
    const ignoreEmpty = this.isArrEmpty(ignoreProps);
    const d = dest as AnyRecord;
    const s = src as AnyRecord;
    for (const key of Object.keys(s)) {
      if (!Object.prototype.hasOwnProperty.call(s, key)) continue;
      if (!Object.prototype.hasOwnProperty.call(d, key)) continue;
      if (s[key] === undefined) continue;
      if (ignoreEmpty) {
        d[key] = s[key];
      } else {
        let ignore = false;
        for (let i = 0; i < ignoreProps.length; ++i) {
          if (key === ignoreProps[i]) {
            ignore = true;
            break;
          }
        }
        if (!ignore) {
          d[key] = s[key];
        }
      }
    }
  }

  static delProps(obj: object, ...props: string[]): void {
    const o = obj as AnyRecord;
    for (const prop of props) {
      if (prop in o) {
        delete o[prop];
      }
    }
  }

  static execute(func: unknown, ...args: unknown[]): void {
    if (!this.isEmpty(func) && typeof func === 'function') {
      (func as (...a: unknown[]) => unknown)(...args);
    }
  }

  static orDefault<T>(obj: T | null | undefined, defaultValue: T): T {
    if (obj == null) return defaultValue;
    return obj;
  }

  static equals(a: unknown, b: unknown): boolean {
    if (a == null || b == null || typeof a !== 'object' || typeof b !== 'object') {
      return a === b;
    }
    const ra = a as AnyRecord;
    const rb = b as AnyRecord;
    const aKeys = Object.keys(ra);
    const bKeys = Object.keys(rb);
    if (aKeys.length !== bKeys.length) return false;
    for (const key of aKeys) {
      if (!Object.prototype.hasOwnProperty.call(rb, key)) return false;
      if (ra[key] !== rb[key]) return false;
    }
    return true;
  }
}

export class Wireguard {
  static gf(init?: ArrayLike<number>): Float64Array {
    const r = new Float64Array(16);
    if (init) {
      for (let i = 0; i < init.length; ++i) r[i] = init[i];
    }
    return r;
  }

  static pack(o: Uint8Array, n: Float64Array): void {
    let b: number;
    const m = this.gf();
    const t = this.gf();
    for (let i = 0; i < 16; ++i) t[i] = n[i];
    this.carry(t);
    this.carry(t);
    this.carry(t);
    for (let j = 0; j < 2; ++j) {
      m[0] = t[0] - 0xffed;
      for (let i = 1; i < 15; ++i) {
        m[i] = t[i] - 0xffff - ((m[i - 1] >> 16) & 1);
        m[i - 1] &= 0xffff;
      }
      m[15] = t[15] - 0x7fff - ((m[14] >> 16) & 1);
      b = (m[15] >> 16) & 1;
      m[14] &= 0xffff;
      this.cswap(t, m, 1 - b);
    }
    for (let i = 0; i < 16; ++i) {
      o[2 * i] = t[i] & 0xff;
      o[2 * i + 1] = t[i] >> 8;
    }
  }

  static carry(o: Float64Array): void {
    for (let i = 0; i < 16; ++i) {
      o[(i + 1) % 16] += (i < 15 ? 1 : 38) * Math.floor(o[i] / 65536);
      o[i] &= 0xffff;
    }
  }

  static cswap(p: Float64Array, q: Float64Array, b: number): void {
    const c = ~(b - 1);
    let t: number;
    for (let i = 0; i < 16; ++i) {
      t = c & (p[i] ^ q[i]);
      p[i] ^= t;
      q[i] ^= t;
    }
  }

  static add(o: Float64Array, a: Float64Array, b: Float64Array): void {
    for (let i = 0; i < 16; ++i) o[i] = (a[i] + b[i]) | 0;
  }

  static subtract(o: Float64Array, a: Float64Array, b: Float64Array): void {
    for (let i = 0; i < 16; ++i) o[i] = (a[i] - b[i]) | 0;
  }

  static multmod(o: Float64Array, a: Float64Array, b: Float64Array): void {
    const t = new Float64Array(31);
    for (let i = 0; i < 16; ++i) {
      for (let j = 0; j < 16; ++j) t[i + j] += a[i] * b[j];
    }
    for (let i = 0; i < 15; ++i) t[i] += 38 * t[i + 16];
    for (let i = 0; i < 16; ++i) o[i] = t[i];
    this.carry(o);
    this.carry(o);
  }

  static invert(o: Float64Array, i: Float64Array): void {
    const c = this.gf();
    for (let a = 0; a < 16; ++a) c[a] = i[a];
    for (let a = 253; a >= 0; --a) {
      this.multmod(c, c, c);
      if (a !== 2 && a !== 4) this.multmod(c, c, i);
    }
    for (let a = 0; a < 16; ++a) o[a] = c[a];
  }

  static clamp(z: Uint8Array): void {
    z[31] = (z[31] & 127) | 64;
    z[0] &= 248;
  }

  static generatePublicKey(privateKey: Uint8Array): Uint8Array {
    let r: number;
    const z = new Uint8Array(32);
    const a = this.gf([1]);
    const b = this.gf([9]);
    const c = this.gf();
    const d = this.gf([1]);
    const e = this.gf();
    const f = this.gf();
    const _121665 = this.gf([0xdb41, 1]);
    const _9 = this.gf([9]);
    for (let i = 0; i < 32; ++i) z[i] = privateKey[i];
    this.clamp(z);
    for (let i = 254; i >= 0; --i) {
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

  static generatePresharedKey(): Uint8Array {
    const privateKey = new Uint8Array(32);
    window.crypto.getRandomValues(privateKey);
    return privateKey;
  }

  static generatePrivateKey(): Uint8Array {
    const privateKey = this.generatePresharedKey();
    this.clamp(privateKey);
    return privateKey;
  }

  static encodeBase64(dest: Uint8Array, src: Uint8Array): void {
    const input = Uint8Array.from([
      (src[0] >> 2) & 63,
      ((src[0] << 4) | (src[1] >> 4)) & 63,
      ((src[1] << 2) | (src[2] >> 6)) & 63,
      src[2] & 63,
    ]);
    for (let i = 0; i < 4; ++i) {
      dest[i] = input[i] + 65 +
        (((25 - input[i]) >> 8) & 6) -
        (((51 - input[i]) >> 8) & 75) -
        (((61 - input[i]) >> 8) & 15) +
        (((62 - input[i]) >> 8) & 3);
    }
  }

  static keyToBase64(key: Uint8Array): string {
    let i: number;
    const base64 = new Uint8Array(44);
    for (i = 0; i < 32 / 3; ++i) {
      this.encodeBase64(base64.subarray(i * 4), key.subarray(i * 3));
    }
    this.encodeBase64(base64.subarray(i * 4), Uint8Array.from([key[i * 3 + 0], key[i * 3 + 1], 0]));
    base64[43] = 61;
    return String.fromCharCode.apply(null, Array.from(base64));
  }

  static keyFromBase64(encoded: string): Uint8Array {
    const binaryStr = atob(encoded);
    const bytes = new Uint8Array(binaryStr.length);
    for (let i = 0; i < binaryStr.length; i++) {
      bytes[i] = binaryStr.charCodeAt(i);
    }
    return bytes;
  }

  static generateKeypair(secretKey: string = ''): { publicKey: string; privateKey: string } {
    const privateKey = secretKey.length > 0 ? this.keyFromBase64(secretKey) : this.generatePrivateKey();
    const publicKey = this.generatePublicKey(privateKey);
    return {
      publicKey: this.keyToBase64(publicKey),
      privateKey: secretKey.length > 0 ? secretKey : this.keyToBase64(privateKey),
    };
  }
}

export class ClipboardManager {
  static async copyText(content: unknown = ''): Promise<boolean> {
    const text = String(content ?? '');
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text);
        return true;
      } catch {}
    }
    return ClipboardManager._legacyCopy(text);
  }

  static _legacyCopy(text: string): boolean {
    const span = document.createElement('span');
    span.textContent = text;
    span.style.whiteSpace = 'pre';
    span.style.position = 'absolute';
    span.style.left = '-9999px';
    span.style.top = '0';

    document.body.appendChild(span);

    const selection = window.getSelection();
    if (!selection) {
      document.body.removeChild(span);
      return false;
    }

    const prevSelection = selection.rangeCount > 0 ? selection.getRangeAt(0) : null;

    selection.removeAllRanges();
    const range = window.document.createRange();
    range.selectNodeContents(span);
    selection.addRange(range);

    let ok = false;
    try {
      const exec = (document as unknown as Record<string, unknown>)['execCommand'];
      if (typeof exec === 'function') {
        ok = (exec as (cmd: string) => boolean).call(document, 'copy');
      }
    } catch {}

    selection.removeAllRanges();
    if (prevSelection) {
      selection.addRange(prevSelection);
    }

    document.body.removeChild(span);
    return ok;
  }
}

export class Base64 {
  static encode(content: string = '', safe: boolean = false): string {
    if (safe) {
      return Base64.encode(content)
        .replace(/\+/g, '-')
        .replace(/=/g, '')
        .replace(/\//g, '_');
    }
    return window.btoa(String.fromCharCode(...new TextEncoder().encode(content)));
  }

  static alternativeEncode(content: string): string {
    return window.btoa(content);
  }

  static decode(content: string = ''): string {
    return new TextDecoder().decode(
      Uint8Array.from(window.atob(content), (c) => c.charCodeAt(0)),
    );
  }
}

export class SizeFormatter {
  static readonly ONE_KB = 1024;
  static readonly ONE_MB = SizeFormatter.ONE_KB * 1024;
  static readonly ONE_GB = SizeFormatter.ONE_MB * 1024;
  static readonly ONE_TB = SizeFormatter.ONE_GB * 1024;
  static readonly ONE_PB = SizeFormatter.ONE_TB * 1024;

  static sizeFormat(size: number | null | undefined): string {
    if (size == null || !Number.isFinite(size) || size <= 0) return '0 B';
    if (size < SizeFormatter.ONE_KB) return size.toFixed(0) + ' B';
    if (size < SizeFormatter.ONE_MB) return (size / SizeFormatter.ONE_KB).toFixed(2) + ' KB';
    if (size < SizeFormatter.ONE_GB) return (size / SizeFormatter.ONE_MB).toFixed(2) + ' MB';
    if (size < SizeFormatter.ONE_TB) return (size / SizeFormatter.ONE_GB).toFixed(2) + ' GB';
    if (size < SizeFormatter.ONE_PB) return (size / SizeFormatter.ONE_TB).toFixed(2) + ' TB';
    return (size / SizeFormatter.ONE_PB).toFixed(2) + ' PB';
  }

  // Same unit ladder as sizeFormat, expressed per-second.
  static speedFormat(bps: number | null | undefined): string {
    return SizeFormatter.sizeFormat(bps) + '/s';
  }
}

export class CPUFormatter {
  static cpuSpeedFormat(speed: number): string {
    return speed > 1000 ? (speed / 1000).toFixed(2) + ' GHz' : speed.toFixed(2) + ' MHz';
  }

  static cpuCoreFormat(cores: number): string {
    return cores === 1 ? '1 Core' : cores + ' Cores';
  }
}

export class TimeFormatter {
  static formatSecond(second: number): string {
    if (second < 60) return second.toFixed(0) + 's';
    if (second < 3600) return (second / 60).toFixed(0) + 'm';
    if (second < 3600 * 24) return (second / 3600).toFixed(0) + 'h';
    const day = Math.floor(second / 3600 / 24);
    const remain = Number(((second / 3600) - (day * 24)).toFixed(0));
    return day + 'd' + (remain > 0 ? ' ' + remain + 'h' : '');
  }
}

export class NumberFormatter {
  static addZero(num: number): string | number {
    return num < 10 ? '0' + num : num;
  }

  static toFixed(num: number, n: number): number {
    const m = Math.pow(10, n);
    return Math.floor(num * m) / m;
  }
}

export class Utils {
  static debounce<A extends unknown[]>(fn: (...args: A) => unknown, delay: number): (...args: A) => void {
    let timeoutID: ReturnType<typeof setTimeout> | null = null;
    return function (this: unknown, ...args: A) {
      if (timeoutID !== null) clearTimeout(timeoutID);
      timeoutID = setTimeout(() => fn.apply(this, args), delay);
    };
  }
}

export class CookieManager {
  static getCookie(cname: string): string {
    const name = cname + '=';
    const ca = document.cookie.split(';');
    for (let c of ca) {
      c = c.trim();
      if (c.indexOf(name) === 0) {
        return decodeURIComponent(c.substring(name.length, c.length));
      }
    }
    return '';
  }

  static setCookie(cname: string, cvalue: string, exdays?: number): void {
    let expires = '';
    if (exdays) {
      const d = new Date();
      d.setTime(d.getTime() + exdays * 24 * 60 * 60 * 1000);
      expires = 'expires=' + d.toUTCString() + ';';
    }
    document.cookie = cname + '=' + encodeURIComponent(cvalue) + ';' + expires + 'path=/';
  }
}

const COLORS = {
  success: '#389e0a',
  warning: '#faad14',
  danger: '#ff4d4f',
  purple: '#722ed1',
} as const;

export type UsageColor = 'purple' | 'green' | 'orange' | 'red';

export interface ClientUsageStats {
  total: number;
  up: number;
  down: number;
}

export interface ExpiryClient {
  enable: boolean;
  expiryTime: number | null;
}

export class ColorUtils {
  static usageColor(
    data: number | null | undefined,
    threshold: number,
    total: number | { valueOf(): number } | null | undefined,
  ): UsageColor {
    const t = Number(total ?? 0);
    const d = Number(data);
    switch (true) {
      case data === null || data === undefined: return 'purple';
      case t < 0: return 'green';
      case t == 0: return 'purple';
      case d < t - threshold: return 'green';
      case d < t: return 'orange';
      default: return 'red';
    }
  }

  static clientUsageColor(clientStats: ClientUsageStats | null | undefined, trafficDiff: number): string {
    switch (true) {
      case !clientStats || clientStats.total == 0: return COLORS.purple;
      case clientStats!.up + clientStats!.down < clientStats!.total - trafficDiff: return COLORS.success;
      case clientStats!.up + clientStats!.down < clientStats!.total: return COLORS.warning;
      default: return COLORS.danger;
    }
  }

  static userExpiryColor(threshold: number, client: ExpiryClient, isDark: boolean = false): string {
    if (!client.enable) return isDark ? '#2c3950' : '#bcbcbc';
    const now = new Date().getTime();
    const expiry = client.expiryTime;
    switch (true) {
      case expiry === null: return COLORS.purple;
      case (expiry as number) < 0: return COLORS.success;
      case (expiry as number) == 0: return COLORS.purple;
      case now < (expiry as number) - threshold: return COLORS.success;
      case now < (expiry as number): return COLORS.warning;
      default: return COLORS.danger;
    }
  }
}

export class ArrayUtils {
  static doAllItemsExist<T>(array1: T[], array2: T[]): boolean {
    return array1.every((item) => array2.includes(item));
  }
}

export interface BuildURLOptions {
  host?: string;
  port?: string;
  isTLS?: boolean;
  base: string;
  path: string;
}

export class URLBuilder {
  static buildURL({ host, port, isTLS, base, path }: BuildURLOptions): string {
    if (!host || host.length === 0) host = window.location.hostname;
    if (!port || port.length === 0) port = window.location.port;
    if (isTLS === undefined) isTLS = window.location.protocol === 'https:';

    const protocol = isTLS ? 'https:' : 'http:';
    let portPart = String(port);
    if (portPart === '' || (isTLS && portPart === '443') || (!isTLS && portPart === '80')) {
      portPart = '';
    } else {
      portPart = `:${portPart}`;
    }

    return `${protocol}//${host}${portPart}${base}${path}`;
  }
}

export interface SupportedLanguage {
  name: string;
  value: string;
  icon: string;
}

export class LanguageManager {
  static readonly supportedLanguages: readonly SupportedLanguage[] = [
    { name: 'العربية', value: 'ar-EG', icon: '🇪🇬' },
    { name: 'English', value: 'en-US', icon: '🇺🇸' },
    { name: 'فارسی', value: 'fa-IR', icon: '🇮🇷' },
    { name: '简体中文', value: 'zh-CN', icon: '🇨🇳' },
    { name: '繁體中文', value: 'zh-TW', icon: '🇹🇼' },
    { name: '日本語', value: 'ja-JP', icon: '🇯🇵' },
    { name: 'Русский', value: 'ru-RU', icon: '🇷🇺' },
    { name: 'Tiếng Việt', value: 'vi-VN', icon: '🇻🇳' },
    { name: 'Español', value: 'es-ES', icon: '🇪🇸' },
    { name: 'Indonesian', value: 'id-ID', icon: '🇮🇩' },
    { name: 'Український', value: 'uk-UA', icon: '🇺🇦' },
    { name: 'Türkçe', value: 'tr-TR', icon: '🇹🇷' },
    { name: 'Português', value: 'pt-BR', icon: '🇧🇷' },
  ];

  static getLanguage(): string {
    let lang = CookieManager.getCookie('lang');
    if (lang) return lang;

    if (window.navigator) {
      const nav = window.navigator as Navigator & { userLanguage?: string };
      lang = nav.language || nav.userLanguage || '';

      const simularLangs: [string, string][] = [
        ['ar', LanguageManager.supportedLanguages[0].value],
        ['fa', LanguageManager.supportedLanguages[2].value],
        ['ja', LanguageManager.supportedLanguages[5].value],
        ['ru', LanguageManager.supportedLanguages[6].value],
        ['vi', LanguageManager.supportedLanguages[7].value],
        ['es', LanguageManager.supportedLanguages[8].value],
        ['id', LanguageManager.supportedLanguages[9].value],
        ['uk', LanguageManager.supportedLanguages[10].value],
        ['tr', LanguageManager.supportedLanguages[11].value],
        ['pt', LanguageManager.supportedLanguages[12].value],
      ];

      simularLangs.forEach((pair) => {
        if (lang === pair[0]) {
          lang = pair[1];
        }
      });

      if (LanguageManager.isSupportLanguage(lang)) {
        CookieManager.setCookie('lang', lang, 365);
      } else {
        CookieManager.setCookie('lang', 'en-US', 365);
        window.location.reload();
      }
    } else {
      CookieManager.setCookie('lang', 'en-US', 365);
      window.location.reload();
    }

    return lang;
  }

  static setLanguage(language: string): void {
    if (!LanguageManager.isSupportLanguage(language)) {
      language = 'en-US';
    }
    CookieManager.setCookie('lang', language, 365);
    window.location.reload();
  }

  static isSupportLanguage(language: string): boolean {
    return LanguageManager.supportedLanguages.some((lang) => lang.value === language);
  }
}

export class FileManager {
  static downloadTextFile(content: BlobPart, filename: string = 'file.txt', options: BlobPropertyBag = { type: 'text/plain' }): void {
    const link = window.document.createElement('a');
    link.download = filename;
    link.style.border = '0';
    link.style.padding = '0';
    link.style.margin = '0';
    link.style.position = 'absolute';
    link.style.left = '-9999px';
    link.style.top = `${window.pageYOffset || window.document.documentElement.scrollTop}px`;
    link.href = URL.createObjectURL(new Blob([content], options));
    link.click();
    URL.revokeObjectURL(link.href);
    link.remove();
  }
}

export type CalendarKind = 'gregorian' | 'jalalian';

export class IntlUtil {
  static formatDate(date: string | number | Date | null | undefined, calendar: CalendarKind = 'gregorian'): string {
    if (date == null) return '';
    const language = LanguageManager.getLanguage();
    const locale = calendar === 'jalalian' ? 'fa-IR' : language;

    const intlOptions: Intl.DateTimeFormatOptions = {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    };

    const intl = new Intl.DateTimeFormat(locale, intlOptions);
    return intl.format(new Date(date));
  }

  static formatRelativeTime(date: number | null | undefined): string {
    if (date == null) return '';
    const language = LanguageManager.getLanguage();
    const now = new Date();
    const diff = date < 0
      ? Math.round(date / (1000 * 60 * 60 * 24))
      : Math.round((date - now.getTime()) / (1000 * 60 * 60 * 24));
    const formatter = new Intl.RelativeTimeFormat(language, { numeric: 'auto' });
    return formatter.format(diff, 'day');
  }
}
