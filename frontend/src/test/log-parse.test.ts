import { describe, it, expect } from 'vitest';

import { parseLogLine } from '@/pages/index/logParse';

// Fixtures are real lines captured from `journalctl -u x-ui` on a production
// host (the SysLog view) plus the in-memory app-log format. Each journald entry
// carries a "Mon DD HH:MM:SS host ident[pid]: " prefix that the viewer used to
// mistake for the level, leaving only a bare timestamp on screen.
describe('parseLogLine — SysLog (journalctl) formats', () => {
  it('x-ui go-logging line: keeps level, strips prefix, tags X-UI', () => {
    const r = parseLogLine(
      'Jun 08 23:57:28 ubuntu-4gb-fsn1-1 /usr/local/x-ui/x-ui[72297]: INFO - mtproto: started mtg for inbound 3 on 0.0.0.0:8443',
    );
    expect(r.stamp).toBe('Jun 08 23:57:28');
    expect(r.levelText).toBe('INFO');
    expect(r.service).toBe('X-UI:');
    expect(r.body).toBe('mtproto: started mtg for inbound 3 on 0.0.0.0:8443');
  });

  it('xray go-logging line: lifts the XRAY service tag', () => {
    const r = parseLogLine(
      'Jun 08 23:56:52 ubuntu-4gb-fsn1-1 /usr/local/x-ui/x-ui[72297]: WARNING - XRAY: core: Xray 26.6.1 started',
    );
    expect(r.stamp).toBe('Jun 08 23:56:52');
    expect(r.levelText).toBe('WARNING');
    expect(r.service).toBe('XRAY:');
    expect(r.body).toBe('core: Xray 26.6.1 started');
  });

  it('Go std-log line: strips the redundant embedded date, keeps the message', () => {
    const r = parseLogLine(
      'Jun 08 19:22:22 ubuntu-4gb-fsn1-1 x-ui[1439]: 2026/06/08 19:22:22 http: TLS handshake error from 18.97.5.1:36022: EOF',
    );
    expect(r.stamp).toBe('Jun 08 19:22:22');
    expect(r.levelText).toBe('');
    expect(r.body).toBe('http: TLS handshake error from 18.97.5.1:36022: EOF');
  });

  it('telego bracketed line: lifts the ERROR level out of "[ts] ERROR ..."', () => {
    const r = parseLogLine(
      'Jun 09 00:14:52 ubuntu-4gb-fsn1-1 x-ui[72297]: [Tue Jun  9 00:14:52 UTC 2026] ERROR Retrying getting updates in 8s...',
    );
    expect(r.stamp).toBe('Jun 09 00:14:52');
    expect(r.levelText).toBe('ERROR');
    expect(r.body).toBe('Retrying getting updates in 8s...');
  });

  it('systemd line: shows the body rather than a bare timestamp', () => {
    const r = parseLogLine(
      'Jun 08 23:56:47 ubuntu-4gb-fsn1-1 systemd[1]: Stopping x-ui.service - x-ui Service...',
    );
    expect(r.stamp).toBe('Jun 08 23:56:47');
    expect(r.body).toBe('Stopping x-ui.service - x-ui Service...');
  });

  it('never collapses a journald entry to just its timestamp', () => {
    const r = parseLogLine(
      'Jun 09 00:15:00 ubuntu-4gb-fsn1-1 x-ui[72297]: [Tue Jun  9 00:15:00 UTC 2026] ERROR Getting updates: telego: getUpdates: api: 409 "Conflict"',
    );
    expect(r.body.length).toBeGreaterThan(0);
    expect(r.body).toContain('Conflict');
  });
});

describe('parseLogLine — app-log format (SysLog off)', () => {
  it('parses "YYYY/MM/DD HH:MM:SS LEVEL - body"', () => {
    const r = parseLogLine('2026/06/09 00:35:09 INFO - mtproto: started mtg for inbound 3 on 0.0.0.0:8443');
    expect(r.date).toBe('2026/06/09');
    expect(r.time).toBe('00:35:09');
    expect(r.levelText).toBe('INFO');
    expect(r.service).toBe('X-UI:');
    expect(r.body).toBe('mtproto: started mtg for inbound 3 on 0.0.0.0:8443');
  });

  it('handles an empty line without throwing', () => {
    const r = parseLogLine('');
    expect(r.stamp).toBe('');
    expect(r.body).toBe('');
  });
});
