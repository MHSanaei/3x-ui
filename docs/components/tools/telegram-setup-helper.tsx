'use client';

import type { ReactNode } from 'react';
import { useState } from 'react';
import {
  validateBotToken,
  parseAdminIds,
  validateRunTime,
  telegramApiBase,
  buildBotConfigSummary,
} from '@/lib/xray/telegram';
import { ToolFrame } from './tool-frame';
import { TextField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

function Status({ ok, children }: { ok: boolean; children: ReactNode }) {
  return (
    <p
      className={`text-xs ${ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}
    >
      {children}
    </p>
  );
}

export function TelegramSetupHelper() {
  const [token, setToken] = useState('');
  const [adminIds, setAdminIds] = useState('');
  const [runTime, setRunTime] = useState('@daily');

  const tokenV = validateBotToken(token);
  const idsV = parseAdminIds(adminIds);
  const cronV = validateRunTime(runTime);
  const summary = buildBotConfigSummary({ token, adminIds, runTime });

  const settingsText = [
    `tgBotEnable = true`,
    `tgBotToken  = ${summary.tgBotToken || '<token>'}`,
    `tgBotChatId = ${summary.tgBotChatId || '<admin ids>'}`,
    `tgRunTime   = ${summary.tgRunTime || '@daily'}`,
  ].join('\n');

  function reset() {
    setToken('');
    setAdminIds('');
    setRunTime('@daily');
  }

  return (
    <ToolFrame
      title="Telegram bot setup helper"
      description="Validate your bot token, admin IDs, and report schedule, then copy the panel settings."
      onReset={reset}
    >
      <div className="grid grid-cols-1 gap-4">
        <div>
          <TextField
            label="Bot token (from @BotFather)"
            value={token}
            onChange={setToken}
            placeholder="123456789:AA..."
          />
          {token ? (
            tokenV.valid ? (
              <Status ok>Valid — bot id {tokenV.botId}</Status>
            ) : (
              <Status ok={false}>{tokenV.error}</Status>
            )
          ) : null}
        </div>
        <div>
          <TextField
            label="Admin chat IDs (comma-separated)"
            value={adminIds}
            onChange={setAdminIds}
            placeholder="111111111, 222222222"
          />
          {adminIds ? (
            idsV.invalid.length > 0 ? (
              <Status ok={false}>Not numeric: {idsV.invalid.join(', ')}</Status>
            ) : (
              <Status ok>
                {idsV.ids.length} admin id{idsV.ids.length === 1 ? '' : 's'}
              </Status>
            )
          ) : null}
        </div>
        <div>
          <TextField
            label="Report schedule (tgRunTime)"
            value={runTime}
            onChange={setRunTime}
            placeholder="@daily, @every 8h, or a cron expression"
          />
          {runTime ? (
            cronV.valid ? (
              <Status ok>Valid ({cronV.kind})</Status>
            ) : (
              <Status ok={false}>{cronV.error}</Status>
            )
          ) : null}
        </div>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4">
        <OutputBlock label="Panel settings" value={settingsText} />
        <OutputBlock
          label="Bot API base (keep secret)"
          value={tokenV.valid ? telegramApiBase(token) : 'https://api.telegram.org/bot<token>'}
        />
      </div>
    </ToolFrame>
  );
}
