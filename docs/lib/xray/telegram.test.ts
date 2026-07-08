import { describe, it, expect } from 'vitest';
import {
  validateBotToken,
  parseAdminIds,
  validateRunTime,
  telegramApiBase,
  renderMessageTemplate,
  buildBotConfigSummary,
} from './telegram';

const VALID_TOKEN = '123456789:AAH-abcdefghijklmnopqrstuvwxyz0123456';

describe('validateBotToken', () => {
  it('accepts a BotFather-format token and extracts the bot id', () => {
    const r = validateBotToken(VALID_TOKEN);
    expect(r.valid).toBe(true);
    expect(r.botId).toBe('123456789');
  });

  it('rejects a token with no colon', () => {
    expect(validateBotToken('123456789AAH').valid).toBe(false);
  });

  it('rejects a too-short secret', () => {
    expect(validateBotToken('123:short').valid).toBe(false);
  });

  it('rejects an empty token', () => {
    expect(validateBotToken('   ').valid).toBe(false);
  });
});

describe('parseAdminIds', () => {
  it('splits a comma list into integer ids', () => {
    expect(parseAdminIds('111, 222')).toEqual({ ids: [111, 222], invalid: [] });
  });

  it('accepts negative group ids and captures invalid entries', () => {
    expect(parseAdminIds('-1001234567, abc, 42')).toEqual({ ids: [-1001234567, 42], invalid: ['abc'] });
  });

  it('returns empty for blank input', () => {
    expect(parseAdminIds('  ')).toEqual({ ids: [], invalid: [] });
  });
});

describe('validateRunTime', () => {
  it('accepts the @daily macro', () => {
    expect(validateRunTime('@daily')).toMatchObject({ valid: true, kind: 'macro' });
  });

  it('accepts an @every duration', () => {
    expect(validateRunTime('@every 8h')).toMatchObject({ valid: true, kind: 'every' });
  });

  it('accepts a 5-field cron', () => {
    expect(validateRunTime('0 0 * * *')).toMatchObject({ valid: true, kind: 'cron' });
  });

  it('accepts a 6-field cron', () => {
    expect(validateRunTime('*/30 * * * * *')).toMatchObject({ valid: true, kind: 'cron' });
  });

  it('rejects an unknown macro', () => {
    expect(validateRunTime('@bogus').valid).toBe(false);
  });

  it('rejects a malformed @every duration', () => {
    expect(validateRunTime('@every 8x').valid).toBe(false);
  });
});

describe('telegramApiBase', () => {
  it('builds the bot API base URL', () => {
    expect(telegramApiBase(VALID_TOKEN)).toBe(`https://api.telegram.org/bot${VALID_TOKEN}`);
  });
});

describe('renderMessageTemplate', () => {
  it('substitutes known variables', () => {
    expect(renderMessageTemplate('Host {{host}} up {{uptime}}', { host: 'srv', uptime: '3d' })).toBe(
      'Host srv up 3d',
    );
  });

  it('leaves unknown variables literal', () => {
    expect(renderMessageTemplate('{{a}} {{b}}', { a: 'x' })).toBe('x {{b}}');
  });
});

describe('buildBotConfigSummary', () => {
  it('emits the panel settings keys with admin ids joined', () => {
    const s = buildBotConfigSummary({ token: VALID_TOKEN, adminIds: '111, 222', runTime: '@daily' });
    expect(s.tgBotEnable).toBe(true);
    expect(s.tgBotToken).toBe(VALID_TOKEN);
    expect(s.tgBotChatId).toBe('111,222');
    expect(s.tgRunTime).toBe('@daily');
  });
});
