"use client";

import React, { useState, useEffect, useCallback } from 'react';

type NetworkType = "tcp" | "kcp" | "ws" | "http" | "grpc" | "quic" | "";
type SecurityType = "none" | "tls" | "reality" | "";

interface StreamSettingsFormProps {
  initialStreamSettingsJson: string;
  onChange: (newJsonString: string) => void;
}

const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";

const StreamSettingsForm: React.FC<StreamSettingsFormProps> = ({ initialStreamSettingsJson, onChange }) => {
  const [network, setNetwork] = useState<NetworkType>('');
  const [security, setSecurity] = useState<SecurityType>('none');
  const [specificSettingsJson, setSpecificSettingsJson] = useState('{}');
  const [error, setError] = useState<string>('');

  const parseAndSetStates = useCallback((jsonString: string) => {
    try {
      const parsed = jsonString && jsonString.trim() !== "" ? JSON.parse(jsonString) : {};
      setNetwork(parsed.network || '');
      setSecurity(parsed.security || 'none');

      // Destructure again to get 'rest' after 'network' and 'security' have been read
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { network: _, security: __, ...rest } = parsed;
      setSpecificSettingsJson(Object.keys(rest).length > 0 ? JSON.stringify(rest, null, 2) : '{}');
      setError('');
    } catch (err) { // Use err
      setError('Stream Settings JSON is invalid. Displaying raw content for correction.');
      setSpecificSettingsJson(jsonString);
      console.error("Error parsing initial stream settings:", err); // Log error
    }
  }, []);

  useEffect(() => {
    parseAndSetStates(initialStreamSettingsJson);
  }, [initialStreamSettingsJson, parseAndSetStates]);

  const reconstructAndCallback = useCallback(() => {
    try {
      const specificParsed = JSON.parse(specificSettingsJson || '{}');
      const combined: Record<string, unknown> = {}; // Use Record<string, unknown>
      if (network) combined.network = network;
      if (security && security !== 'none') combined.security = security;

      const finalCombined = { ...specificParsed, ...combined };
      if (!finalCombined.network) delete finalCombined.network;
      if (!finalCombined.security) delete finalCombined.security;


      const finalJsonString = Object.keys(finalCombined).length === 0 ? '{}' : JSON.stringify(finalCombined, null, 2);
      onChange(finalJsonString);
      setError('');
    } catch (err) { // Use err
      setError('Invalid JSON in specific settings details. Fix to see combined output.');
      console.error("Error reconstructing stream settings JSON:", err); // Log error
    }
  }, [network, security, specificSettingsJson, onChange]);

  useEffect(() => {
    reconstructAndCallback();
  }, [network, security, specificSettingsJson, reconstructAndCallback]);

  const handleSpecificSettingsChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setSpecificSettingsJson(e.target.value);
  };

  return (
    <div className="space-y-4">
      {error && <p className="text-sm text-red-500 dark:text-red-400 mb-2">{error}</p>}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label htmlFor="stream-network" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Network</label>
          <select
            id="stream-network"
            value={network}
            onChange={(e) => setNetwork(e.target.value as NetworkType)}
            className={`mt-1 w-full ${inputStyles}`}
          >
            <option value="">(Detect from JSON or None)</option>
            <option value="tcp">TCP</option>
            <option value="kcp">mKCP</option>
            <option value="ws">WebSocket</option>
            <option value="http">HTTP/2 (H2)</option>
            <option value="grpc">gRPC</option>
            <option value="quic">QUIC</option>
          </select>
        </div>
        <div>
          <label htmlFor="stream-security" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Security</label>
          <select
            id="stream-security"
            value={security}
            onChange={(e) => setSecurity(e.target.value as SecurityType)}
            className={`mt-1 w-full ${inputStyles}`}
          >
            <option value="none">None</option>
            <option value="tls">TLS</option>
            <option value="reality">REALITY</option>
          </select>
        </div>
      </div>
      <div>
        <label htmlFor="stream-specific-settings" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
          Specific Network/Security Settings (JSON for details like tcpSettings, wsSettings, tlsSettings, etc.)
        </label>
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-1">
          Edit the JSON details for the selected network/security type here.
          The &apos;network&apos; and &apos;security&apos; fields above will be included in the final JSON.
        </p>
        <textarea
          id="stream-specific-settings"
          value={specificSettingsJson}
          onChange={handleSpecificSettingsChange}
          rows={10}
          className={`mt-1 w-full font-mono text-sm ${inputStyles}`}
          placeholder='e.g., { "wsSettings": { "path": "/ws" }, "tlsSettings": { "serverName": "domain.com" } }'
        />
      </div>
    </div>
  );
};
export default StreamSettingsForm;
