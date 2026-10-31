export const LOG_RANGE_PRESETS = ['all', '15m', '1h', '6h', '24h', '7d', 'custom'] as const;
export type LogRangePreset = (typeof LOG_RANGE_PRESETS)[number];

export const LOG_LEVELS = ['all', 'error', 'warning', 'info', 'debug'] as const;
export type LogLevelFilter = (typeof LOG_LEVELS)[number];

export const LOG_TEXT_FIELDS = [
  'ip',
  'country',
  'user',
  'host',
  'inbound',
  'protocol',
  'status',
] as const;
export type LogTextField = (typeof LOG_TEXT_FIELDS)[number];

export interface LogFilters extends Record<LogTextField, string> {
  query: string;
  level: LogLevelFilter;
  range: LogRangePreset;
  since?: number;
  until?: number;
}

export const EMPTY_LOG_FILTERS: LogFilters = {
  query: '',
  ip: '',
  country: '',
  user: '',
  host: '',
  inbound: '',
  protocol: '',
  status: '',
  level: 'all',
  range: 'all',
};

const MINUTE = 60_000;
const RANGE_MILLIS: Partial<Record<LogRangePreset, number>> = {
  '15m': 15 * MINUTE,
  '1h': 60 * MINUTE,
  '6h': 6 * 60 * MINUTE,
  '24h': 24 * 60 * MINUTE,
  '7d': 7 * 24 * 60 * MINUTE,
};

export function hasActiveLogFilters(filters: LogFilters): boolean {
  return (
    filters.level !== 'all' ||
    filters.range !== 'all' ||
    filters.query.trim() !== '' ||
    LOG_TEXT_FIELDS.some((field) => filters[field].trim() !== '')
  );
}

// A preset resolves at request time, so an auto-refreshing "last hour" keeps sliding forward.
export function logQueryParams(
  file: string,
  filters: LogFilters,
  now: number,
): Record<string, string | number> {
  const params: Record<string, string | number> = { file };
  for (const key of ['query', ...LOG_TEXT_FIELDS] as const) {
    const term = filters[key].trim();
    if (term) params[key] = term;
  }
  if (filters.level !== 'all') params.levels = filters.level;
  const preset = RANGE_MILLIS[filters.range];
  if (preset) {
    params.since = now - preset;
  } else if (filters.range === 'custom') {
    if (filters.since) params.since = filters.since;
    if (filters.until) params.until = filters.until;
  }
  return params;
}

export interface HighlightPart {
  text: string;
  match: boolean;
}

export function splitHighlights(text: string, terms: readonly string[]): HighlightPart[] {
  const needles = terms
    .map((term) => term.trim())
    .filter(Boolean)
    .sort((a, b) => b.length - a.length)
    .map((term) => term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  if (needles.length === 0 || text === '') return [{ text, match: false }];

  const parts: HighlightPart[] = [];
  let cursor = 0;
  for (const found of text.matchAll(new RegExp(needles.join('|'), 'giu'))) {
    if (found.index > cursor) parts.push({ text: text.slice(cursor, found.index), match: false });
    parts.push({ text: found[0], match: true });
    cursor = found.index + found[0].length;
  }
  if (cursor < text.length) parts.push({ text: text.slice(cursor), match: false });
  return parts;
}

const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

// A label only repeats what the axis's span leaves ambiguous: the time alone within a day.
export function formatBucketLabel(
  millis: number,
  stepMillis: number,
  spanMillis: number,
  locale: string,
): string {
  const options: Intl.DateTimeFormatOptions =
    stepMillis >= DAY
      ? { month: '2-digit', day: '2-digit' }
      : spanMillis < DAY
        ? { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }
        : {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            hourCycle: 'h23',
          };
  return new Intl.DateTimeFormat(locale, options).format(millis);
}
