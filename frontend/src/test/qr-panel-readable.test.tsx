import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { genAmneziaWGConfig } from '@/lib/xray/inbound-link';
import QrPanel from '@/pages/inbounds/qr/QrPanel';
import { AmneziawgInboundSettingsSchema } from '@/schemas/protocols/inbound/amneziawg';

const KEY = `${'A'.repeat(43)}=`;

function awgConfig(disableCookies: boolean): string {
  const settings = AmneziawgInboundSettingsSchema.parse({
    server: {
      publicKey: KEY,
      mtu: 1420,
      primaryDns: '8.8.8.8',
      secondaryDns: '8.8.4.4',
      jc: 4,
      jmin: 65,
      jmax: 220,
      s1: 87,
      s2: 44,
      s3: 21,
      s4: 19,
      h1: '462980921-463150218',
      h2: '1177681572-1177787900',
      h3: '1907413509-1907903969',
      h4: '2029908558-2030313135',
      i1: '<r 148>',
      headerProtectionKey: KEY,
      contentPaddingAddition: '17-49',
      rekeyAfterTime: '111-139',
      rekeyTimeout: '4-7',
      rejectAfterTime: '187-251',
      keepaliveTimeout: '9-14',
      maxHandshakeAttempts: '19-36',
      randomTrailers: true,
      disableCookies,
    },
    clients: [
      {
        email: 'my-client',
        privateKey: KEY,
        preSharedKey: KEY,
        allowedIPs: ['10.8.1.2/32'],
        keepAlive: 25,
      },
    ],
  });

  return genAmneziaWGConfig({
    settings,
    address: 'your-server.example.com',
    port: 443,
    remark: 'my-client',
    peerIndex: 0,
  });
}

function qrGeometry(value: string): { viewBox: string; foreground: string } {
  const { container } = render(<QrPanel value={value} />);
  const svg = container.querySelector('.qr-code svg');
  const paths = svg?.querySelectorAll('path');

  expect(svg).not.toBeNull();
  expect(paths).toHaveLength(2);

  return {
    viewBox: svg?.getAttribute('viewBox') ?? '',
    foreground: paths?.item(1).getAttribute('d') ?? '',
  };
}

describe('QrPanel dense AmneziaWG config', () => {
  it('keeps the complete 3.1 config readable across the DisableCookies QR boundary', () => {
    const complete = awgConfig(true);
    const withoutDisableCookies = awgConfig(false);

    expect(complete).toContain('DisableCookies = on\n');
    expect(complete.length - withoutDisableCookies.length).toBe(20);

    const completeQr = qrGeometry(complete);
    const shorterQr = qrGeometry(withoutDisableCookies);

    expect(completeQr.viewBox).toBe('0 0 105 105');
    expect(shorterQr.viewBox).toBe('0 0 101 101');
    expect(completeQr.foreground).toMatch(/^M4 4h7/);
    expect(shorterQr.foreground).toMatch(/^M4 4h7/);
  });
});
