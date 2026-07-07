import { describe, it, expect } from 'vitest';
import { buildUfwCommands, buildNftablesRuleset, type FirewallOptions } from './firewall';

const options: FirewallOptions = {
  allowSsh: true,
  sshPort: 22,
  ports: [
    { port: 443, protocol: 'tcp', label: 'inbound' },
    { port: 51820, protocol: 'udp', label: 'wireguard' },
    { port: 2053, protocol: 'both', label: 'panel' },
  ],
};

describe('buildUfwCommands', () => {
  it('emits ufw allow rules per protocol plus ssh and enable', () => {
    const out = buildUfwCommands(options);
    expect(out).toContain('ufw allow 22/tcp');
    expect(out).toContain('ufw allow 443/tcp');
    expect(out).toContain('ufw allow 51820/udp');
    // 'both' expands to tcp + udp
    expect(out).toContain('ufw allow 2053/tcp');
    expect(out).toContain('ufw allow 2053/udp');
    expect(out.trim().endsWith('ufw enable')).toBe(true);
  });
});

describe('buildNftablesRuleset', () => {
  it('emits a drop-by-default inet table with accept rules', () => {
    const out = buildNftablesRuleset(options);
    expect(out).toContain('table inet filter');
    expect(out).toContain('policy drop;');
    expect(out).toContain('ct state established,related accept');
    expect(out).toContain('tcp dport 443 accept');
    expect(out).toContain('udp dport 51820 accept');
    expect(out).toContain('tcp dport 2053 accept');
    expect(out).toContain('udp dport 2053 accept');
  });
});
