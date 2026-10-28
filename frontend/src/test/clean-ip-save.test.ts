import { describe, expect, it } from 'vitest';

import { inboundDomain, inboundTargets, ispGroupsOf, planSave } from '@/pages/clean-ip/scan';
import type { HostRecord } from '@/schemas/api/host';
import type { InboundOption } from '@/schemas/client';

const wsGroup: HostRecord = {
  groupId: 'g1',
  inboundIds: [3],
  hosts: ['cdn.example.com:443'],
  remark: 'US WS',
  port: 443,
  security: 'tls',
  sni: '',
  isDisabled: true,
};

describe('inboundTargets', () => {
  const options: InboundOption[] = [
    { id: 1, remark: 'HttpU', network: 'httpupgrade' },
    { id: 2, remark: 'Reality', network: 'tcp', security: 'reality' },
    { id: 3, remark: 'WS', network: 'ws' },
    { id: 4, remark: 'XHTTP', network: 'xhttp' },
  ];

  // Without a host group the page had nothing to save into and the admin had to make one by hand.
  it('offers each CDN-capable inbound that no host group serves yet', () => {
    const got = inboundTargets(options, [wsGroup]);
    expect(got.map((g) => [g.groupId, g.remark, g.inboundIds])).toEqual([
      ['inbound:1', 'HttpU', [1]],
      ['inbound:4', 'XHTTP', [4]],
    ]);
  });
});

describe('inboundDomain', () => {
  // A new host for an inbound has no SNI yet: the scan's domain has to come from the inbound.
  it('reads the transport host first, then a Host header, then the TLS server name', () => {
    expect(
      inboundDomain(
        JSON.stringify({ network: 'ws', wsSettings: { host: 'ws.example.com' }, security: 'none' }),
      ),
    ).toBe('ws.example.com');
    expect(
      inboundDomain({
        network: 'httpupgrade',
        httpupgradeSettings: { host: '', headers: { Host: 'hdr.example.com' } },
      }),
    ).toBe('hdr.example.com');
    expect(
      inboundDomain({
        network: 'grpc',
        grpcSettings: {},
        security: 'tls',
        tlsSettings: { serverName: 'tls.example.com' },
      }),
    ).toBe('tls.example.com');
    expect(inboundDomain('not json')).toBe('');
  });
});

describe('planSave', () => {
  const entries = ['104.16.1.1:443', '104.16.1.2:443'];

  it('writes the IPs into the chosen host itself, turns it on and pins the tested domain as SNI', () => {
    const plan = planSave({
      group: wsGroup,
      ispGroups: new Map(),
      isp: 'MCI',
      to: 'same',
      entries,
      domain: 'cdn.example.com',
      fragment: undefined,
    });
    expect(plan.groupId).toBe('g1');
    expect(plan.remark).toBe('US WS');
    expect(plan.values.hosts).toEqual(entries);
    expect(plan.values.isDisabled).toBe(false);
    expect(plan.values.sni).toBe('cdn.example.com');
    expect(plan.values.snis).toEqual(['cdn.example.com']);
  });

  it('creates the host for an inbound that had none', () => {
    const [target] = inboundTargets([{ id: 1, remark: 'HttpU', network: 'httpupgrade' }], []);
    const plan = planSave({
      group: target,
      ispGroups: new Map(),
      isp: 'MCI',
      to: 'same',
      entries,
      domain: 'cdn.example.com',
      fragment: undefined,
    });
    expect(plan.groupId).toBeUndefined();
    expect(plan.remark).toBe('HttpU');
    expect(plan.values.inboundIds).toEqual([1]);
    expect(plan.values.security).toBe('tls');
    expect(plan.values.port).toBe(443);
  });

  it('updates the existing ISP host instead of adding a second one', () => {
    const ispGroup: HostRecord = {
      ...wsGroup,
      groupId: 'g2',
      remark: 'US WS · MCI',
      hosts: ['1.1.1.1:443'],
      sni: 'other.example.com',
    };
    const plan = planSave({
      group: wsGroup,
      ispGroups: ispGroupsOf(wsGroup, [wsGroup, ispGroup]),
      isp: 'MCI',
      to: 'isp',
      entries,
      domain: 'cdn.example.com',
      fragment: null,
    });
    expect(plan.groupId).toBe('g2');
    expect(plan.remark).toBe('US WS · MCI');
    expect(plan.values.sni).toBe('other.example.com');
  });

  it('starts a separate host for an ISP that has none yet', () => {
    const plan = planSave({
      group: wsGroup,
      ispGroups: new Map(),
      isp: 'MCI',
      to: 'isp',
      entries,
      domain: 'cdn.example.com',
      fragment: undefined,
    });
    expect(plan.groupId).toBeUndefined();
    expect(plan.remark).toBe('US WS · MCI');
  });
});
