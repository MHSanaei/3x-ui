import { describe, expect, it } from 'vitest';

import { validateSessionIDLength, validateSessionIDTable } from '@/lib/xray/xhttp-session-id';
import { XHttpStreamSettingsSchema } from '@/schemas/protocols/stream/xhttp';

// xray-core #6258: sessionPlacement/sessionKey were renamed to
// sessionIDPlacement/sessionIDKey. The schema must lift legacy keys off
// stored configs so an upgraded panel never silently drops them.
describe('XHttpStreamSettingsSchema legacy migration', () => {
  it('lifts legacy sessionPlacement/sessionKey onto the renamed keys', () => {
    const parsed = XHttpStreamSettingsSchema.parse({
      sessionPlacement: 'cookie',
      sessionKey: 'x_session',
    });
    expect(parsed.sessionIDPlacement).toBe('cookie');
    expect(parsed.sessionIDKey).toBe('x_session');
    // legacy keys must not survive — we never emit both names
    expect((parsed as Record<string, unknown>).sessionPlacement).toBeUndefined();
    expect((parsed as Record<string, unknown>).sessionKey).toBeUndefined();
  });

  it('prefers an explicit new key over a legacy one', () => {
    const parsed = XHttpStreamSettingsSchema.parse({
      sessionPlacement: 'cookie',
      sessionIDPlacement: 'header',
    });
    expect(parsed.sessionIDPlacement).toBe('header');
  });

  it('defaults the new fields to empty', () => {
    const parsed = XHttpStreamSettingsSchema.parse({});
    expect(parsed.sessionIDTable).toBe('');
    expect(parsed.sessionIDLength).toBe('');
  });
});

describe('sessionID validators', () => {
  it('accepts empty and ASCII tables, rejects non-ASCII', async () => {
    await expect(validateSessionIDTable(null, '')).resolves.toBeUndefined();
    await expect(validateSessionIDTable(null, 'Base62')).resolves.toBeUndefined();
    await expect(validateSessionIDTable(null, 'ABCdef0123')).resolves.toBeUndefined();
    await expect(validateSessionIDTable(null, ' café')).rejects.toThrow();
  });

  it('accepts a positive length/range, rejects zero or junk', async () => {
    await expect(validateSessionIDLength(null, '')).resolves.toBeUndefined();
    await expect(validateSessionIDLength(null, '8')).resolves.toBeUndefined();
    await expect(validateSessionIDLength(null, '16-32')).resolves.toBeUndefined();
    await expect(validateSessionIDLength(null, '8-8')).resolves.toBeUndefined();
    await expect(validateSessionIDLength(null, '0-16')).rejects.toThrow();
    await expect(validateSessionIDLength(null, '32-16')).rejects.toThrow();
    await expect(validateSessionIDLength(null, 'abc')).rejects.toThrow();
  });
});
