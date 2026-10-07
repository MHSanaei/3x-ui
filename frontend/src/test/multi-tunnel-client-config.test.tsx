import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import ClientInfoModal from '@/pages/clients/ClientInfoModal';
import ClientQrModal from '@/pages/clients/ClientQrModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import type { HostRecord } from '@/schemas/api/host';
import { renderWithProviders } from './test-utils';

const deAwgInbound: InboundOption = {
  id: 101,
  tag: 'awg-de',
  remark: 'DE · Kelsterbach',
  port: 52716,
  protocol: 'amneziawg',
  nodeAddress: 'de.vpn.example.com',
  awgServer: {
    publicKey: 'deServerPublicKey==',
    primaryDns: '1.1.1.1',
    secondaryDns: '1.0.0.1',
    mtu: 1420,
    jc: 4,
    jmin: 40,
    jmax: 100,
    s1: 30,
    s2: 90,
    s3: 0,
    s4: 0,
    h1: '123',
    h2: '456',
    h3: '789',
    h4: '101112',
  },
};

const fiAwgInbound: InboundOption = {
  id: 102,
  tag: 'awg-fi',
  remark: 'FI · Helsinki',
  port: 26641,
  protocol: 'amneziawg',
  nodeAddress: 'fi.vpn.example.com',
  awgServer: {
    publicKey: 'fiServerPublicKey==',
    primaryDns: '8.8.8.8',
    secondaryDns: '8.8.4.4',
    mtu: 1380,
    jc: 10,
    jmin: 20,
    jmax: 80,
    s1: 25,
    s2: 50,
    s3: 0,
    s4: 0,
    h1: '999',
    h2: '888',
    h3: '777',
    h4: '666',
  },
};

const usWgInbound: InboundOption = {
  id: 201,
  tag: 'wg-us',
  remark: 'US · New York',
  port: 51820,
  protocol: 'wireguard',
  nodeAddress: 'us.vpn.example.com',
  wgPublicKey: 'usWgServerPublicKey==',
  wgDns: '1.1.1.1',
  wgMtu: 1420,
};

const euWgInbound: InboundOption = {
  id: 202,
  tag: 'wg-eu',
  remark: 'EU · Frankfurt',
  port: 51821,
  protocol: 'wireguard',
  nodeAddress: 'eu.vpn.example.com',
  wgPublicKey: 'euWgServerPublicKey==',
  wgDns: '9.9.9.9',
  wgMtu: 1400,
};

const multiAwgClient: ClientRecord = {
  id: 'c1',
  email: 'NSK-RT-01',
  privateKey: 'clientPrivateKey==',
  publicKey: 'clientPublicKey==',
  preSharedKey: 'clientPsk==',
  allowedIPs: '10.8.0.2/32',
  keepAlive: 25,
  inboundIds: [101, 102],
  enable: true,
} as unknown as ClientRecord;

const multiWgClient: ClientRecord = {
  id: 'c2',
  email: 'WG-CLIENT',
  privateKey: 'wgClientPrivateKey==',
  publicKey: 'wgClientPublicKey==',
  preSharedKey: 'wgClientPsk==',
  allowedIPs: '10.0.0.2/32',
  keepAlive: 25,
  inboundIds: [201, 202],
  enable: true,
} as unknown as ClientRecord;

const singleAwgClient: ClientRecord = {
  id: 'c3',
  email: 'SINGLE-CLIENT',
  privateKey: 'clientPrivateKey==',
  publicKey: 'clientPublicKey==',
  allowedIPs: '10.8.0.2/32',
  inboundIds: [101],
  enable: true,
} as unknown as ClientRecord;

describe('Multi-tunnel Client Modals', () => {
  it('renders distinct labeled ConfigBlocks in ClientInfoModal for multiple AmneziaWG inbounds', () => {
    renderWithProviders(
      <ClientInfoModal
        open
        client={multiAwgClient}
        inboundsById={{ 101: deAwgInbound, 102: fiAwgInbound }}
        isOnline={false}
        tunnelAllowedIPs={{ 101: '10.8.1.5/32', 102: '10.8.2.10/32' }}
        onOpenChange={() => {}}
      />,
    );

    expect(screen.getAllByText('DE · Kelsterbach')).toHaveLength(2);
    expect(screen.getByText('FI · Helsinki')).toBeTruthy();
    expect(document.querySelectorAll('.config-block')).toHaveLength(2);
  });

  it('renders distinct labeled ConfigBlocks in ClientInfoModal for multiple WireGuard inbounds', () => {
    renderWithProviders(
      <ClientInfoModal
        open
        client={multiWgClient}
        inboundsById={{ 201: usWgInbound, 202: euWgInbound }}
        isOnline={false}
        tunnelAllowedIPs={{ 201: '10.0.1.2/32', 202: '10.0.2.2/32' }}
        onOpenChange={() => {}}
      />,
    );

    expect(screen.getAllByText('US · New York')).toHaveLength(2);
    expect(screen.getByText('EU · Frankfurt')).toBeTruthy();
    expect(document.querySelectorAll('.config-block')).toHaveLength(2);
  });

  it('renders single default-labeled ConfigBlock in ClientInfoModal for single inbound', () => {
    renderWithProviders(
      <ClientInfoModal
        open
        client={singleAwgClient}
        inboundsById={{ 101: deAwgInbound }}
        isOnline={false}
        onOpenChange={() => {}}
      />,
    );

    expect(document.querySelectorAll('.config-block')).toHaveLength(1);
    expect(screen.getByText('Config')).toBeTruthy();
  });

  it('renders separate collapse panels in ClientQrModal for multiple AmneziaWG inbounds', () => {
    renderWithProviders(
      <MemoryRouter initialEntries={['/clients']}>
        <ClientQrModal
          open
          client={multiAwgClient}
          inboundsById={{ 101: deAwgInbound, 102: fiAwgInbound }}
          tunnelAllowedIPs={{ 101: '10.8.1.5/32', 102: '10.8.2.10/32' }}
          onOpenChange={() => {}}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText('DE · Kelsterbach')).toBeTruthy();
    expect(screen.getByText('FI · Helsinki')).toBeTruthy();
  });

  // The subscription beside these configs advertises the inbound's Hosts, so
  // the panel-built configs must too — one per Host address.
  const edgeHosts: HostRecord[] = [
    {
      groupId: 'cdn',
      inboundIds: [201],
      hosts: ['edge.example.com:443', 'edge2.example.com'],
      remark: 'CDN',
    },
  ];
  const edgeClient = { ...multiWgClient, inboundIds: [201] } as ClientRecord;
  const edgeLabels = [
    'US · New York - edge.example.com:443',
    'US · New York - edge2.example.com:51820',
  ];

  it('renders one ConfigBlock per Host in ClientInfoModal', () => {
    renderWithProviders(
      <ClientInfoModal
        open
        client={edgeClient}
        inboundsById={{ 201: usWgInbound }}
        isOnline={false}
        hosts={edgeHosts}
        onOpenChange={() => {}}
      />,
    );

    for (const label of edgeLabels) expect(screen.getByText(label)).toBeTruthy();
  });

  it('renders one collapse panel per Host in ClientQrModal', () => {
    renderWithProviders(
      <MemoryRouter initialEntries={['/clients']}>
        <ClientQrModal
          open
          client={edgeClient}
          inboundsById={{ 201: usWgInbound }}
          hosts={edgeHosts}
          onOpenChange={() => {}}
        />
      </MemoryRouter>,
    );

    for (const label of edgeLabels) expect(screen.getByText(label)).toBeTruthy();
  });

  it('builds the TUIC Clash config only from Hosts the Clash subscription serves', () => {
    const tuicInbound = { id: 301, remark: 'tuic', protocol: 'tuic', port: 8443 } as InboundOption;
    const tuicClient = {
      id: 'c4',
      email: 'TUIC-CLIENT',
      uuid: 'e79b9107-1607-4e6c-a496-d8f99e4f0dc5',
      password: 'secret',
      inboundIds: [301],
    } as unknown as ClientRecord;
    const hosts: HostRecord[] = [
      { groupId: 'clash', inboundIds: [301], hosts: ['clash.example.com:443'] },
      {
        groupId: 'raw-only',
        inboundIds: [301],
        hosts: ['raw.example.com:443'],
        excludeFromSubTypes: ['clash'],
      },
    ];
    renderWithProviders(
      <MemoryRouter initialEntries={['/clients']}>
        <ClientQrModal
          open
          client={tuicClient}
          inboundsById={{ 301: tuicInbound }}
          hosts={hosts}
          onOpenChange={() => {}}
        />
      </MemoryRouter>,
    );

    expect(screen.getAllByText('TUIC config (Clash)')).toHaveLength(1);
    expect(screen.queryByText('raw.example.com:443')).toBeNull();
  });
});
