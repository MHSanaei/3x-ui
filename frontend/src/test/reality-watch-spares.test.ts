import { describe, expect, it } from 'vitest';

import { addSpare } from '@/pages/inbounds/form/security/realityWatch';

// The server keys spares as lower-case host:port and drops the current target; adding the
// same site twice from the scanner must not create a second entry that the server collapses.
describe('addSpare', () => {
  it('normalizes to host:port and skips repeats and the current target', () => {
    let spares: string[] = [];
    spares = addSpare(spares, 'WWW.Example.com', 'cur.example.com:443');
    spares = addSpare(spares, 'www.example.com:443', 'cur.example.com:443');
    spares = addSpare(spares, 'other.example.com:8443', 'cur.example.com:443');
    spares = addSpare(spares, 'cur.example.com', 'cur.example.com:443');
    expect(spares).toEqual(['www.example.com:443', 'other.example.com:8443']);
  });

  it('keeps at most 10, like the server', () => {
    let spares: string[] = [];
    for (let i = 0; i < 12; i++) spares = addSpare(spares, `s${i}.example.com`, '');
    expect(spares).toHaveLength(10);
    expect(spares[9]).toBe('s9.example.com:443');
  });
});
