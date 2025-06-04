"use client";

import React, { useState, useEffect, useCallback } from 'react';
import { post } from '@/services/api';

// Define styles locally
const btnSecondaryStyles = "px-3 py-1 text-xs bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors";

interface XrayGeoManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentXrayVersion: string | undefined;
  onActionComplete: () => Promise<void>; // Callback to refresh dashboard status
}

const GEO_FILES_LIST = [
  "geoip.dat",
  "geosite.dat",
  "geoip_IR.dat",
  "geosite_IR.dat",
];

const XrayGeoManagementModal: React.FC<XrayGeoManagementModalProps> = ({
  isOpen, onClose, currentXrayVersion, onActionComplete
}) => {
  const [xrayVersions, setXrayVersions] = useState<string[]>([]);
  const [isLoadingVersions, setIsLoadingVersions] = useState(false);
  const [isInstalling, setIsInstalling] = useState<string | null>(null);
  const [isUpdatingFile, setIsUpdatingFile] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const [activeTab, setActiveTab] = useState<'xray' | 'geo'>('xray');

  const fetchXrayVersions = useCallback(async () => {
    if (!isOpen) return; // Ensure modal is open before fetching
    setIsLoadingVersions(true); setError(null);
    try {
      // Explicitly type the expected response structure
      const response = await post<{ versions?: string[], data?: string[] }>('/server/getXrayVersion', {});
      if (response.success) {
        const versionsData = response.data; // data might be string[] or { versions: string[] }
        if (Array.isArray(versionsData)) { // Case: data is string[]
            setXrayVersions(versionsData);
        } else if (versionsData && Array.isArray(versionsData.versions)) { // Case: data is { versions: string[] }
            setXrayVersions(versionsData.versions);
        } else {
            setError('Fetched Xray versions data is not in the expected format.');
            setXrayVersions([]);
        }
      } else {
        setError(response.message || 'Failed to fetch Xray versions.');
        setXrayVersions([]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error fetching versions.');
      setXrayVersions([]);
    } finally {
      setIsLoadingVersions(false);
    }
  }, [isOpen]);

  useEffect(() => {
    if (isOpen && activeTab === 'xray') {
      fetchXrayVersions();
    }
  }, [isOpen, activeTab, fetchXrayVersions]);

  const handleInstallXray = async (version: string) => {
    if (!window.confirm(`Are you sure you want to install Xray version: ${version}?\nThis will restart Xray service.`)) return;
    setIsInstalling(version); setError(null); setSuccessMessage(null);
    let response; // Declare response here
    try {
      response = await post(`/server/installXray/${version}`, {});
      if (response.success) {
        setSuccessMessage(response.message || `Xray ${version} installed successfully. Xray service is restarting.`);
        onActionComplete();
      } else {
        setError(response.message || 'Failed to install Xray version.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error during Xray installation.');
    } finally {
      setIsInstalling(null);
      if (response?.success) setTimeout(() => setSuccessMessage(null), 4000);
      else if (error) setTimeout(() => setError(null), 4000); // Check local error state
    }
  };

  const handleUpdateGeoFile = async (fileName: string) => {
    if (!window.confirm(`Are you sure you want to update ${fileName}?\nThis may restart Xray service if changes are detected.`)) return;
    setIsUpdatingFile(fileName); setError(null); setSuccessMessage(null);
    let response; // Declare response here
    try {
      response = await post(`/server/updateGeofile/${fileName}`, {});
      if (response.success) {
        setSuccessMessage(response.message || `${fileName} updated successfully.`);
        onActionComplete();
      } else {
        setError(response.message || `Failed to update ${fileName}.`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : `Unknown error updating ${fileName}.`);
    } finally {
      setIsUpdatingFile(null);
      if (response?.success) setTimeout(() => setSuccessMessage(null), 4000);
      else if (error) setTimeout(() => setError(null), 4000); // Check local error state
    }
  };

  const handleClose = () => {
    setError(null);
    setSuccessMessage(null);
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center p-4 z-50" onClick={handleClose}>
      <div className="bg-white dark:bg-gray-800 p-5 rounded-lg shadow-xl w-full max-w-lg max-h-[90vh] flex flex-col" onClick={e => e.stopPropagation()}>
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-100">Xray & Geo Management</h2>
          <button onClick={handleClose} className="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 text-2xl leading-none">&times;</button>
        </div>

        {error && <div className="mb-3 p-2 bg-red-100 dark:bg-red-800/60 text-red-700 dark:text-red-200 rounded text-sm">{error}</div>}
        {successMessage && <div className="mb-3 p-2 bg-green-100 dark:bg-green-800/60 text-green-700 dark:text-green-200 rounded text-sm">{successMessage}</div>}

        <div className="border-b border-gray-200 dark:border-gray-700">
            <nav className="-mb-px flex space-x-4" aria-label="Tabs">
                <button type="button" onClick={() => setActiveTab('xray')} className={`px-3 py-2 text-sm font-medium rounded-t-md focus:outline-none ${activeTab === 'xray' ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}`}>Xray Versions</button>
                <button type="button" onClick={() => setActiveTab('geo')} className={`px-3 py-2 text-sm font-medium rounded-t-md focus:outline-none ${activeTab === 'geo' ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}`}>Geo Files</button>
            </nav>
        </div>

        <div className="flex-grow overflow-y-auto py-4">
          {activeTab === 'xray' && (
            <div>
              <p className="text-sm mb-2 text-gray-700 dark:text-gray-300">Current Xray Version: <span className="font-semibold text-primary-600 dark:text-primary-400">{currentXrayVersion || 'Unknown'}</span></p>
              {isLoadingVersions && <p className="text-gray-500 dark:text-gray-400">Loading versions...</p>}
              {!isLoadingVersions && xrayVersions.length === 0 && !error && <p className="text-gray-500 dark:text-gray-400">No versions found or failed to load.</p>}
              <ul className="space-y-2 max-h-60 overflow-y-auto">
                {xrayVersions.map(version => (
                  <li key={version} className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                    <span className="text-gray-800 dark:text-gray-200">{version}</span>
                    {version === currentXrayVersion ? (
                      <span className="px-2 py-1 text-xs text-green-700 bg-green-100 dark:bg-green-700 dark:text-green-100 rounded-full">Current</span>
                    ) : (
                      <button
                        onClick={() => handleInstallXray(version)}
                        disabled={isInstalling === version || !!isInstalling}
                        className={btnSecondaryStyles}
                      >
                        {isInstalling === version ? 'Installing...' : 'Install'}
                      </button>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}

          {activeTab === 'geo' && (
            <div>
              <p className="text-sm mb-3 text-gray-700 dark:text-gray-300">Manage GeoIP and GeoSite files.</p>
              <ul className="space-y-2">
                {GEO_FILES_LIST.map(fileName => (
                  <li key={fileName} className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                    <span className="text-gray-800 dark:text-gray-200">{fileName}</span>
                    <button
                      onClick={() => handleUpdateGeoFile(fileName)}
                      disabled={isUpdatingFile === fileName || !!isUpdatingFile}
                      className={btnSecondaryStyles}
                    >
                      {isUpdatingFile === fileName ? 'Updating...' : 'Update'}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
        <div className="flex justify-end pt-4 border-t border-gray-200 dark:border-gray-700">
          <button onClick={handleClose} className={btnSecondaryStyles}>Close</button>
        </div>
      </div>
    </div>
  );
};
export default XrayGeoManagementModal;
