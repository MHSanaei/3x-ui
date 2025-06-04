"use client";

import React from 'react';
import { ClientSetting, Protocol } from '@/types/inbound';
import ProtocolClientListItem from './ProtocolClientListItem'; // Updated import

const generateRandomId = (length = 8) => Math.random().toString(36).substring(2, 2 + length);
const generateUUID = () => 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
  const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
  return v.toString(16);
});

interface ProtocolClientSettingsProps {
  clients: ClientSetting[];
  onChange: (clients: ClientSetting[]) => void;
  protocol: Protocol; // Now 'vmess', 'vless', or 'trojan'
}

const ProtocolClientSettings: React.FC<ProtocolClientSettingsProps> = ({ clients, onChange, protocol }) => {

  const addClient = () => {
    const newClientBase: Partial<ClientSetting> = {
      email: `user${clients.length + 1}@example.com`,
      totalGB: 0,
      expiryTime: 0,
      limitIp: 0,
    };

    let newClientSpecific: Partial<ClientSetting> = {};
    if (protocol === 'vmess' || protocol === 'vless') {
      newClientSpecific = {
        id: generateUUID(),
        flow: protocol === 'vless' ? 'xtls-rprx-vision' : undefined, // Ensure flow is undefined for vmess
      };
    } else if (protocol === 'trojan') {
      newClientSpecific = {
        password: generateRandomId(12),
      };
    }
    onChange([...clients, { ...newClientBase, ...newClientSpecific } as ClientSetting]);
  };

  const updateClient = (index: number, updatedClient: ClientSetting) => {
    const newClients = [...clients];
    newClients[index] = updatedClient;
    onChange(newClients);
  };

  const removeClient = (index: number) => {
    const newClients = clients.filter((_, i) => i !== index);
    onChange(newClients);
  };

  return (
    <div className="space-y-3">
      <h4 className="text-md font-medium text-gray-700 dark:text-gray-300 mb-2">Clients</h4>
      {clients.length === 0 && <p className="text-sm text-gray-500 dark:text-gray-400">No clients configured. Click &quot;Add Client&quot; to begin.</p>}
      {clients.map((client, index) => (
        <ProtocolClientListItem
          key={index}
          client={client}
          index={index}
          onUpdateClient={updateClient}
          onRemoveClient={removeClient}
          protocol={protocol} // Pass the protocol down
        />
      ))}
      <button
        type="button"
        onClick={addClient}
        className="mt-2 px-3 py-1.5 text-sm bg-green-500 hover:bg-green-600 text-white rounded-md shadow-sm"
      >
        Add Client
      </button>
    </div>
  );
};
export default ProtocolClientSettings;
