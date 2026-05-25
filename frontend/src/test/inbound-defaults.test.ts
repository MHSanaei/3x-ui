import { describe, expect, it } from 'vitest';

import {
  createDefaultHysteriaClient,
  createDefaultShadowsocksClient,
  createDefaultTrojanClient,
  createDefaultVlessClient,
  createDefaultVmessClient,
} from '@/lib/xray/inbound-defaults';
import { HysteriaClientSchema } from '@/schemas/protocols/inbound/hysteria';
import { ShadowsocksClientSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { TrojanClientSchema } from '@/schemas/protocols/inbound/trojan';
import { VlessClientSchema } from '@/schemas/protocols/inbound/vless';
import { VmessClientSchema } from '@/schemas/protocols/inbound/vmess';

// Tests pass explicit seeds for every random field so the assertions don't
// depend on window.crypto (the node test env has no crypto.randomUUID).
// Each factory is verified two ways:
//   1. snapshot — locks the exact shape
//   2. Zod parse round-trip — confirms the factory output is a valid
//      member of the protocol's client schema (no missing defaults, no
//      stray fields)

const seed = {
  email: 'fixture@example.test',
  subId: 'fixed-sub-id-1234',
};

describe('createDefaultVlessClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultVlessClient({ ...seed, id: '11111111-2222-4333-8444-555555555555' });
    expect(c).toMatchSnapshot();
    expect(VlessClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultVmessClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultVmessClient({ ...seed, id: 'aaaaaaaa-bbbb-4ccc-9ddd-eeeeeeeeeeee' });
    expect(c).toMatchSnapshot();
    expect(VmessClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultTrojanClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultTrojanClient({ ...seed, password: 'fixed-trojan-pw' });
    expect(c).toMatchSnapshot();
    expect(TrojanClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultShadowsocksClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultShadowsocksClient({ ...seed, password: 'ZmFrZS1zcy1wYXNzd29yZA==' });
    expect(c).toMatchSnapshot();
    expect(ShadowsocksClientSchema.parse(c)).toEqual(c);
  });
});

describe('createDefaultHysteriaClient', () => {
  it('produces a Zod-valid client', () => {
    const c = createDefaultHysteriaClient({ ...seed, auth: 'fixed-hyst-auth' });
    expect(c).toMatchSnapshot();
    expect(HysteriaClientSchema.parse(c)).toEqual(c);
  });
});
