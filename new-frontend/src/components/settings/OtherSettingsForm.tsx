"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { AllSetting } from '@/types/settings';

// Define styles locally
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-6 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";

interface OtherSettingsFormProps {
  initialSettings: Partial<AllSetting>;
  onSave: (updatedSettings: Partial<AllSetting>) => Promise<void>;
  isLoading: boolean;
  formError?: string | null;
}

const OtherSettingsForm: React.FC<OtherSettingsFormProps> = ({
  initialSettings, onSave, isLoading, formError
}) => {
  const [remarkModel, setRemarkModel] = useState('');
  const [expireDiff, setExpireDiff] = useState<number | string>('');
  const [trafficDiff, setTrafficDiff] = useState<number | string>('');
  const [externalTrafficInformEnable, setExternalTrafficInformEnable] = useState(false);
  const [externalTrafficInformURI, setExternalTrafficInformURI] = useState('');

  useEffect(() => {
    setRemarkModel(initialSettings.remarkModel || '');
    setExpireDiff(initialSettings.expireDiff === undefined ? '' : initialSettings.expireDiff);
    setTrafficDiff(initialSettings.trafficDiff === undefined ? '' : initialSettings.trafficDiff);
    setExternalTrafficInformEnable(initialSettings.externalTrafficInformEnable || false);
    setExternalTrafficInformURI(initialSettings.externalTrafficInformURI || '');
  }, [initialSettings]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const updatedOtherSettings: Partial<AllSetting> = {
      ...initialSettings,
      remarkModel,
      expireDiff: Number(expireDiff) || 0,
      trafficDiff: Number(trafficDiff) || 0,
      externalTrafficInformEnable,
      externalTrafficInformURI,
    };
    onSave(updatedOtherSettings);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 p-6">
      {formError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Miscellaneous Settings</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-2">

          <div className="md:col-span-2">
            <label htmlFor="remarkModel" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Remark Template Model</label>
            <input
              type="text"
              id="remarkModel"
              value={remarkModel}
              onChange={e => setRemarkModel(e.target.value)}
              className={`mt-1 w-full ${inputStyles}`}
              placeholder="e.g., {protocol}-{port}-{id}"
            />
            <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Variables: { "{protocol}, {port}, {id}, {email}, {rand(int)}" }, etc.</p>
          </div>

          <div>
            <label htmlFor="expireDiff" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Expire Notification Threshold (days)</label>
            <input
              type="number"
              id="expireDiff"
              value={expireDiff}
              onChange={e => setExpireDiff(e.target.value === '' ? '' : Number(e.target.value))}
              min="0"
              className={`mt-1 w-full ${inputStyles}`}
              placeholder="e.g., 7"
            />
          </div>
          <div>
            <label htmlFor="trafficDiff" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Traffic Notification Threshold (GB)</label>
            <input
              type="number"
              id="trafficDiff"
              value={trafficDiff}
              onChange={e => setTrafficDiff(e.target.value === '' ? '' : Number(e.target.value))}
              min="0"
              className={`mt-1 w-full ${inputStyles}`}
              placeholder="e.g., 5"
            />
             <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">Unit is assumed to be GB by panel. Backend handles interpretation.</p>
          </div>

          <div className="md:col-span-2 flex items-center space-x-3 mt-2">
            <button
              type="button"
              onClick={() => setExternalTrafficInformEnable(!externalTrafficInformEnable)}
              className={`${externalTrafficInformEnable ? 'bg-primary-600' : 'bg-gray-300 dark:bg-gray-600'} relative inline-flex items-center h-6 rounded-full w-11 transition-colors focus:outline-none`}
            >
              <span className="sr-only">Toggle External Traffic Inform</span>
              <span className={`${externalTrafficInformEnable ? 'translate-x-6' : 'translate-x-1'} inline-block w-4 h-4 transform bg-white rounded-full transition-transform`} />
            </button>
            <label htmlFor="externalTrafficInformEnable" className="text-sm font-medium text-gray-700 dark:text-gray-300">Enable External Traffic Informer</label>
          </div>

          <div className="md:col-span-2">
            <label htmlFor="externalTrafficInformURI" className="block text-sm font-medium text-gray-700 dark:text-gray-300">External Traffic Informer URI</label>
            <input
              type="text"
              id="externalTrafficInformURI"
              value={externalTrafficInformURI}
              onChange={e => setExternalTrafficInformURI(e.target.value)}
              className={`mt-1 w-full ${inputStyles}`}
              disabled={!externalTrafficInformEnable}
              placeholder="e.g., http://your-service.com/traffic_update"
            />
          </div>
        </div>
      </fieldset>

      <div className="flex justify-end pt-4">
        <button type="submit" disabled={isLoading} className={btnPrimaryStyles}>
          {isLoading ? 'Saving...' : 'Save Other Settings'}
        </button>
      </div>
    </form>
  );
};
export default OtherSettingsForm;
