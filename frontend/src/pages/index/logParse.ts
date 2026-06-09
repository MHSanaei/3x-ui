// Parser for the panel log viewer. Logs reach the UI in two shapes:
//
//  - App log (SysLog off): the in-memory buffer, formatted as
//      "2006/01/02 15:04:05 LEVEL - message"
//  - SysLog (journalctl -o short): every entry is prefixed with
//      "Mon DD HH:MM:SS host ident[pid]: " before the real message, and the
//    message itself is one of several shapes depending on which subsystem
//    emitted it:
//      "INFO - mtproto: ..."                  go-logging (x-ui + xray)
//      "2026/06/08 19:22:22 http: ..."        Go std log (net/http, runtime)
//      "[Mon Jun  8 23:56:52 UTC 2026] ERROR ..."  telego bot
//      "Stopping x-ui.service - ..."          systemd
//
// parseLogLine normalises all of these into a stamp + level + service + body so
// the viewer renders a readable line instead of a bare timestamp.

export interface ParsedLog {
  date: string;
  time: string;
  stamp: string;
  levelText: string;
  levelClass: string;
  service: string;
  body: string;
}

export const LEVELS = ['DEBUG', 'INFO', 'NOTICE', 'WARNING', 'ERROR'];
export const LEVEL_CLASSES = [
  'level-debug',
  'level-info',
  'level-notice',
  'level-warning',
  'level-error',
];

// "Mon DD HH:MM:SS host ident[pid]: <message>" — captures the journal date,
// time, and the message that follows the syslog identifier.
const SYSLOG_PREFIX = /^([A-Za-z]{3}\s+\d{1,2})\s+(\d{2}:\d{2}:\d{2})\s+\S+\s+\S+?:\s+(.*)$/;
// Redundant Go std-log date prefix ("2006/01/02 15:04:05 ") to strip — the
// journal already carries the timestamp.
const GO_LOG_DATE = /^\d{4}\/\d{2}\/\d{2}\s+\d{2}:\d{2}:\d{2}\s+/;
// telego's own line prefix: "[Mon Jan _2 15:04:05 MST 2006] LEVEL rest".
const TELEGO = /^\[[^\]]+\]\s+([A-Z]+)\s+(.*)$/;

// splitLevelDash pulls a leading "LEVEL - " off a message, returning the level
// and the remainder. Returns null when the message does not start with a level.
function splitLevelDash(message: string): { level: string; rest: string } | null {
  const dash = message.indexOf(' - ');
  if (dash < 0) return null;
  const level = message.slice(0, dash).trim();
  if (LEVELS.indexOf(level) < 0) return null;
  return { level, rest: message.slice(dash + 3) };
}

export function parseLogLine(line: string): ParsedLog {
  const raw = (line || '').trim();

  let date = '';
  let time = '';
  let levelText = '';
  let body: string;

  const sys = raw.match(SYSLOG_PREFIX);
  if (sys) {
    date = sys[1];
    time = sys[2];
    let message = sys[3];

    const ld = splitLevelDash(message);
    if (ld) {
      // go-logging: "LEVEL - message"
      levelText = ld.level;
      body = ld.rest;
    } else {
      // Strip the redundant Go std-log date, then try to lift a level out of a
      // telego "[timestamp] LEVEL ..." line; otherwise keep the message as-is.
      message = message.replace(GO_LOG_DATE, '');
      const tg = message.match(TELEGO);
      if (tg && LEVELS.indexOf(tg[1]) >= 0) {
        levelText = tg[1];
        body = tg[2];
      } else {
        body = message;
      }
    }
  } else {
    // App-log format: "2006/01/02 15:04:05 LEVEL - body"
    const [head, ...rest] = raw.split(' - ');
    const message = rest.join(' - ');
    const parts = head.split(' ');
    if (parts.length >= 3) {
      [date, time, levelText] = parts;
    } else {
      levelText = head;
    }
    body = message || '';
  }

  const li = LEVELS.indexOf(levelText);
  const levelClass = li >= 0 ? LEVEL_CLASSES[li] : 'level-unknown';

  let service = '';
  if (body.startsWith('XRAY:')) {
    service = 'XRAY:';
    body = body.slice('XRAY:'.length).trimStart();
  } else if (body) {
    service = 'X-UI:';
  }

  const stamp = [date, time].filter(Boolean).join(' ');

  return { date, time, stamp, levelText, levelClass, service, body };
}
