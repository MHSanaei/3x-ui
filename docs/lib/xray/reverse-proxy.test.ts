import { describe, it, expect } from 'vitest';
import { buildProxyConfig, buildCertCommand, type ReverseProxyOptions } from './reverse-proxy';

const base: ReverseProxyOptions = {
  server: 'nginx',
  domain: 'panel.example.com',
  panelPort: '2053',
  panelPath: '/panel',
  certTool: 'certbot',
};

describe('buildProxyConfig — nginx', () => {
  it('includes WebSocket upgrade headers and the upstream port', () => {
    const cfg = buildProxyConfig(base);
    expect(cfg).toContain('server_name panel.example.com;');
    expect(cfg).toContain('proxy_pass http://127.0.0.1:2053;');
    expect(cfg).toContain('proxy_set_header Upgrade $http_upgrade;');
    expect(cfg).toContain('proxy_set_header Connection "upgrade";');
    expect(cfg).toContain('location /panel/');
  });
});

describe('buildProxyConfig — caddy', () => {
  it('produces a path-scoped reverse_proxy', () => {
    const cfg = buildProxyConfig({ ...base, server: 'caddy' });
    expect(cfg).toContain('panel.example.com {');
    expect(cfg).toContain('reverse_proxy /panel/* 127.0.0.1:2053');
  });

  it('proxies the whole site when path is root', () => {
    const cfg = buildProxyConfig({ ...base, server: 'caddy', panelPath: '/' });
    expect(cfg).toContain('reverse_proxy 127.0.0.1:2053');
  });
});

describe('buildCertCommand', () => {
  it('supports certbot and acme.sh', () => {
    expect(buildCertCommand(base)).toContain('certbot certonly --nginx -d panel.example.com');
    expect(buildCertCommand({ ...base, certTool: 'acme.sh' })).toContain(
      'acme.sh --issue -d panel.example.com',
    );
  });
});
