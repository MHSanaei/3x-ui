"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { Inbound, Protocol, ClientSetting } from '@/types/inbound';
import { useRouter } from 'next/navigation';
import ProtocolClientSettings from './ProtocolClientSettings';
import StreamSettingsForm from './stream_settings/StreamSettingsForm'; // Import new StreamSettingsForm

const availableProtocols: Protocol[] = ["vmess", "vless", "trojan", "shadowsocks", "dokodemo-door", "socks", "http"];
const availableShadowsocksCiphers: string[] = [
  "aes-256-gcm", "aes-128-gcm", "chacha20-poly1305", "xchacha20-poly1305",
  "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "none"
];
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";

interface InboundFormProps {
  initialData?: Partial<Inbound>;
  isEditMode?: boolean;
  formLoading?: boolean;
  onSubmitForm: (inboundData: Partial<Inbound>) => Promise<void>;
}

const InboundForm: React.FC<InboundFormProps> = ({ initialData, isEditMode = false, formLoading, onSubmitForm }) => {
  const router = useRouter();
  const [remark, setRemark] = useState('');
  const [listen, setListen] = useState('');
  const [port, setPort] = useState<number | string>('');
  const [protocol, setProtocol] = useState<Protocol | ''>('');
  const [enable, setEnable] = useState(true);
  const [expiryTime, setExpiryTime] = useState<number | string>(0);
  const [total, setTotal] = useState<number | string>(0);

  const [clientList, setClientList] = useState<ClientSetting[]>([]);
  const [ssMethod, setSsMethod] = useState(availableShadowsocksCiphers[0]);
  const [ssPassword, setSsPassword] = useState('');

  const [settingsJson, setSettingsJson] = useState('{}');
  const [streamSettingsJson, setStreamSettingsJson] = useState('{}');
  const [sniffingJson, setSniffingJson] = useState('{}');

  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (initialData) {
      const currentProtocol = (initialData.protocol || '') as Protocol;
      setRemark(initialData.remark || '');
      setListen(initialData.listen || '');
      setPort(initialData.port || '');
      setProtocol(currentProtocol);
      setEnable(initialData.enable !== undefined ? initialData.enable : true);
      setExpiryTime(initialData.expiryTime || 0);
      setTotal(initialData.total && initialData.total > 0 ? initialData.total / (1024 * 1024 * 1024) : 0);

      setStreamSettingsJson(initialData.streamSettings || '{}');
      setSniffingJson(initialData.sniffing || '{}');

      const initialSettings = initialData.settings || '{}';
      setSettingsJson(initialSettings); // Initialize settingsJson with all settings

      if ((currentProtocol === 'vmess' || currentProtocol === 'vless' || currentProtocol === 'trojan')) {
        try {
          const parsedSettings = JSON.parse(initialSettings);
          setClientList(parsedSettings.clients || []);
        } catch (e) { console.error(`Error parsing settings for ${currentProtocol}:`, e); setClientList([]); }
      } else if (currentProtocol === 'shadowsocks') {
        try {
          const parsedSettings = JSON.parse(initialSettings);
          setSsMethod(parsedSettings.method || availableShadowsocksCiphers[0]);
          setSsPassword(parsedSettings.password || '');
        } catch (e) { console.error("Error parsing Shadowsocks settings:", e); }
      } else {
        // For other protocols, specific UI states are not used for settings
        setClientList([]);
        setSsMethod(availableShadowsocksCiphers[0]); setSsPassword('');
      }
    } else {
      // Reset all fields for a new form
      setRemark(''); setListen(''); setPort(''); setProtocol(''); setEnable(true);
      setExpiryTime(0); setTotal(0); setClientList([]);
      setSsMethod(availableShadowsocksCiphers[0]); setSsPassword('');
      setSettingsJson('{}'); setStreamSettingsJson('{}'); setSniffingJson('{}');
    }
  }, [initialData]);

  // This useEffect updates settingsJson based on UI changes for client lists or SS settings
  useEffect(() => {
    if (protocol === 'vmess' || protocol === 'vless' || protocol === 'trojan' || protocol === 'shadowsocks') {
      let baseSettings: Record<string, unknown> = {};
      try {
        baseSettings = JSON.parse(settingsJson) || {}; // Preserve other settings
      } catch { /* If settingsJson is invalid, it will be overwritten */ }

      const newSettingsObject = { ...baseSettings }; // Changed to const

      if (protocol === 'vmess' || protocol === 'vless' || protocol === 'trojan') {
          delete newSettingsObject.method; delete newSettingsObject.password;
          newSettingsObject.clients = clientList;
      } else if (protocol === 'shadowsocks') {
          delete newSettingsObject.clients;
          newSettingsObject.method = ssMethod;
          newSettingsObject.password = ssPassword;
      }

      try {
          const finalJson = JSON.stringify(newSettingsObject, null, 2);
          if (finalJson !== settingsJson) { // Only update if there's an actual change
               setSettingsJson(finalJson);
          }
      } catch (e) { console.error("Error stringifying settings:", e); }
    }
    // No 'else' needed: for other protocols, settingsJson is managed by its textarea directly.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clientList, ssMethod, ssPassword, protocol]); // settingsJson is NOT a dependency here.


  const handleProtocolChange = (newProtocol: Protocol) => {
    setProtocol(newProtocol);
    setFormError(null);

    // When protocol changes, re-initialize specific UI states from current settingsJson
    let currentSettings: Record<string, unknown> = {}; // Changed any to unknown
    try { currentSettings = JSON.parse(settingsJson || '{}'); } catch {}

    if (newProtocol === 'vmess' || newProtocol === 'vless' || newProtocol === 'trojan') {
        if (Array.isArray(currentSettings.clients)) {
            setClientList(currentSettings.clients as ClientSetting[]);
        } else {
            setClientList([]);
        }
        setSsMethod(availableShadowsocksCiphers[0]); setSsPassword('');
    } else if (newProtocol === 'shadowsocks') {
        setClientList([]);
        if (typeof currentSettings.method === 'string') {
            setSsMethod(currentSettings.method);
        } else {
            setSsMethod(availableShadowsocksCiphers[0]);
        }
        if (typeof currentSettings.password === 'string') {
            setSsPassword(currentSettings.password);
        } else {
            setSsPassword('');
        }
    } else {
        // For "other" protocols, clear all specific UI states
        setClientList([]);
        setSsMethod(availableShadowsocksCiphers[0]); setSsPassword('');
        // settingsJson remains as is, for manual editing
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault(); setFormError(null);
    if (!protocol) { setFormError("Protocol is required."); return; }
    if (port === '' || Number(port) <= 0 || Number(port) > 65535) { setFormError("Valid Port (1-65535) is required."); return; }

    // settingsJson should already be up-to-date from the useEffect hook
    // for vmess/vless/trojan/shadowsocks.
    // For other protocols, it's taken directly from its textarea.
    if (protocol === 'shadowsocks' && !ssPassword) {
         setFormError("Password is required for Shadowsocks."); return;
    }

    // Validate all JSON fields before submitting
    for (const [fieldName, jsonStr] of Object.entries({ settings: settingsJson, streamSettings: streamSettingsJson, sniffing: sniffingJson })) {
        try { JSON.parse(jsonStr); } catch (err) {
            setFormError(`Invalid JSON in ${fieldName === 'settings' ? 'protocol settings' : fieldName}: ${(err as Error).message}`); return;
        }
    }

    const inboundData: Partial<Inbound> = {
      remark, listen, port: Number(port), protocol, enable,
      expiryTime: Number(expiryTime), total: Number(total) * 1024 * 1024 * 1024,
      settings: settingsJson, streamSettings: streamSettingsJson, sniffing: sniffingJson,
    };

    if (isEditMode && initialData?.id) {
        inboundData.id = initialData.id;
        inboundData.up = initialData.up; inboundData.down = initialData.down;
    }
    await onSubmitForm(inboundData);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 bg-white dark:bg-gray-800 p-4 md:p-6 rounded-lg shadow">
      {formError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Basic Settings</legend>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-2">
          <div><label htmlFor="remark" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Remark</label><input type="text" id="remark" value={remark} onChange={(e) => setRemark(e.target.value)} className={inputStyles} /></div>
          <div><label htmlFor="protocol" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Protocol <span className="text-red-500">*</span></label><select id="protocol" value={protocol} onChange={(e) => handleProtocolChange(e.target.value as Protocol)} required className={inputStyles}><option value="" disabled>Select...</option>{availableProtocols.map(p => <option key={p} value={p}>{p}</option>)}</select></div>
          <div><label htmlFor="listen" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Listen IP</label><input type="text" id="listen" value={listen} onChange={(e) => setListen(e.target.value)} placeholder="Default: 0.0.0.0" className={inputStyles} /></div>
          <div><label htmlFor="port" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Port <span className="text-red-500">*</span></label><input type="number" id="port" value={port} onChange={(e) => setPort(e.target.value === '' ? '' : Number(e.target.value))} required min="1" max="65535" className={inputStyles} /></div>
          <div><label htmlFor="total" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Quota (GB)</label><input type="number" id="total" value={total} onChange={(e) => setTotal(e.target.value === '' ? '' : Number(e.target.value))} min="0" placeholder="0 for unlimited" className={inputStyles} /></div>
          <div><label htmlFor="expiryTime" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Expiry Time (ms)</label><input type="number" id="expiryTime" value={expiryTime} onChange={(e) => setExpiryTime(e.target.value === '' ? '' : Number(e.target.value))} min="0" placeholder="0 for never" className={inputStyles} /></div>
          <div className="flex items-center space-x-2 md:col-span-2"><input type="checkbox" id="enable" checked={enable} onChange={(e) => setEnable(e.target.checked)} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-500 rounded focus:ring-primary-500 bg-white dark:bg-gray-700" /><label htmlFor="enable" className="text-sm font-medium text-gray-700 dark:text-gray-300">Enable</label></div>
        </div>
      </fieldset>

      {protocol && (
        <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
          <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">{protocol} Settings</legend>
          {(protocol === 'vmess' || protocol === 'vless' || protocol === 'trojan') ? (
            <ProtocolClientSettings clients={clientList} onChange={setClientList} protocol={protocol} />
          ) : protocol === 'shadowsocks' ? (
            <div className="space-y-3 mt-2">
              <div><label htmlFor="ssMethod" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Cipher <span className="text-red-500">*</span></label><select id="ssMethod" value={ssMethod} onChange={(e) => setSsMethod(e.target.value)} required className={inputStyles}>{availableShadowsocksCiphers.map(c => <option key={c} value={c}>{c}</option>)}</select></div>
              <div><label htmlFor="ssPassword" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Password <span className="text-red-500">*</span></label><input type="text" id="ssPassword" value={ssPassword} onChange={(e) => setSsPassword(e.target.value)} required className={inputStyles} /></div>
            </div>
          ) : (
            <textarea value={settingsJson} onChange={(e) => setSettingsJson(e.target.value)} rows={8} className={inputStyles + " font-mono text-sm"} placeholder={`Enter JSON for '${protocol}' settings`}/>
          )}
        </fieldset>
      )}

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Stream Settings</legend>
        <StreamSettingsForm initialStreamSettingsJson={streamSettingsJson} onChange={setStreamSettingsJson} />
      </fieldset>

      <fieldset className="border border-gray-300 dark:border-gray-600 p-4 rounded-md">
        <legend className="text-lg font-medium text-primary-600 dark:text-primary-400 px-2">Sniffing Settings (JSON)</legend>
        <textarea value={sniffingJson} onChange={(e) => setSniffingJson(e.target.value)} rows={4} className={inputStyles + " font-mono text-sm"} />
      </fieldset>

      <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
        <button type="button" onClick={() => router.back()}
                className="px-4 py-2 border border-gray-300 dark:border-gray-500 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-primary-500 dark:focus:ring-offset-gray-800">
                Cancel
        </button>
        <button type="submit" disabled={formLoading}
                className="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-primary-500 dark:focus:ring-offset-gray-800 disabled:opacity-50">
          {formLoading ? (isEditMode ? 'Saving...' : 'Creating...') : (isEditMode ? 'Save Changes' : 'Create Inbound')}
        </button>
      </div>
    </form>
  );
};
export default InboundForm;
