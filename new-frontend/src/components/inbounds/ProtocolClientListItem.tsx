"use client";

import React, { useState, useEffect } from 'react';
import { ClientSetting, Protocol } from '@/types/inbound'; // Assuming Protocol is also in types

interface ProtocolClientListItemProps {
  client: ClientSetting;
  index: number;
  onUpdateClient: (index: number, updatedClient: ClientSetting) => void;
  onRemoveClient: (index: number) => void;
  protocol: Protocol; // Now includes 'trojan'
}

// Helper function to apply input styles directly
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";

const ProtocolClientListItem: React.FC<ProtocolClientListItemProps> = ({ client, index, onUpdateClient, onRemoveClient, protocol }) => {
  const [isEditing, setIsEditing] = useState(false);
  const [editableClient, setEditableClient] = useState<ClientSetting>(client);

  useEffect(() => {
    setEditableClient(client);
  }, [client]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value, type } = e.target;
    let processedValue: string | number | boolean = value;
    if (type === 'number') {
      processedValue = value === '' ? '' : Number(value);
    }
    setEditableClient(prev => ({ ...prev, [name]: processedValue }));
  };

  const handleSave = () => {
    onUpdateClient(index, editableClient);
    setIsEditing(false);
  };

  const clientIdentifierField = protocol === 'trojan' ? 'password' : 'id';
  const clientIdentifierLabel = protocol === 'trojan' ? 'Password:' : 'UUID:';

  return (
    <div className="border border-gray-300 dark:border-gray-600 p-3 rounded-md mb-3 space-y-2">
      {isEditing ? (
        <>
          <div>
            <label className="text-xs font-medium text-gray-700 dark:text-gray-300">Email:</label>
            <input type="email" name="email" value={editableClient.email || ''} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} />
          </div>
          <div>
            <label className="text-xs font-medium text-gray-700 dark:text-gray-300">{clientIdentifierLabel}</label>
            <input type="text" name={clientIdentifierField} value={clientIdentifierField === 'id' ? editableClient.id || '' : editableClient.password || ''} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} />
          </div>
          {protocol === 'vless' && ( // Only show flow for VLESS
            <div>
              <label className="text-xs font-medium text-gray-700 dark:text-gray-300">Flow:</label>
              <input type="text" name="flow" value={editableClient.flow || ''} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} placeholder="e.g., xtls-rprx-vision" />
            </div>
          )}
          <div>
            <label className="text-xs font-medium text-gray-700 dark:text-gray-300">Quota (GB):</label>
            <input type="number" name="totalGB" value={editableClient.totalGB === undefined ? '' : editableClient.totalGB} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} placeholder="0 for unlimited" />
          </div>
           <div>
            <label className="text-xs font-medium text-gray-700 dark:text-gray-300">Expiry (Timestamp ms):</label>
            <input type="number" name="expiryTime" value={editableClient.expiryTime === undefined ? '' : editableClient.expiryTime} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} placeholder="0 for never" />
          </div>
           <div>
            <label className="text-xs font-medium text-gray-700 dark:text-gray-300">Limit IP:</label>
            <input type="number" name="limitIp" value={editableClient.limitIp === undefined ? '' : editableClient.limitIp} onChange={handleInputChange} className={`${inputStyles} text-sm p-1`} placeholder="0 for unlimited" />
          </div>
          <div className="flex space-x-2 mt-2">
            <button onClick={handleSave} className="px-2 py-1 text-xs bg-green-500 hover:bg-green-600 text-white rounded">Save</button>
            <button onClick={() => setIsEditing(false)} className="px-2 py-1 text-xs bg-gray-300 hover:bg-gray-400 dark:bg-gray-600 dark:hover:bg-gray-500 rounded">Cancel</button>
          </div>
        </>
      ) : (
        <>
          <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">Email:</span> {client.email}</p>
          <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">{clientIdentifierLabel}</span> {clientIdentifierField === 'id' ? client.id : client.password}</p>
          {client.flow && protocol === 'vless' && <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">Flow:</span> {client.flow}</p>}
          {client.totalGB !== undefined && <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">Quota:</span> {client.totalGB > 0 ? `${client.totalGB} GB` : 'Unlimited'}</p>}
          {client.expiryTime !== undefined && <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">Expiry:</span> {client.expiryTime > 0 ? new Date(client.expiryTime).toLocaleDateString() : 'Never'}</p>}
          {client.limitIp !== undefined && <p className="text-sm"><span className="font-medium text-gray-700 dark:text-gray-300">IP Limit:</span> {client.limitIp > 0 ? client.limitIp : 'Unlimited'}</p>}
          <div className="flex space-x-2 mt-2">
            <button onClick={() => setIsEditing(true)} className="px-2 py-1 text-xs bg-blue-500 hover:bg-blue-600 text-white rounded">Edit</button>
            <button onClick={() => onRemoveClient(index)} className="px-2 py-1 text-xs bg-red-500 hover:bg-red-600 text-white rounded">Remove</button>
          </div>
        </>
      )}
    </div>
  );
};
export default ProtocolClientListItem;
