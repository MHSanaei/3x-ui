"use client";

import React, { useEffect, useState } from 'react';
import { Inbound, ClientSetting } from '@/types/inbound';
import { generateSubscriptionLink } from '@/lib/subscriptionLink';
import { QRCodeCanvas } from 'qrcode.react';

const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 select-all";
const btnSecondaryStyles = "px-4 py-2 bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors text-sm";


interface ClientShareModalProps {
  isOpen: boolean;
  onClose: () => void;
  inbound: Inbound | null;
  client: ClientSetting | null; // For client-specific links (vmess, vless, trojan)
                               // For shadowsocks, client info is mostly in inbound.settings
}

const ClientShareModal: React.FC<ClientShareModalProps> = ({ isOpen, onClose, inbound, client }) => {
  const [link, setLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (isOpen && inbound) {
      // For Shadowsocks, the 'client' might be a conceptual representation derived from inbound settings,
      // or we can pass a minimal ClientSetting object.
      // The generateSubscriptionLink function expects a ClientSetting object.
      // If protocol is shadowsocks and client prop is null, we might need to construct one.
      let effectiveClient = client;
      if (inbound.protocol === 'shadowsocks' && !client) {
        // Construct a conceptual client for shadowsocks if client prop is null
        // The link generator for SS primarily uses inbound.settings anyway.
        effectiveClient = { email: inbound.remark || 'shadowsocks_client' };
      }

      if (effectiveClient) {
        setLink(generateSubscriptionLink(inbound, effectiveClient));
      } else {
        setLink(null);
      }
      setCopied(false);
    }
  }, [isOpen, inbound, client]);

  if (!isOpen || !inbound) return null;
  // If client is null for protocols that require it, link generation will fail.
  // This is handled by generateSubscriptionLink returning null.

  const handleCopy = () => {
    if (link) {
      navigator.clipboard.writeText(link)
        .then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
        })
        .catch(err => console.error('Failed to copy link: ', err));
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center p-4 z-50 transition-opacity duration-300 ease-in-out"
         onClick={onClose} // Close on overlay click
    >
      <div className="bg-white dark:bg-gray-800 p-6 rounded-lg shadow-xl w-full max-w-md space-y-4 transform transition-all duration-300 ease-in-out scale-100 opacity-100"
           onClick={e => e.stopPropagation()} // Prevent modal close when clicking inside modal
      >
        <div className="flex justify-between items-center">
          <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-100">Share Client Configuration</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 text-2xl leading-none">&times;</button>
        </div>

        {link ? (
          <div className="space-y-4">
            <div className="flex justify-center items-center p-4 border border-gray-200 dark:border-gray-700 rounded-md bg-gray-50 dark:bg-gray-900">
              <QRCodeCanvas value={link} size={220} bgColor={"#ffffff"} fgColor={"#000000"} level={"M"} />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-gray-700 dark:text-gray-300">Subscription Link:</label>
              <div className="flex items-center space-x-2">
                <input
                  type="text"
                  readOnly
                  value={link}
                  className={`${inputStyles} w-full text-xs`}
                />
                <button
                  onClick={handleCopy}
                  className={`${btnSecondaryStyles} whitespace-nowrap`}
                >
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Note: The server address used (&quot;YOUR_SERVER_IP_OR_DOMAIN&quot;) is a placeholder.
              For this link to work, ensure your actual server address/domain is correctly configured in the generation logic
              (<code>src/lib/subscriptionLink.ts</code>) or, ideally, fetched from panel settings in a future update.
            </p>
          </div>
        ) : (
          <p className="text-sm text-red-500 dark:text-red-400">Could not generate subscription link for this client/protocol combination.</p>
        )}

        <div className="flex justify-end pt-3">
          <button onClick={onClose} className={btnSecondaryStyles}>Close</button>
        </div>
      </div>
    </div>
  );
};
export default ClientShareModal;
