import { describe, expect, it } from 'vitest';

import { mergeWarpRotation } from '@/pages/xray/overrides/WarpModal';

const clientId = btoa(String.fromCharCode(1, 2, 3));

function rotatedConfig(overrides: { public_key?: string; host?: string; v4?: string; v6?: string } = {}) {
  return {
    config: {
      client_id: clientId,
      interface: { addresses: { v4: overrides.v4 ?? '172.16.0.2', v6: overrides.v6 ?? '2606:4700::2' } },
      peers: [{ public_key: overrides.public_key ?? 'newPub', endpoint: { host: overrides.host ?? 'engage.cloudflareclient.com:2408' } }],
    },
  };
}

describe('mergeWarpRotation', () => {
  it('patches only rotated fields and preserves user customizations', () => {
    const existing = {
      tag: 'warp',
      protocol: 'wireguard',
      settings: {
        mtu: 1280,
        secretKey: 'oldSecret',
        address: ['172.16.0.2/32'],
        reserved: [9, 9, 9],
        domainStrategy: 'ForceIPv4',
        noKernelTun: false,
        peers: [
          {
            publicKey: 'oldPub',
            endpoint: 'engage.cloudflareclient.com:2408',
            keepAlive: 25,
            allowedIPs: ['10.0.0.0/24'],
            preSharedKey: 'psk',
          },
          { publicKey: 'extraPeer', endpoint: 'extra.test:51820' },
        ],
      },
    };

    const merged = mergeWarpRotation(
      existing,
      { private_key: 'newSecret' },
      rotatedConfig({ public_key: 'newPub', host: 'engage.cloudflareclient.com:2408', v4: '172.16.0.9' }),
    );

    expect(merged).not.toBeNull();
    const settings = (merged as { settings: Record<string, unknown> }).settings;
    expect(settings.secretKey).toBe('newSecret');
    expect(settings.address).toEqual(['172.16.0.9/32', '2606:4700::2/128']);
    expect(settings.reserved).toEqual([1, 2, 3]);
    const peers = settings.peers as Array<Record<string, unknown>>;
    expect(peers[0].publicKey).toBe('newPub');
    expect(peers[0].endpoint).toBe('engage.cloudflareclient.com:2408');
    expect(peers[0].keepAlive).toBe(25);
    expect(peers[0].allowedIPs).toEqual(['10.0.0.0/24']);
    expect(peers[0].preSharedKey).toBe('psk');
    expect(peers[1]).toEqual({ publicKey: 'extraPeer', endpoint: 'extra.test:51820' });
    expect(settings.mtu).toBe(1280);
    expect(settings.domainStrategy).toBe('ForceIPv4');
    expect(settings.noKernelTun).toBe(false);
  });

  it('does not mutate the existing outbound object', () => {
    const existing = {
      tag: 'warp',
      protocol: 'wireguard',
      settings: {
        secretKey: 'oldSecret',
        address: ['172.16.0.2/32'],
        reserved: [9, 9, 9],
        peers: [{ publicKey: 'oldPub', endpoint: 'old:1', keepAlive: 25 }],
      },
    };
    const snapshot = JSON.parse(JSON.stringify(existing));

    mergeWarpRotation(existing, { private_key: 'newSecret' }, rotatedConfig());

    expect(existing).toEqual(snapshot);
  });

  it('returns null when the rotation response has no peer', () => {
    expect(mergeWarpRotation(undefined, null, { config: { peers: [] } })).toBeNull();
    expect(mergeWarpRotation(undefined, null, null)).toBeNull();
  });

  it('seeds a default warp outbound when none existed yet', () => {
    const merged = mergeWarpRotation(undefined, { private_key: 'newSecret' }, rotatedConfig());
    expect(merged).not.toBeNull();
    expect((merged as Record<string, unknown>).tag).toBe('warp');
    expect((merged as Record<string, unknown>).protocol).toBe('wireguard');
    const settings = (merged as { settings: Record<string, unknown> }).settings;
    expect(settings.secretKey).toBe('newSecret');
    expect(settings.peers).toEqual([
      { publicKey: 'newPub', endpoint: 'engage.cloudflareclient.com:2408' },
    ]);
  });
});
