// Pure validation + templating helpers for 3x-ui's Telegram bot settings.
// Grounded in internal/web/service/tgbot/tgbot.go (admin ids parsed with
// strconv.ParseInt(_,10,64); token handed to telego.NewBot → api.telegram.org)
// and the panel's tg* settings (tgRunTime uses robfig/cron). No React/DOM.

export interface BotConfig {
  token: string;
  adminIds: string; // raw comma-separated input
  runTime: string; // @daily | @every 8h | 5/6-field cron
}

export interface TokenValidation {
  valid: boolean;
  botId?: string;
  error?: string;
}

export interface AdminIdsResult {
  ids: number[];
  invalid: string[];
}

export type CronKind = 'macro' | 'every' | 'cron' | 'invalid';
export interface CronValidation {
  valid: boolean;
  kind: CronKind;
  error?: string;
}

// BotFather tokens are <bot-id>:<35+ char secret of [A-Za-z0-9_-]>.
const TOKEN_RE = /^(\d+):[A-Za-z0-9_-]{35,}$/;

export function validateBotToken(token: string): TokenValidation {
  const t = token.trim();
  if (!t) return { valid: false, error: 'Token is empty.' };
  const m = TOKEN_RE.exec(t);
  if (!m) {
    return { valid: false, error: 'Expected the BotFather format <bot-id>:<35+ char secret>.' };
  }
  return { valid: true, botId: m[1] };
}

export function parseAdminIds(raw: string): AdminIdsResult {
  const ids: number[] = [];
  const invalid: string[] = [];
  for (const part of raw.split(',').map((s) => s.trim()).filter(Boolean)) {
    // Telegram chat ids are integers; group/channel ids are negative.
    if (/^-?\d+$/.test(part)) ids.push(Number(part));
    else invalid.push(part);
  }
  return { ids, invalid };
}

// robfig/cron predefined macros (note: @reboot is NOT supported).
const CRON_MACROS = new Set([
  '@yearly',
  '@annually',
  '@monthly',
  '@weekly',
  '@daily',
  '@midnight',
  '@hourly',
]);

// Go duration: one or more <number><unit> chunks (ns, us/µs, ms, s, m, h).
const GO_DURATION_RE = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/;

export function validateRunTime(s: string): CronValidation {
  const v = s.trim();
  if (!v) return { valid: false, kind: 'invalid', error: 'Schedule is empty.' };

  if (v.startsWith('@every ')) {
    const dur = v.slice('@every '.length).trim();
    if (GO_DURATION_RE.test(dur)) return { valid: true, kind: 'every' };
    return { valid: false, kind: 'invalid', error: `Invalid @every duration: "${dur}".` };
  }

  if (v.startsWith('@')) {
    if (CRON_MACROS.has(v)) return { valid: true, kind: 'macro' };
    return { valid: false, kind: 'invalid', error: `Unknown macro: "${v}".` };
  }

  const fields = v.split(/\s+/);
  if (fields.length === 5 || fields.length === 6) return { valid: true, kind: 'cron' };
  return {
    valid: false,
    kind: 'invalid',
    error: 'Use a 5/6-field cron, an @macro (e.g. @daily), or @every <duration>.',
  };
}

export function telegramApiBase(token: string): string {
  return `https://api.telegram.org/bot${token.trim()}`;
}

export function renderMessageTemplate(tpl: string, vars: Record<string, string>): string {
  return tpl.replace(/\{\{\s*(\w+)\s*\}\}/g, (_match, key: string) =>
    key in vars ? vars[key] : `{{${key}}}`,
  );
}

export function buildBotConfigSummary(c: BotConfig): {
  tgBotEnable: boolean;
  tgBotToken: string;
  tgBotChatId: string;
  tgRunTime: string;
} {
  const { ids } = parseAdminIds(c.adminIds);
  return {
    tgBotEnable: true,
    tgBotToken: c.token.trim(),
    tgBotChatId: ids.join(','),
    tgRunTime: c.runTime.trim(),
  };
}
