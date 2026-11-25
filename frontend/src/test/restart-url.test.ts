import { describe, expect, it } from 'vitest';

import { panelUrlAfterRestart } from '@/pages/settings/restart-url';

const plain2053 = { port: 2053, tls: false, nextPort: 2053, nextTls: false };

describe('panelUrlAfterRestart', () => {
  it.each([
    {
      name: 'a TLS-terminating proxy on its own port keeps its scheme and port',
      current: 'https://panel.example.com:8443/panel/settings#security',
      restart: plain2053,
      want: 'https://panel.example.com:8443/panel/settings#security',
    },
    {
      name: 'reached directly, the panel turning TLS on moves the browser to https',
      current: 'http://203.0.113.5:2053/panel/settings',
      restart: { ...plain2053, nextTls: true },
      want: 'https://203.0.113.5:2053/panel/settings',
    },
    {
      name: 'reached directly by name, a new panel port is followed',
      current: 'https://panel.example.com:2053/panel/settings',
      restart: { port: 2053, tls: true, nextPort: 3000, nextTls: true },
      want: 'https://panel.example.com:3000/panel/settings',
    },
    {
      name: 'a published port that maps onto the panel port is kept',
      current: 'http://203.0.113.5:8080/panel/settings',
      restart: plain2053,
      want: 'http://203.0.113.5:8080/panel/settings',
    },
    {
      name: 'an implicit port 80 stays 80 when the scheme changes under it',
      current: 'http://203.0.113.5/panel/settings',
      restart: { port: 80, tls: false, nextPort: 80, nextTls: true },
      want: 'https://203.0.113.5:80/panel/settings',
    },
    {
      name: 'without a listener answer only the path is rebuilt',
      current: 'https://panel.example.com:8443/old/panel/settings',
      restart: undefined,
      settings: { webBasePath: '/new' },
      want: 'https://panel.example.com:8443/new/panel/settings',
    },
    {
      name: 'an IP entry follows a panel domain that is itself an IP',
      current: 'http://203.0.113.5:2053/panel/settings',
      restart: plain2053,
      settings: { webDomain: '2001:db8::7' },
      want: 'http://[2001:db8::7]:2053/panel/settings',
    },
  ])('$name', ({ current, restart, settings, want }) => {
    expect(panelUrlAfterRestart(current, restart, settings ?? {})).toBe(want);
  });
});
