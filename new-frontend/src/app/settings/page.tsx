"use client";

import React, { useEffect, useState, useCallback } from 'react';
import { useAuth } from '@/context/AuthContext';
import { post } from '@/services/api';
import { AllSetting, UpdateUserPayload } from '@/types/settings';
import PanelSettingsForm from '@/components/settings/PanelSettingsForm';
import UserAccountSettingsForm from '@/components/settings/UserAccountSettingsForm';
import TelegramSettingsForm from '@/components/settings/TelegramSettingsForm';
import SubscriptionSettingsForm from '@/components/settings/SubscriptionSettingsForm';
import OtherSettingsForm from '@/components/settings/OtherSettingsForm'; // Import the new form

// Define button styles locally
const btnSecondaryStyles = "px-4 py-2 bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors";

interface TabProps { label: string; isActive: boolean; onClick: () => void; }
const Tab: React.FC<TabProps> = ({ label, isActive, onClick }) => (
  <button
    type="button"
    onClick={onClick}
    className={`px-3 py-2 text-sm font-medium rounded-t-md focus:outline-none whitespace-nowrap ${isActive ? 'border-b-2 border-primary-500 text-primary-600 dark:text-primary-400' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'}`}
  >
    {label}
  </button>
);

const SettingsPage: React.FC = () => {
  const { user: authUser, checkAuthState, isAuthenticated, isLoading: authContextLoading } = useAuth();
  const [settings, setSettings] = useState<Partial<AllSetting>>({});
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isUpdatingUser, setIsUpdatingUser] = useState(false);

  const [pageError, setPageError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<string>('panel');

  const fetchSettings = useCallback(async () => {
    if (!isAuthenticated) return;
    setIsLoading(true); setPageError(null); setFormError(null);
    try {
      const response = await post<AllSetting>('/setting/all', {});
      if (response.success && response.data) {
        setSettings(response.data);
      } else { setPageError(response.message || 'Failed to fetch settings.'); }
    } catch (err) { setPageError(err instanceof Error ? err.message : 'Unknown error fetching settings.'); }
    finally { setIsLoading(false); }
  }, [isAuthenticated]);

  useEffect(() => {
    if (!authContextLoading && isAuthenticated) fetchSettings();
    else if (!authContextLoading && !isAuthenticated) setIsLoading(false);
  }, [isAuthenticated, authContextLoading, fetchSettings]);

  const handleSaveSettings = async (updatedSettingsData: Partial<AllSetting>) => {
    setIsSaving(true); setFormError(null); setSuccessMessage(null);
    let response;
    try {
      const fullSettingsPayload = { ...settings, ...updatedSettingsData };
      response = await post('/setting/update', fullSettingsPayload);
      if (response.success) {
        setSuccessMessage(response.message || 'Settings updated successfully! Some changes may require a panel restart.');
        await fetchSettings();
      } else { setFormError(response.message || 'Failed to update settings.'); }
    } catch (err) { setFormError(err instanceof Error ? err.message : 'Unknown error saving settings.'); }
    finally {
        setIsSaving(false);
        if (response?.success) {
            setTimeout(()=>setSuccessMessage(null), 5000);
        } else {
            setTimeout(()=>setFormError(null), 6000);
        }
    }
  };

  const handleUpdateUserCredentials = async (payload: UpdateUserPayload): Promise<boolean> => {
    setIsUpdatingUser(true); setFormError(null); setSuccessMessage(null);
    const finalPayload = { ...payload, oldUsername: payload.oldUsername || authUser?.username };
    let response;
    try {
      response = await post('/setting/updateUser', finalPayload);
      if (response.success) {
        setSuccessMessage(response.message || 'User credentials updated. You might need to log in again.');
        await checkAuthState();
        return true;
      } else { setFormError(response.message || 'Failed to update user credentials.'); return false; }
    } catch (err) { setFormError(err instanceof Error ? err.message : 'Unknown error updating user.'); return false; }
    finally { setIsUpdatingUser(false); if (response?.success) setTimeout(()=>setSuccessMessage(null), 5000); if (formError) setTimeout(()=>setFormError(null), 6000);}
  };

  const handleUpdateTwoFactor = async (twoFactorEnabled: boolean) : Promise<boolean> => {
    setIsSaving(true); setFormError(null); setSuccessMessage(null);
    let opSuccess = false;
    try {
        await handleSaveSettings({ twoFactorEnable: twoFactorEnabled });
        if (formError) { // Check if handleSaveSettings set an error
            opSuccess = false;
        } else {
            const refreshed = await post<AllSetting>('/setting/all', {});
            if(refreshed.success && refreshed.data){
                setSettings(refreshed.data);
                if(refreshed.data.twoFactorEnable === twoFactorEnabled){
                    setSuccessMessage(`2FA status successfully ${twoFactorEnabled ? 'enabled' : 'disabled'}.`);
                    opSuccess = true;
                } else {
                     setFormError("2FA status change was not reflected after save. Please check.");
                }
            } else {
                 setFormError("Failed to re-fetch settings after 2FA update.");
            }
        }
    } catch (err) {
        setFormError(err instanceof Error ? err.message : "Error in 2FA update process.");
    } finally {
        setIsSaving(false);
        if (opSuccess) setTimeout(()=>setSuccessMessage(null), 5000);
        else if (formError) setTimeout(()=>setFormError(null), 8000);
    }
    return opSuccess;
  };

  const handleRestartPanel = async () => {
    if (!window.confirm("Are you sure you want to restart the panel?")) return;
    setIsSaving(true); setFormError(null); setSuccessMessage(null);
    try {
      const response = await post('/setting/restartPanel', {});
      if (response.success) {
        setSuccessMessage(response.message || "Panel is restarting... Please wait and refresh.");
      } else { setFormError(response.message || "Failed to restart panel."); }
    } catch (err) { setFormError(err instanceof Error ? err.message : "Error restarting panel."); }
    finally { setIsSaving(false); if (successMessage) setTimeout(()=>setSuccessMessage(null), 7000); if (formError) setTimeout(()=>setFormError(null), 6000); }
  };

  const onTabChange = (tab: string) => {
    setActiveTab(tab); setFormError(null); setSuccessMessage(null);
  };

  const renderSettingsContent = () => {
    if (isLoading && !Object.keys(settings).length) return <p className="p-6 text-center text-gray-700 dark:text-gray-300">Loading settings data...</p>;

    switch(activeTab) {
      case 'panel':
        return <PanelSettingsForm initialSettings={settings} onSave={handleSaveSettings} isLoading={isSaving} formError={formError} />;
      case 'user':
        return <UserAccountSettingsForm
                    initialSettings={settings}
                    onUpdateUser={handleUpdateUserCredentials}
                    onUpdateTwoFactor={handleUpdateTwoFactor}
                    isSavingUser={isUpdatingUser}
                    isSavingSettings={isSaving}
                    formError={formError}
                    successMessage={successMessage}
                />;
      case 'telegram':
        return <TelegramSettingsForm initialSettings={settings} onSave={handleSaveSettings} isLoading={isSaving} formError={formError} />;
      case 'subscription':
        return <SubscriptionSettingsForm initialSettings={settings} onSave={handleSaveSettings} isLoading={isSaving} formError={formError} />;
      case 'other':
        return <OtherSettingsForm initialSettings={settings} onSave={handleSaveSettings} isLoading={isSaving} formError={formError} />;
      default:
        return <div className="p-6 bg-white dark:bg-gray-800 rounded-b-lg shadow-md"><p>Please select a settings category.</p></div>;
    }
  };

  if (authContextLoading) return <div className="p-4 text-center text-gray-700 dark:text-gray-300">Loading authentication...</div>;
  if (pageError && !Object.keys(settings).length) return <div className="p-4 text-red-500 dark:text-red-400 text-center">Error: {pageError}</div>;

  return (
    <div className="text-gray-800 dark:text-gray-200 p-2 md:p-0 max-w-4xl mx-auto">
      <h1 className="text-2xl md:text-3xl font-semibold mb-6">Panel Settings</h1>

      {successMessage && <div className="mb-4 p-3 bg-green-100 dark:bg-green-700/90 text-green-700 dark:text-green-100 rounded-md">{successMessage}</div>}
      {pageError && !isLoading && <div className="mb-4 p-3 bg-red-100 dark:bg-red-700/90 text-red-700 dark:text-red-100 rounded-md">Page Error: {pageError}</div>}

      <div className="border-b border-gray-200 dark:border-gray-700">
        <nav className="-mb-px flex space-x-1 sm:space-x-2 overflow-x-auto" aria-label="Tabs">
          <Tab label="Panel" isActive={activeTab === 'panel'} onClick={() => onTabChange('panel')} />
          <Tab label="User Account" isActive={activeTab === 'user'} onClick={() => onTabChange('user')} />
          <Tab label="Telegram Bot" isActive={activeTab === 'telegram'} onClick={() => onTabChange('telegram')} />
          <Tab label="Subscription" isActive={activeTab === 'subscription'} onClick={() => onTabChange('subscription')} />
          <Tab label="Other" isActive={activeTab === 'other'} onClick={() => onTabChange('other')} />
        </nav>
      </div>

      <div className="mt-1 bg-white dark:bg-gray-800 shadow-md rounded-b-lg">
        {renderSettingsContent()}
      </div>

      <div className="mt-8 flex flex-col sm:flex-row justify-end space-y-2 sm:space-y-0 sm:space-x-3">
        <button onClick={handleRestartPanel} disabled={isSaving || isUpdatingUser} className={`${btnSecondaryStyles} w-full sm:w-auto`}>
            {(isSaving || isUpdatingUser) ? 'Processing...' : 'Restart Panel'}
        </button>
      </div>
    </div>
  );
};

export default SettingsPage;
