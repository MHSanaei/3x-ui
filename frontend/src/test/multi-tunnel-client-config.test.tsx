import { describe, it, expect } from 'vitest';
import { screen } from '@testing-library/react';

import ClientInfoModal from '@/pages/clients/ClientInfoModal';
import ClientQrModal from '@/pages/clients/ClientQrModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
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
      <ClientQrModal
        open
        client={multiAwgClient}
        inboundsById={{ 101: deAwgInbound, 102: fiAwgInbound }}
        tunnelAllowedIPs={{ 101: '10.8.1.5/32', 102: '10.8.2.10/32' }}
        onOpenChange={() => {}}
      />,
    );

    expect(screen.getByText('DE · Kelsterbach')).toBeTruthy();
    expect(screen.getByText('FI · Helsinki')).toBeTruthy();
  });
});
