import type { AmneziawgServer } from '@/schemas/protocols/inbound/amneziawg';

/*
 * Client-side AmneziaWG 3.1 obfuscation generator, mirroring the ranges and
 * constraints of the Go backend's amneziawg.GenerateObfuscation31
 * (internal/amneziawg/params.go). Exact parity isn't required — the user can
 * edit any field afterward and the backend validates on save — but the two
 * generators must stay range-compatible so a value produced here always
 * passes the Go-side ValidateObfuscation.
 */

export type AwgObfuscation = Pick<
  AmneziawgServer,
  | 'jc'
  | 'jmin'
  | 'jmax'
  | 's1'
  | 's2'
  | 's3'
  | 's4'
  | 'h1'
  | 'h2'
  | 'h3'
  | 'h4'
  | 'i1'
  | 'i2'
  | 'i3'
  | 'i4'
  | 'i5'
  | 'headerProtectionKey'
  | 'contentPaddingAddition'
  | 'rekeyAfterTime'
  | 'rekeyTimeout'
  | 'rejectAfterTime'
  | 'keepaliveTimeout'
  | 'maxHandshakeAttempts'
  | 'randomTrailers'
  | 'disableCookies'
>;

const randInt = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));

/*
 * base64 of 32 crypto-grade random bytes — the exact HeaderProtectionKey
 * shape amneziawg-tools parses and the Go backend validates.
 */
const generateHeaderProtectionKey = (): string => {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes));
};

/*
 * Four distinct values for H1-H4: split the space into four bands and take
 * one random value from each (low bound >= 5 since 1-4 are reserved for
 * vanilla WireGuard message types).
 *
 * Each H is a single value, not a range. amneziawg-go's packet classifier
 * only ever compares a fixed-size ciphertext prefix against these bounds, so
 * a wider range buys no DPI resistance -- the range boundaries themselves are
 * never observable on the wire, only the single value the classifier
 * happens to let through. A wide range does cost real throughput: with
 * randomTrailers on (as every generated set here has it), the handshake-size
 * checks relax from == to >, so a wide H-range starts misclassifying an
 * equally wide fraction of ordinary transport packets as handshakes and
 * silently dropping them (amnezia-vpn/amneziawg-go#183).
 */
const generateHValues = (): [string, string, string, string] => {
  const hMax = 2147483647;
  const lo = 5;
  const bandSize = Math.floor((hMax - lo + 1) / 4);
  return Array.from({ length: 4 }, (_, i) => {
    const bandLo = lo + i * bandSize;
    const bandHi = bandLo + bandSize - 1;
    return `${randInt(bandLo, bandHi)}`;
  }) as [string, string, string, string];
};

export function generateAwgObfuscation(): AwgObfuscation {
  const jmin = randInt(40, 89);
  const s1 = randInt(15, 150);
  let s2 = randInt(15, 150);
  while (s1 + 56 === s2) {
    s2 = randInt(15, 150);
  }
  const [h1, h2, h3, h4] = generateHValues();

  /*
   * Timing windows bracket WireGuard's stock constants (rekey 120s, reject
   * 180s, retry 5s, keepalive 10s); every reject value exceeds every rekey
   * value by >= 30s by construction, matching the Go generator and its
   * ValidateObfuscation cross-check. Content padding stays <= 64 total for
   * the same MTU-headroom reason that caps s4 at 32.
   */
  const cpLo = randInt(8, 24);
  const rekeyLo = randInt(100, 120);
  const rekeyHi = rekeyLo + randInt(10, 40);
  const rejectLo = rekeyHi + randInt(30, 60);
  const rekeyTimeoutLo = randInt(3, 6);
  const keepaliveLo = randInt(8, 12);
  const attemptsLo = randInt(15, 25);

  return {
    jc: randInt(3, 6),
    jmin,
    jmax: jmin + randInt(50, 250),
    s1,
    s2,
    // Floored at 12, not the protocol's 0/8/4 minima: headerProtectionKey is
    // always generated below, and IpcSet rejects it unless every s1-s4 >= 12.
    s3: randInt(12, 55),
    s4: randInt(12, 27),
    h1,
    h2,
    h3,
    h4,
    i1: `<r ${randInt(32, 256)}>`,
    i2: '',
    i3: '',
    i4: '',
    i5: '',
    headerProtectionKey: generateHeaderProtectionKey(),
    contentPaddingAddition: `${cpLo}-${cpLo + randInt(8, 40)}`,
    rekeyAfterTime: `${rekeyLo}-${rekeyHi}`,
    rekeyTimeout: `${rekeyTimeoutLo}-${rekeyTimeoutLo + randInt(1, 4)}`,
    rejectAfterTime: `${rejectLo}-${rejectLo + randInt(30, 90)}`,
    keepaliveTimeout: `${keepaliveLo}-${keepaliveLo + randInt(2, 8)}`,
    maxHandshakeAttempts: `${attemptsLo}-${attemptsLo + randInt(5, 25)}`,
    randomTrailers: true,
    disableCookies: true,
  };
}
