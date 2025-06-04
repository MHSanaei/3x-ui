"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { ClientSetting, Protocol } from '@/types/inbound';

// Basic UUID v4 generator
const generateUUID = () => 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
  const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
  return v.toString(16);
});
const generateRandomPassword = (length = 12) => Math.random().toString(36).substring(2, 2 + length);

const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-4 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";
const btnSecondaryStyles = "px-4 py-2 bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors";


interface ClientFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (client: ClientSetting) => void;
  protocol: Protocol;
  existingClient?: ClientSetting | null;
  formError?: string | null;
  isLoading?: boolean;
}

const ClientFormModal: React.FC<ClientFormModalProps> = ({
  isOpen, onClose, onSubmit, protocol, existingClient = null, formError, isLoading
}) => {
  const isEditMode = !!existingClient;
  const [email, setEmail] = useState('');
  const [identifier, setIdentifier] = useState('');
  const [flow, setFlow] = useState('');
  const [totalGB, setTotalGB] = useState<number | string>(0);
  const [expiryTime, setExpiryTime] = useState<number | string>(0);
  const [limitIp, setLimitIp] = useState<number | string>(0);

  useEffect(() => {
    if (isOpen) { // Only reset/populate when modal becomes visible or critical props change
        if (existingClient) {
            setEmail(existingClient.email || '');
            if (protocol === 'trojan') {
                setIdentifier(existingClient.password || '');
            } else { // vmess, vless
                setIdentifier(existingClient.id || '');
            }
            setFlow(existingClient.flow || '');
            setTotalGB(existingClient.totalGB === undefined ? 0 : existingClient.totalGB);
            setExpiryTime(existingClient.expiryTime === undefined ? 0 : existingClient.expiryTime);
            setLimitIp(existingClient.limitIp === undefined ? 0 : existingClient.limitIp);
        } else {
            setEmail('');
            setIdentifier(protocol === 'trojan' ? generateRandomPassword() : generateUUID());
            setFlow(protocol === 'vless' ? 'xtls-rprx-vision' : '');
            setTotalGB(0);
            setExpiryTime(0);
            setLimitIp(0);
        }
    }
  }, [isOpen, existingClient, protocol]);

  if (!isOpen) return null;

  const identifierLabel = protocol === 'trojan' ? 'Password' : 'UUID';

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const clientData: ClientSetting = {
      email,
      flow: protocol === 'vless' ? flow : undefined,
      totalGB: Number(totalGB),
      expiryTime: Number(expiryTime),
      limitIp: Number(limitIp),
    };
    if (protocol === 'trojan') {
      clientData.password = identifier;
    } else { // vmess, vless
      clientData.id = identifier;
    }
    // If editing, preserve original ID if it was a UUID that shouldn't change
    if (isEditMode && existingClient?.id && protocol !== 'trojan') {
        clientData.id = existingClient.id;
    }
    onSubmit(clientData);
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-xl w-full max-w-md space-y-4">
        <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-100">
          {isEditMode ? 'Edit Client' : 'Add New Client'}
        </h2>
        {formError && <p className="text-sm text-red-500 dark:text-red-400">{formError}</p>}
        <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Email <span className="text-red-500">*</span></label>
            <input type="email" id="email" value={email} onChange={e => setEmail(e.target.value)} required className={`mt-1 w-full ${inputStyles}`} />
          </div>
          <div>
            <label htmlFor="identifier" className="block text-sm font-medium text-gray-700 dark:text-gray-300">{identifierLabel} <span className="text-red-500">*</span></label>
            <div className="flex">
              <input type="text" id="identifier" value={identifier} onChange={e => setIdentifier(e.target.value)} required className={`mt-1 w-full rounded-r-none ${inputStyles}`} />
              <button type="button" onClick={() => setIdentifier(protocol === 'trojan' ? generateRandomPassword() : generateUUID())} className="mt-1 px-3 py-2 border border-l-0 border-gray-300 dark:border-gray-600 rounded-r-md bg-gray-50 dark:bg-gray-700 hover:bg-gray-100 dark:hover:bg-gray-600 text-sm text-gray-700 dark:text-gray-200">
                Generate
              </button>
            </div>
          </div>
          {protocol === 'vless' && (
            <div>
              <label htmlFor="flow" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Flow (VLESS only)</label>
              <input type="text" id="flow" value={flow} onChange={e => setFlow(e.target.value)} className={`mt-1 w-full ${inputStyles}`} placeholder="e.g., xtls-rprx-vision" />
            </div>
          )}
          <div>
            <label htmlFor="totalGB" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Quota (GB, 0 for unlimited)</label>
            <input type="number" id="totalGB" value={totalGB} onChange={e => setTotalGB(e.target.value)} min="0" className={`mt-1 w-full ${inputStyles}`} />
          </div>
          <div>
            <label htmlFor="expiryTime" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Expiry Time (Timestamp ms, 0 for never)</label>
            <input type="number" id="expiryTime" value={expiryTime} onChange={e => setExpiryTime(e.target.value)} min="0" className={`mt-1 w-full ${inputStyles}`} />
          </div>
          <div>
            <label htmlFor="limitIp" className="block text-sm font-medium text-gray-700 dark:text-gray-300">IP Limit (0 for unlimited)</label>
            <input type="number" id="limitIp" value={limitIp} onChange={e => setLimitIp(e.target.value)} min="0" className={`mt-1 w-full ${inputStyles}`} />
          </div>
          <div className="flex justify-end space-x-3 pt-3">
            <button type="button" onClick={onClose} disabled={isLoading} className={btnSecondaryStyles}>Cancel</button>
            <button type="submit" disabled={isLoading} className={btnPrimaryStyles}>
              {isLoading ? (isEditMode ? 'Saving...' : 'Adding...') : (isEditMode ? 'Save Changes' : 'Add Client')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
export default ClientFormModal;
