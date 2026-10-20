// Pure validation + templating helpers for 3x-ui's Telegram bot settings.
// Grounded in internal/web/service/tgbot/tgbot.go (admin ids parsed by
// parseAdminChats as "chatId[:topicId]"; token handed to telego.NewBot → api.telegram.org)
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
  entries: string[]; // normalized "chatId" or "chatId:topicId", as the panel stores them
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
  // Mirrors parseAdminChats in tgbot.go: one topic per chat, repeats collapse.
  const topics = new Map<string, number>();
  const invalid: string[] = [];
  for (const part of raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)) {
    // Telegram chat ids are integers; group/channel ids are negative. An
    // optional ":<topicId>" targets a forum topic inside a supergroup.
    const m = /^(-?\d+)\s*(?::\s*(\d+))?$/.exec(part);
    if (!m || (m[2] !== undefined && Number(m[2]) <= 0)) {
      invalid.push(part);
      continue;
    }
    // General (thread 1) is reached by omitting the topic; the Bot API rejects 1.
    const topic = m[2] === undefined || Number(m[2]) === 1 ? 0 : Number(m[2]);
    const prev = topics.get(m[1]) ?? 0;
    if (topic && prev && prev !== topic) invalid.push(part);
    else topics.set(m[1], topic || prev);
  }
  const ids = [...topics.keys()].map(Number);
  const entries = [...topics].map(([chat, topic]) => (topic ? `${chat}:${topic}` : chat));
  return { ids, entries, invalid };
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
  const { entries } = parseAdminIds(c.adminIds);
  return {
    tgBotEnable: true,
    tgBotToken: c.token.trim(),
    tgBotChatId: entries.join(','),
    tgRunTime: c.runTime.trim(),
  };
}
