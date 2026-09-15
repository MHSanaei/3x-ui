import { HappRoutingProfileSchema, type HappRoutingProfile } from '@/schemas/happRouting';
import { toBase64Utf8 } from './happPresets';

export type HappRoutingMode = 'add' | 'onadd';
export type HappRoutingListKey =
  | 'DirectSites'
  | 'DirectIp'
  | 'ProxySites'
  | 'ProxyIp'
  | 'BlockSites'
  | 'BlockIp';

export type HappRoutingLoadResult =
  | { success: true; profile: HappRoutingProfile; mode: HappRoutingMode; isNew: boolean }
  | { success: false; error: 'off' | 'remote' | 'invalid' };

export function parseHappRoutingJson(input: string): HappRoutingProfile | null {
  try {
    const value: unknown = JSON.parse(input);
    const result = HappRoutingProfileSchema.safeParse(value);
    // Keep the validated input object; schema output can discard some unknown extension keys.
    return result.success ? (value as HappRoutingProfile) : null;
  } catch {
    return null;
  }
}

export function loadHappRouting(input: string): HappRoutingLoadResult {
  const source = input.trim();
  if (!source) {
    return {
      success: true,
      profile: {
        Name: 'Custom Rules',
        GlobalProxy: 'true',
        DirectSites: [],
        DirectIp: [],
        ProxySites: [],
        ProxyIp: [],
        BlockSites: [],
        BlockIp: [],
        DomainStrategy: 'IPIfNonMatch',
      },
      mode: 'onadd',
      isNew: true,
    };
  }
  if (source === 'happ://routing/off') return { success: false, error: 'off' };
  if (/^https?:\/\//i.test(source)) return { success: false, error: 'remote' };

  if (source.startsWith('{')) {
    const profile = parseHappRoutingJson(source);
    return profile
      ? { success: true, profile, mode: 'onadd', isNew: false }
      : { success: false, error: 'invalid' };
  }

  const match = /^happ:\/\/routing\/(onadd|add)\/([A-Za-z0-9+/_-]+={0,2})$/.exec(source);
  if (!match) return { success: false, error: 'invalid' };

  try {
    const binary = atob(match[2].replace(/-/g, '+').replace(/_/g, '/'));
    // Decode UTF-8 strictly so malformed bytes cannot silently change profile names or rules.
    const json = new TextDecoder('utf-8', { fatal: true }).decode(
      Uint8Array.from(binary, (character) => character.charCodeAt(0)),
    );
    const profile = parseHappRoutingJson(json);
    return profile
      ? { success: true, profile, mode: match[1] as HappRoutingMode, isNew: false }
      : { success: false, error: 'invalid' };
  } catch {
    return { success: false, error: 'invalid' };
  }
}

export function buildHappRoutingDeeplink(
  profile: HappRoutingProfile,
  mode: HappRoutingMode = 'onadd',
): string {
  return `happ://routing/${mode}/` + toBase64Utf8(JSON.stringify(profile));
}

export function parseHappRoutingList(input: string): string[] {
  // Commas can belong to regexp rules, so the editor uses one rule per line.
  return input
    .split(/\r?\n/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}
