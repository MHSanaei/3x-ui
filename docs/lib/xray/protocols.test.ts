import { describe, it, expect } from 'vitest';
import { recommend } from './protocols';

describe('recommend', () => {
  it('recommends REALITY for heavy censorship + modern clients', () => {
    const r = recommend({ useCase: 'censorship', censorship: 'high', clientSupport: 'modern' });
    expect(r.protocol).toBe('VLESS');
    expect(r.security).toContain('REALITY');
    expect(r.links.some((l) => l.href === '/docs/config/reality')).toBe(true);
  });

  it('falls back to VMess+WS+TLS for broad clients under censorship', () => {
    const r = recommend({ useCase: 'censorship', censorship: 'high', clientSupport: 'broad' });
    expect(r.protocol).toBe('VMess');
    expect(r.transport).toBe('WebSocket');
    expect(r.security).toBe('TLS');
  });

  it('recommends Trojan for speed with broad clients', () => {
    const r = recommend({ useCase: 'speed', censorship: 'low', clientSupport: 'broad' });
    expect(r.protocol).toBe('Trojan');
  });

  it('recommends a CDN-friendly default for general modern use', () => {
    const r = recommend({ useCase: 'general', censorship: 'low', clientSupport: 'modern' });
    expect(r.protocol).toBe('VLESS');
    expect(r.transport).toBe('WebSocket');
  });

  it('always returns a non-empty rationale and at least one link', () => {
    const r = recommend({ useCase: 'general', censorship: 'medium', clientSupport: 'broad' });
    expect(r.rationale.length).toBeGreaterThan(0);
    expect(r.links.length).toBeGreaterThan(0);
  });
});
