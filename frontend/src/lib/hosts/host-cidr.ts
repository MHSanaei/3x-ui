export const MAX_RANDOM_CIDR_COUNT = 100;

export function cleanCIDR(s: string): string {
  let clean = s.trim();
  if (clean.startsWith('[') && clean.endsWith(']')) {
    clean = clean.slice(1, -1);
  }
  if (clean.startsWith('[')) {
    clean = clean.slice(1);
  }
  clean = clean.replace(/\]\//, '/');
  return clean;
}

export function parseIPv4(ipStr: string): number | null {
  const parts = ipStr.trim().split('.');
  if (parts.length !== 4) return null;
  let u32 = 0;
  for (const part of parts) {
    if (!/^\d{1,3}$/.test(part)) return null;
    const n = Number(part);
    if (n < 0 || n > 255) return null;
    u32 = ((u32 << 8) | n) >>> 0;
  }
  return u32;
}

export function formatIPv4(val: number): string {
  return `${(val >>> 24) & 255}.${(val >>> 16) & 255}.${(val >>> 8) & 255}.${val & 255}`;
}

export function parseIPv6(ipStr: string): bigint | null {
  let s = ipStr.trim().toLowerCase();
  if (s.startsWith('[') && s.endsWith(']')) {
    s = s.slice(1, -1);
  }
  const doubleColonCount = (s.match(/::/g) ?? []).length;
  if (doubleColonCount > 1) return null;

  let leftParts: string[] = [];
  let rightParts: string[] = [];

  if (s.includes('::')) {
    const [left, right] = s.split('::');
    leftParts = left ? left.split(':') : [];
    rightParts = right ? right.split(':') : [];
  } else {
    leftParts = s.split(':');
  }

  const totalParts = leftParts.length + rightParts.length;
  if (s.includes('::')) {
    if (totalParts > 7) return null;
  } else {
    if (totalParts !== 8) return null;
  }

  const missingZeros = 8 - totalParts;
  const allParts = [...leftParts, ...Array(missingZeros).fill('0'), ...rightParts];

  let result = 0n;
  for (const part of allParts) {
    if (!/^[0-9a-f]{1,4}$/.test(part)) return null;
    result = (result << 16n) | BigInt(parseInt(part, 16));
  }
  return result;
}

export function formatIPv6(val: bigint): string {
  const parts: number[] = [];
  for (let i = 7; i >= 0; i--) {
    parts.push(Number((val >> BigInt(i * 16)) & 0xffffn));
  }

  let bestStart = -1;
  let bestLen = 0;
  let currentStart = -1;
  let currentLen = 0;

  for (let i = 0; i < 8; i++) {
    if (parts[i] === 0) {
      if (currentStart === -1) {
        currentStart = i;
        currentLen = 1;
      } else {
        currentLen++;
      }
    } else {
      if (currentLen > bestLen) {
        bestLen = currentLen;
        bestStart = currentStart;
      }
      currentStart = -1;
      currentLen = 0;
    }
  }
  if (currentLen > bestLen) {
    bestLen = currentLen;
    bestStart = currentStart;
  }

  if (bestLen >= 2) {
    const left = parts
      .slice(0, bestStart)
      .map((p) => p.toString(16))
      .join(':');
    const right = parts
      .slice(bestStart + bestLen)
      .map((p) => p.toString(16))
      .join(':');
    return `${left}::${right}`;
  }

  return parts.map((p) => p.toString(16)).join(':');
}

export function isCIDR(s: string): boolean {
  const clean = cleanCIDR(s);
  const slashIdx = clean.indexOf('/');
  if (slashIdx === -1) return false;
  const ipPart = clean.slice(0, slashIdx);
  const prefixStr = clean.slice(slashIdx + 1);
  if (!/^\d+$/.test(prefixStr)) return false;
  const prefix = Number(prefixStr);

  const v4 = parseIPv4(ipPart);
  if (v4 !== null) {
    return prefix >= 0 && prefix <= 32;
  }
  const v6 = parseIPv6(ipPart);
  if (v6 !== null) {
    return prefix >= 0 && prefix <= 128;
  }
  return false;
}

export function randomIPsFromCIDR(cidrStr: string, count: number): string[] {
  if (count <= 0) count = 1;
  if (count > MAX_RANDOM_CIDR_COUNT) count = MAX_RANDOM_CIDR_COUNT;

  const clean = cleanCIDR(cidrStr);
  const slashIdx = clean.indexOf('/');
  if (slashIdx === -1) return [];
  const ipPart = clean.slice(0, slashIdx);
  const prefixStr = clean.slice(slashIdx + 1);
  if (!/^\d+$/.test(prefixStr)) return [];
  const prefix = Number(prefixStr);

  const v4 = parseIPv4(ipPart);
  if (v4 !== null && prefix >= 0 && prefix <= 32) {
    return randomIPv4FromCIDR(v4, prefix, count);
  }
  const v6 = parseIPv6(ipPart);
  if (v6 !== null && prefix >= 0 && prefix <= 128) {
    return randomIPv6FromCIDR(v6, prefix, count);
  }
  return [];
}

function randomIPv4FromCIDR(ip4: number, prefix: number, count: number): string[] {
  const hostBits = 32 - prefix;
  const mask = prefix === 0 ? 0 : (~0 << hostBits) >>> 0;
  const baseU32 = (ip4 & mask) >>> 0;

  if (hostBits === 0) {
    return [formatIPv4(baseU32)];
  }
  if (hostBits === 1) {
    const out = [formatIPv4(baseU32)];
    if (count > 1) {
      out.push(formatIPv4((baseU32 + 1) >>> 0));
    }
    return out;
  }

  const total = 2 ** hostBits;
  const usableCount = total - 2;
  if (usableCount <= 0) {
    return [formatIPv4(baseU32)];
  }
  if (count > usableCount) {
    count = usableCount;
  }

  const seen = new Set<string>();
  const result: string[] = [];
  let maxAttempts = count * 100;

  while (result.length < count && maxAttempts > 0) {
    maxAttempts--;
    const offset = Math.floor(Math.random() * usableCount) + 1;
    const targetU32 = (baseU32 + offset) >>> 0;
    const ipStr = formatIPv4(targetU32);
    if (!seen.has(ipStr)) {
      seen.add(ipStr);
      result.push(ipStr);
    }
  }
  return result;
}

function randomIPv6FromCIDR(ip6: bigint, prefix: number, count: number): string[] {
  const hostBits = 128 - prefix;
  const mask = prefix === 0 ? 0n : ((1n << 128n) - 1n) ^ ((1n << BigInt(hostBits)) - 1n);
  const baseInt = ip6 & mask;

  if (hostBits === 0) {
    return [formatIPv6(baseInt)];
  }
  if (hostBits === 1) {
    const out = [formatIPv6(baseInt)];
    if (count > 1) {
      out.push(formatIPv6(baseInt + 1n));
    }
    return out;
  }

  const maxHost = 1n << BigInt(hostBits);
  // Exclude the Subnet-Router Anycast address (all host bits 0, RFC 4291 §2.6.1)
  const usableCount = maxHost - 1n;
  if (usableCount < BigInt(count)) {
    count = Number(usableCount);
  }

  const seen = new Set<string>();
  const result: string[] = [];
  let maxAttempts = count * 100;

  while (result.length < count && maxAttempts > 0) {
    maxAttempts--;
    const offset = randomBigIntInRange(1n, usableCount);
    const targetInt = baseInt + offset;
    const ipStr = formatIPv6(targetInt);
    if (!seen.has(ipStr)) {
      seen.add(ipStr);
      result.push(ipStr);
    }
  }
  return result;
}

function randomBigIntInRange(min: bigint, max: bigint): bigint {
  const range = max - min + 1n;
  const bits = range.toString(2).length;
  const bytes = Math.ceil(bits / 8);
  const buf = new Uint8Array(bytes);

  while (true) {
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
      crypto.getRandomValues(buf);
    } else {
      for (let i = 0; i < bytes; i++) {
        buf[i] = Math.floor(Math.random() * 256);
      }
    }
    let val = 0n;
    for (let i = 0; i < bytes; i++) {
      val = (val << 8n) | BigInt(buf[i]);
    }
    const mask = (1n << BigInt(bits)) - 1n;
    val = val & mask;
    if (val < range) {
      return min + val;
    }
  }
}
