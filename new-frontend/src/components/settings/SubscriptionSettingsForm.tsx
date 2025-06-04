"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { AllSetting } from '@/types/settings';

// Define styles locally
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-6 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";

interface SubscriptionSettingsFormProps {
  initialSettings: Partial<AllSetting>;
  onSave: (updatedSettings: Partial<AllSetting>) => Promise<void>;
  isLoading: boolean;
  formError?: string | null;
}

const SubscriptionSettingsForm: React.FC<SubscriptionSettingsFormProps> = ({
  initialSettings, onSave, isLoading, formError
}) => {
  const [subEnable, setSubEnable] = useState(false);
  const [subTitle, setSubTitle] = useState('');
  const [subListen, setSubListen] = useState('');
  const [subPort, setSubPort] = useState<number | string>('');
  const [subPath, setSubPath] = useState('');
  const [subDomain, setSubDomain] = useState('');
  const [subCertFile, setSubCertFile] = useState('');
  const [subKeyFile, setSubKeyFile] = useState('');
  const [subUpdates, setSubUpdates] = useState<number | string>('');
  const [subEncrypt, setSubEncrypt] = useState(false);
  const [subShowInfo, setSubShowInfo] = useState(false);

  const [subURI, setSubURI] = useState('');
  const [subJsonPath, setSubJsonPath] = useState('');
  const [subJsonURI, setSubJsonURI] = useState('');
  const [subJsonFragment, setSubJsonFragment] = useState('');
  const [subJsonNoises, setSubJsonNoises] = useState('');
  const [subJsonMux, setSubJsonMux] = useState('');
  const [subJsonRules, setSubJsonRules] = useState('');

  useEffect(() => {
    setSubEnable(initialSettings.subEnable || false);
    setSubTitle(initialSettings.subTitle || '');
    setSubListen(initialSettings.subListen || '');
    setSubPort(initialSettings.subPort === undefined ? '' : initialSettings.subPort);
    setSubPath(initialSettings.subPath || '');
    setSubDomain(initialSettings.subDomain || '');
    setSubCertFile(initialSettings.subCertFile || '');
    setSubKeyFile(initialSettings.subKeyFile || '');
    setSubUpdates(initialSettings.subUpdates === undefined ? '' : initialSettings.subUpdates);
    setSubEncrypt(initialSettings.subEncrypt || false);
    setSubShowInfo(initialSettings.subShowInfo || false);

    setSubURI(initialSettings.subURI || '');
    setSubJsonPath(initialSettings.subJsonPath || '');
    setSubJsonURI(initialSettings.subJsonURI || '');
    setSubJsonFragment(initialSettings.subJsonFragment || '');
    setSubJsonNoises(initialSettings.subJsonNoises || '');
    setSubJsonMux(initialSettings.subJsonMux || '');
    setSubJsonRules(initialSettings.subJsonRules || '');
  }, [initialSettings]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const updatedSubSettings: Partial<AllSetting> = {
      ...initialSettings,
      subEnable, subTitle, subListen,
      subPort: Number(subPort) || undefined,
      subPath, subDomain, subCertFile, subKeyFile,
      subUpdates: Number(subUpdates) || undefined,
      subEncrypt, subShowInfo,
      subURI, subJsonPath, subJsonURI, subJsonFragment,
      subJsonNoises, subJsonMux, subJsonRules,
    };
    onSave(updatedSubSettings);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 p-6">
      {formError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Subscription Link Settings</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">
          <div className="md:col-span-2 flex items-center space-x-3">
            <button
              type="button"
              onClick={() => setSubEnable(!subEnable)}
              className={`${subEnable ? 'bg-primary-600' : 'bg-gray-300 dark:bg-gray-600'} relative inline-flex items-center h-6 rounded-full w-11 transition-colors focus:outline-none`}
            >
              <span className="sr-only">Toggle Subscription Link</span>
              <span className={`${subEnable ? 'translate-x-6' : 'translate-x-1'} inline-block w-4 h-4 transform bg-white rounded-full transition-transform`} />
            </button>
            <label htmlFor="subEnable" className="text-sm font-medium text-gray-700 dark:text-gray-300">Enable Subscription Link Server</label>
          </div>

          <div>
            <label htmlFor="subTitle" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Title</label>
            <input type="text" id="subTitle" value={subTitle} onChange={e => setSubTitle(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable}/>
          </div>
          <div>
            <label htmlFor="subDomain" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Domain</label>
            <input type="text" id="subDomain" value={subDomain} onChange={e => setSubDomain(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="e.g., sub.yourdomain.com"/>
          </div>
          <div>
            <label htmlFor="subListen" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Listen IP</label>
            <input type="text" id="subListen" value={subListen} onChange={e => setSubListen(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="Leave blank for panel IP"/>
          </div>
          <div>
            <label htmlFor="subPort" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Port</label>
            <input type="number" id="subPort" value={subPort} onChange={e => setSubPort(e.target.value === '' ? '' : Number(e.target.value))} min="1" max="65535" className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="Leave blank for panel port"/>
          </div>
          <div>
            <label htmlFor="subPath" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Path</label>
            <input type="text" id="subPath" value={subPath} onChange={e => setSubPath(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="e.g., /subscribe"/>
          </div>
          <div>
            <label htmlFor="subUpdates" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Update Interval (hours)</label>
            <input type="number" id="subUpdates" value={subUpdates} onChange={e => setSubUpdates(e.target.value === '' ? '' : Number(e.target.value))} min="1" className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable}/>
          </div>
          <div>
            <label htmlFor="subCertFile" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription SSL Cert Path</label>
            <input type="text" id="subCertFile" value={subCertFile} onChange={e => setSubCertFile(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="/path/to/sub_cert.pem"/>
          </div>
          <div>
            <label htmlFor="subKeyFile" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription SSL Key Path</label>
            <input type="text" id="subKeyFile" value={subKeyFile} onChange={e => setSubKeyFile(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="/path/to/sub_key.pem"/>
          </div>
          <div className="md:col-span-2 grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="flex items-center space-x-3">
                <input type="checkbox" id="subEncrypt" checked={subEncrypt} onChange={e => setSubEncrypt(e.target.checked)} disabled={!subEnable} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-600 rounded focus:ring-primary-500 bg-white dark:bg-gray-700"/>
                <label htmlFor="subEncrypt" className="text-sm font-medium text-gray-700 dark:text-gray-300">Encrypt Subscription</label>
            </div>
            <div className="flex items-center space-x-3">
                <input type="checkbox" id="subShowInfo" checked={subShowInfo} onChange={e => setSubShowInfo(e.target.checked)} disabled={!subEnable} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-600 rounded focus:ring-primary-500 bg-white dark:bg-gray-700"/>
                <label htmlFor="subShowInfo" className="text-sm font-medium text-gray-700 dark:text-gray-300">Show More Info in Subscription</label>
            </div>
          </div>
        </div>
      </fieldset>

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Advanced Subscription JSON Settings</legend>
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">These settings are for advanced customization of subscription links, especially for specific client compatibility.</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">
            <div><label htmlFor="subURI" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Subscription URI (override)</label><input type="text" id="subURI" value={subURI} onChange={e => setSubURI(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable}/></div>
            <div><label htmlFor="subJsonPath" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Subscription Path</label><input type="text" id="subJsonPath" value={subJsonPath} onChange={e => setSubJsonPath(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable}/></div>
            <div><label htmlFor="subJsonURI" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Subscription URI (override)</label><input type="text" id="subJsonURI" value={subJsonURI} onChange={e => setSubJsonURI(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable}/></div>
            <div><label htmlFor="subJsonFragment" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Fragment Mode</label><input type="text" id="subJsonFragment" value={subJsonFragment} onChange={e => setSubJsonFragment(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="e.g., true/false or specific mode"/></div>
            <div><label htmlFor="subJsonNoises" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Noises</label><input type="text" id="subJsonNoises" value={subJsonNoises} onChange={e => setSubJsonNoises(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="e.g., 5,10"/></div>
            <div><label htmlFor="subJsonMux" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Mux (override)</label><input type="text" id="subJsonMux" value={subJsonMux} onChange={e => setSubJsonMux(e.target.value)} className={`mt-1 w-full ${inputStyles}`} disabled={!subEnable} placeholder="e.g., true/false"/></div>
            <div className="md:col-span-2"><label htmlFor="subJsonRules" className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Rules (JSON string)</label><textarea id="subJsonRules" value={subJsonRules} onChange={e => setSubJsonRules(e.target.value)} rows={3} className={`mt-1 w-full font-mono text-sm ${inputStyles}`} disabled={!subEnable}/></div>
        </div>
      </fieldset>

      <div className="flex justify-end pt-4">
        <button type="submit" disabled={isLoading} className={btnPrimaryStyles}>
          {isLoading ? 'Saving...' : 'Save Subscription Settings'}
        </button>
      </div>
    </form>
  );
};
export default SubscriptionSettingsForm;
