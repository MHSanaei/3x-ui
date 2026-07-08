export type VlessAuthKind =
  | 'x25519'
  | 'x25519_xorpub'
  | 'x25519_random'
  | 'mlkem768'
  | 'mlkem768_xorpub'
  | 'mlkem768_random';

export const VLESS_AUTH_LABEL_KEYS: Record<VlessAuthKind, string> = {
  x25519: 'pages.inbounds.vlessAuthX25519',
  x25519_xorpub: 'pages.inbounds.vlessAuthX25519Xorpub',
  x25519_random: 'pages.inbounds.vlessAuthX25519Random',
  mlkem768: 'pages.inbounds.vlessAuthMlkem768',
  mlkem768_xorpub: 'pages.inbounds.vlessAuthMlkem768Xorpub',
  mlkem768_random: 'pages.inbounds.vlessAuthMlkem768Random',
};

const MLKEM768_MIN_KEY_LENGTH = 300;

export function vlessEncryptionAuthKind(encryption: string): VlessAuthKind | null {
  if (!encryption || encryption === 'none') return null;
  const parts = encryption.split('.').filter(Boolean);
  const authKey = parts[parts.length - 1] || '';
  if (!authKey) return null;
  const mode = parts[1] || 'native';
  const keyType = authKey.length > MLKEM768_MIN_KEY_LENGTH ? 'mlkem768' : 'x25519';
  if (mode === 'xorpub' || mode === 'random') return `${keyType}_${mode}`;
  return keyType;
}
