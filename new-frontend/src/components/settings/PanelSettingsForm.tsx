"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { AllSetting } from '@/types/settings';

// Define inputStyles locally
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-6 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";


interface PanelSettingsFormProps {
  initialSettings: Partial<AllSetting>;
  onSave: (updatedSettings: Partial<AllSetting>) => Promise<void>;
  isLoading: boolean;
  formError?: string | null;
}

const PanelSettingsForm: React.FC<PanelSettingsFormProps> = ({ initialSettings, onSave, isLoading, formError }) => {
  const [webListen, setWebListen] = useState('');
  const [webDomain, setWebDomain] = useState('');
  const [webPort, setWebPort] = useState<number | string>('');
  const [webCertFile, setWebCertFile] = useState('');
  const [webKeyFile, setWebKeyFile] = useState('');
  const [webBasePath, setWebBasePath] = useState('');
  const [sessionMaxAge, setSessionMaxAge] = useState<number | string>('');
  const [pageSize, setPageSize] = useState<number | string>('');
  const [timeLocation, setTimeLocation] = useState('');
  const [datepicker, setDatepicker] = useState<'gregorian' | 'jalali' | string>('gregorian');

  useEffect(() => {
    setWebListen(initialSettings.webListen || '');
    setWebDomain(initialSettings.webDomain || '');
    setWebPort(initialSettings.webPort || '');
    setWebCertFile(initialSettings.webCertFile || '');
    setWebKeyFile(initialSettings.webKeyFile || '');
    setWebBasePath(initialSettings.webBasePath || '');
    setSessionMaxAge(initialSettings.sessionMaxAge || '');
    setPageSize(initialSettings.pageSize || 10); // Default page size
    setTimeLocation(initialSettings.timeLocation || 'Asia/Tehran');
    setDatepicker(initialSettings.datepicker || 'gregorian');
  }, [initialSettings]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const updatedPanelSettings: Partial<AllSetting> = {
      // It's crucial to only send fields that this form is responsible for,
      // or ensure the backend handles partial updates correctly without nullifying other settings.
      // For /setting/update, we send the whole AllSettings object usually.
      // So, merge with initialSettings to keep other tabs' data.
      ...initialSettings,
      webListen,
      webDomain,
      webPort: Number(webPort) || undefined,
      webCertFile,
      webKeyFile,
      webBasePath,
      sessionMaxAge: Number(sessionMaxAge) || undefined,
      pageSize: Number(pageSize) || undefined,
      timeLocation,
      datepicker,
    };
    onSave(updatedPanelSettings);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {formError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Web Server Settings</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">
          <div>
            <label htmlFor="webListen" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Listen IP</label>
            <input type="text" id="webListen" value={webListen} onChange={e => setWebListen(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="0.0.0.0 for all interfaces"/>
          </div>
          <div>
            <label htmlFor="webPort" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Listen Port <span className="text-red-500">*</span></label>
            <input type="number" id="webPort" value={webPort} onChange={e => setWebPort(e.target.value === '' ? '' : Number(e.target.value))} required min="1" max="65535" className={`mt-1 w-full ${inputStyles}`}/>
          </div>
          <div>
            <label htmlFor="webDomain" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Panel Domain</label>
            <input type="text" id="webDomain" value={webDomain} onChange={e => setWebDomain(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="e.g., yourdomain.com"/>
          </div>
           <div>
            <label htmlFor="webBasePath" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Base Path</label>
            <input type="text" id="webBasePath" value={webBasePath} onChange={e => setWebBasePath(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="e.g., /xui/"/>
          </div>
          <div>
            <label htmlFor="webCertFile" className="block text-sm font-medium text-gray-700 dark:text-gray-300">SSL Certificate File Path</label>
            <input type="text" id="webCertFile" value={webCertFile} onChange={e => setWebCertFile(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="/path/to/cert.pem"/>
          </div>
          <div>
            <label htmlFor="webKeyFile" className="block text-sm font-medium text-gray-700 dark:text-gray-300">SSL Key File Path</label>
            <input type="text" id="webKeyFile" value={webKeyFile} onChange={e => setWebKeyFile(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="/path/to/key.pem"/>
          </div>
          <div>
            <label htmlFor="sessionMaxAge" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Session Max Age (minutes)</label>
            <input type="number" id="sessionMaxAge" value={sessionMaxAge} onChange={e => setSessionMaxAge(e.target.value === '' ? '' : Number(e.target.value))} min="1" className={`mt-1 w-full ${inputStyles}`}/>
          </div>
        </div>
      </fieldset>

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Panel UI & General</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">
          <div>
            <label htmlFor="pageSize" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Items Per Page (Tables)</label>
            <input type="number" id="pageSize" value={pageSize} onChange={e => setPageSize(e.target.value === '' ? '' : Number(e.target.value))} min="1" className={`mt-1 w-full ${inputStyles}`}/>
          </div>
          <div>
            <label htmlFor="timeLocation" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Timezone</label>
            <input type="text" id="timeLocation" value={timeLocation} onChange={e => setTimeLocation(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="e.g., Asia/Tehran or UTC"/>
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Use TZ Database Name (e.g., America/New_York, Europe/London).</p>
          </div>
          <div>
            <label htmlFor="datepicker" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Date Picker Type</label>
            <select id="datepicker" value={datepicker} onChange={e => setDatepicker(e.target.value)} className={`mt-1 w-full ${inputStyles}`}>
              <option value="gregorian">Gregorian</option>
              <option value="jalali">Jalali (Persian)</option>
            </select>
          </div>
        </div>
      </fieldset>

      <div className="flex justify-end pt-4">
        <button type="submit" disabled={isLoading} className={btnPrimaryStyles}>
          {isLoading ? 'Saving...' : 'Save Panel Settings'}
        </button>
      </div>
    </form>
  );
};
export default PanelSettingsForm;
