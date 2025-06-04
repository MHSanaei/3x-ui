"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { AllSetting } from '@/types/settings';

// Define styles locally
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-6 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";

interface TelegramSettingsFormProps {
  initialSettings: Partial<AllSetting>;
  onSave: (updatedSettings: Partial<AllSetting>) => Promise<void>;
  isLoading: boolean;
  formError?: string | null;
}

const availableTgLangs = ["en", "fa", "ru", "zh-cn"]; // Example languages

const TelegramSettingsForm: React.FC<TelegramSettingsFormProps> = ({
  initialSettings, onSave, isLoading, formError
}) => {
  const [tgBotEnable, setTgBotEnable] = useState(false);
  const [tgBotToken, setTgBotToken] = useState('');
  const [tgBotProxy, setTgBotProxy] = useState('');
  const [tgBotAPIServer, setTgBotAPIServer] = useState('');
  const [tgBotChatId, setTgBotChatId] = useState('');
  const [tgRunTime, setTgRunTime] = useState('');
  const [tgBotBackup, setTgBotBackup] = useState(false);
  const [tgBotLoginNotify, setTgBotLoginNotify] = useState(false);
  const [tgCpu, setTgCpu] = useState<number | string>('');
  const [tgLang, setTgLang] = useState('en');

  useEffect(() => {
    setTgBotEnable(initialSettings.tgBotEnable || false);
    setTgBotToken(initialSettings.tgBotToken || '');
    setTgBotProxy(initialSettings.tgBotProxy || '');
    setTgBotAPIServer(initialSettings.tgBotAPIServer || '');
    setTgBotChatId(initialSettings.tgBotChatId || '');
    setTgRunTime(initialSettings.tgRunTime || '');
    setTgBotBackup(initialSettings.tgBotBackup || false);
    setTgBotLoginNotify(initialSettings.tgBotLoginNotify || false);
    setTgCpu(initialSettings.tgCpu === undefined ? '' : initialSettings.tgCpu);
    setTgLang(initialSettings.tgLang || 'en');
  }, [initialSettings]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const updatedTgSettings: Partial<AllSetting> = {
      ...initialSettings,
      tgBotEnable,
      tgBotToken,
      tgBotProxy,
      tgBotAPIServer,
      tgBotChatId,
      tgRunTime,
      tgBotBackup,
      tgBotLoginNotify,
      tgCpu: Number(tgCpu) || 0,
      tgLang,
    };
    onSave(updatedTgSettings);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 p-6">
      {formError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Telegram Bot Settings</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">

          <div className="md:col-span-2 flex items-center space-x-3">
            <button
              type="button"
              onClick={() => setTgBotEnable(!tgBotEnable)}
              className={`${tgBotEnable ? 'bg-primary-600' : 'bg-gray-300 dark:bg-gray-600'} relative inline-flex items-center h-6 rounded-full w-11 transition-colors focus:outline-none`}
            >
              <span className="sr-only">Toggle Telegram Bot</span>
              <span className={`${tgBotEnable ? 'translate-x-6' : 'translate-x-1'} inline-block w-4 h-4 transform bg-white rounded-full transition-transform`} />
            </button>
            <label htmlFor="tgBotEnable" className="text-sm font-medium text-gray-700 dark:text-gray-300">Enable Telegram Bot</label>
          </div>

          <div>
            <label htmlFor="tgBotToken" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Bot Token</label>
            <input type="text" id="tgBotToken" value={tgBotToken} onChange={e => setTgBotToken(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable}/>
          </div>
          <div>
            <label htmlFor="tgBotChatId" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Admin Chat ID(s)</label>
            <input type="text" id="tgBotChatId" value={tgBotChatId} onChange={e => setTgBotChatId(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable} placeholder="e.g., 12345,67890"/>
             <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Comma-separated for multiple IDs.</p>
          </div>
          <div>
            <label htmlFor="tgBotProxy" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Bot Proxy URL</label>
            <input type="text" id="tgBotProxy" value={tgBotProxy} onChange={e => setTgBotProxy(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable} placeholder="e.g., socks5://user:pass@host:port"/>
          </div>
           <div>
            <label htmlFor="tgBotAPIServer" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Bot API Server</label>
            <input type="text" id="tgBotAPIServer" value={tgBotAPIServer} onChange={e => setTgBotAPIServer(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable} placeholder="e.g., https://api.telegram.org"/>
          </div>
          <div>
            <label htmlFor="tgRunTime" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Notification Schedule (Cron)</label>
            <input type="text" id="tgRunTime" value={tgRunTime} onChange={e => setTgRunTime(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable} placeholder="e.g., @daily or 0 0 * * *"/>
          </div>
           <div>
            <label htmlFor="tgCpu" className="block text-sm font-medium text-gray-700 dark:text-gray-300">CPU Usage Alert Threshold (%)</label>
            <input type="number" id="tgCpu" value={tgCpu} onChange={e => setTgCpu(e.target.value === '' ? '' : Number(e.target.value))} min="0" max="100" className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable} placeholder="0 to disable"/>
          </div>
           <div>
            <label htmlFor="tgLang" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Bot Language</label>
            <select id="tgLang" value={tgLang} onChange={e => setTgLang(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!tgBotEnable}>
              {availableTgLangs.map(lang => <option key={lang} value={lang}>{lang}</option>)}
            </select>
          </div>
          <div className="md:col-span-2 space-y-2">
            <div className="flex items-center space-x-3">
                <input type="checkbox" id="tgBotLoginNotify" checked={tgBotLoginNotify} onChange={e => setTgBotLoginNotify(e.target.checked)} disabled={!tgBotEnable} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-600 rounded focus:ring-primary-500 bg-white dark:bg-gray-700"/>
                <label htmlFor="tgBotLoginNotify" className="text-sm font-medium text-gray-700 dark:text-gray-300">Login Notification</label>
            </div>
            <div className="flex items-center space-x-3">
                <input type="checkbox" id="tgBotBackup" checked={tgBotBackup} onChange={e => setTgBotBackup(e.target.checked)} disabled={!tgBotEnable} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-600 rounded focus:ring-primary-500 bg-white dark:bg-gray-700"/>
                <label htmlFor="tgBotBackup" className="text-sm font-medium text-gray-700 dark:text-gray-300">Daily Backup via Bot</label>
            </div>
          </div>
        </div>
      </fieldset>

      <div className="flex justify-end pt-4">
        <button type="submit" disabled={isLoading} className={btnPrimaryStyles}>
          {isLoading ? 'Saving...' : 'Save Telegram Settings'}
        </button>
      </div>
    </form>
  );
};
export default TelegramSettingsForm;
