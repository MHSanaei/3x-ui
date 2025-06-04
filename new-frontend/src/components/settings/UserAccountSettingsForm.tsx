"use client";

import React, { useState, useEffect, FormEvent } from 'react';
import { AllSetting, UpdateUserPayload } from '@/types/settings';
import { QRCodeCanvas } from 'qrcode.react';

// Define styles locally
const inputStyles = "mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnPrimaryStyles = "px-4 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";

interface UserAccountSettingsFormProps {
  initialSettings: Partial<AllSetting>;
  onUpdateUser: (payload: UpdateUserPayload) => Promise<boolean>;
  onUpdateTwoFactor: (twoFactorEnabled: boolean) => Promise<boolean>;
  isSavingUser: boolean;
  isSavingSettings: boolean;
  formError?: string | null;
  successMessage?: string | null;
}

const UserAccountSettingsForm: React.FC<UserAccountSettingsFormProps> = ({
  initialSettings, onUpdateUser, onUpdateTwoFactor, isSavingUser, isSavingSettings, formError, successMessage
}) => {
  const [oldUsername, setOldUsername] = useState('');
  const [oldPassword, setOldPassword] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmNewPassword, setConfirmNewPassword] = useState('');
  const [twoFactorEnable, setTwoFactorEnable] = useState(false);
  const [twoFactorToken, setTwoFactorToken] = useState('');
  const [userFormError, setUserFormError] = useState<string | null>(null);
  const [twoFaFormError, setTwoFaFormError] = useState<string | null>(null);

  useEffect(() => {
    // Assuming username might come from a global auth context if it's the logged-in user's username
    // For now, if panel settings have a way to store current admin username (they don't directly), use that.
    // Otherwise, user has to type it. For simplicity, let's leave oldUsername blank initially or use a placeholder.
    // In a real app, this would ideally be pre-filled if known (e.g. from auth context).
    setOldUsername(''); // Or prefill if available from auth context
    setTwoFactorEnable(initialSettings.twoFactorEnable || false);
    setTwoFactorToken(initialSettings.twoFactorToken || '');
    setOldPassword('');
    setNewPassword('');
    setConfirmNewPassword('');
  }, [initialSettings]);

  const handleUserUpdateSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setUserFormError(null);
    setTwoFaFormError(null); // Clear other form error
    if (newPassword !== confirmNewPassword) {
      setUserFormError("New passwords do not match.");
      return;
    }
    if (!newUsername.trim() && !newPassword.trim() && !oldUsername.trim() && !oldPassword.trim()){
        // If all fields are empty, do nothing to prevent accidental submission of empty form
        setUserFormError("Please fill in the fields to update credentials.");
        return;
    }
    if ((newUsername.trim() || newPassword.trim()) && (!oldUsername.trim() || !oldPassword.trim())){
        setUserFormError("Current username and password are required to change credentials.");
        return;
    }
    if (!newPassword.trim() && newUsername.trim() && oldUsername.trim() && oldPassword.trim()){
        // Allow changing only username if new password is not set (but current credentials must be provided)
    } else if (newPassword.trim() && !newUsername.trim() && oldUsername.trim() && oldPassword.trim()){
        // Allow changing only password if new username is not set
         setNewUsername(oldUsername); // Use old username if only password is changing
    } else if (!newUsername.trim() || !newPassword.trim()){
        setUserFormError("New username and password cannot be empty if you intend to change them.");
        return;
    }


    const success = await onUpdateUser({ oldUsername, oldPassword, newUsername: newUsername.trim() || oldUsername, newPassword });
    if (success) {
        setOldPassword('');
        setNewPassword('');
        setConfirmNewPassword('');
        // Old username might need to be updated if it was changed
        setOldUsername(newUsername.trim() || oldUsername);
    }
  };

  const handleTwoFactorToggle = async () => {
    setUserFormError(null); // Clear other form error
    setTwoFaFormError(null);
    const new2FAStatus = !twoFactorEnable;
    const success = await onUpdateTwoFactor(new2FAStatus);
    // Parent (SettingsPage) will re-fetch settings which should update initialSettings
    // causing this component to re-render with new twoFactorEnable and twoFactorToken
    if (success && new2FAStatus && !initialSettings.twoFactorToken) {
        setTwoFaFormError("2FA enabled. Refreshing to get QR code/token...");
    } else if (success && !new2FAStatus) {
        setTwoFactorToken(''); // Clear token display immediately on disable
    }
  };

  const getOtpAuthUrl = () => {
    if (!twoFactorToken) return null;
    const label = `XUI-Panel:${initialSettings.webDomain || oldUsername || 'user'}`;
    const issuer = "XUI-Panel";
    return `otpauth://totp/${encodeURIComponent(label)}?secret=${twoFactorToken}&issuer=${encodeURIComponent(issuer)}`;
  };
  const otpUrl = getOtpAuthUrl();

  return (
    <div className="space-y-8 p-4 md:p-0"> {/* Added padding for consistency if this form is shown alone */}
      <form onSubmit={handleUserUpdateSubmit} className="space-y-4">
        <h3 className="text-lg font-medium text-gray-700 dark:text-gray-300 border-b dark:border-gray-600 pb-2">Change Login Credentials</h3>
        {userFormError && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{userFormError}</div>}
        {formError && formError.toLowerCase().includes("user") && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}
        {successMessage && successMessage.toLowerCase().includes("user") && <div className="p-3 bg-green-100 dark:bg-green-800/30 text-green-700 dark:text-green-200 rounded-md">{successMessage}</div>}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4">
          <div>
            <label htmlFor="oldUsername" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Current Username</label>
            <input type="text" id="oldUsername" value={oldUsername} onChange={e => setOldUsername(e.target.value)} className={`mt-1 w-full ${inputStyles}`} autoComplete="username"/>
          </div>
          <div>
            <label htmlFor="oldPassword" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Current Password</label>
            <input type="password" id="oldPassword" value={oldPassword} onChange={e => setOldPassword(e.target.value)} className={`mt-1 w-full ${inputStyles}`} autoComplete="current-password"/>
          </div>
          <div>
            <label htmlFor="newUsername" className="block text-sm font-medium text-gray-700 dark:text-gray-300">New Username</label>
            <input type="text" id="newUsername" value={newUsername} onChange={e => setNewUsername(e.target.value)} className={`mt-1 w-full ${inputStyles}`} autoComplete="new-password"/> {/* Use new-password to prevent autofill from old username */}
          </div>
          <div>
            <label htmlFor="newPassword" className="block text-sm font-medium text-gray-700 dark:text-gray-300">New Password</label>
            <input type="password" id="newPassword" value={newPassword} onChange={e => setNewPassword(e.target.value)} className={`mt-1 w-full ${inputStyles}`} autoComplete="new-password"/>
          </div>
          <div className="md:col-span-2">
            <label htmlFor="confirmNewPassword" className="block text-sm font-medium text-gray-700 dark:text-gray-300">Confirm New Password</label>
            <input type="password" id="confirmNewPassword" value={confirmNewPassword} onChange={e => setConfirmNewPassword(e.target.value)} className={`mt-1 w-full ${inputStyles}`} autoComplete="new-password"/>
          </div>
        </div>
        <div className="flex justify-end pt-2">
          <button type="submit" disabled={isSavingUser || isSavingSettings} className={btnPrimaryStyles}>
            {isSavingUser ? 'Updating...' : 'Update Credentials'}
          </button>
        </div>
      </form>

      <div className="space-y-4">
        <h3 className="text-lg font-medium text-gray-700 dark:text-gray-300 border-b dark:border-gray-600 pb-2">Two-Factor Authentication (2FA)</h3>
        {twoFaFormError && <div className="p-3 bg-yellow-100 dark:bg-yellow-800/30 text-yellow-700 dark:text-yellow-200 rounded-md">{twoFaFormError}</div>}
        {formError && formError.toLowerCase().includes("2fa") && <div className="p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-md">{formError}</div>}
        {successMessage && successMessage.toLowerCase().includes("2fa") && <div className="p-3 bg-green-100 dark:bg-green-800/30 text-green-700 dark:text-green-200 rounded-md">{successMessage}</div>}

        <div className="flex items-center space-x-3">
          <button
            type="button"
            onClick={handleTwoFactorToggle}
            disabled={isSavingSettings || isSavingUser} // Disable if any save operation is in progress
            className={`${twoFactorEnable ? 'bg-primary-600' : 'bg-gray-300 dark:bg-gray-600'} relative inline-flex items-center h-6 rounded-full w-11 transition-colors focus:outline-none disabled:opacity-50`}
          >
            <span className="sr-only">Toggle 2FA</span>
            <span className={`${twoFactorEnable ? 'translate-x-6' : 'translate-x-1'} inline-block w-4 h-4 transform bg-white rounded-full transition-transform`} />
          </button>
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">{twoFactorEnable ? '2FA Enabled' : '2FA Disabled'}</span>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Click the toggle to change 2FA status. This change requires saving all settings.
          If enabling for the first time, save settings, then the QR code and token will appear.
        </p>

        {twoFactorEnable && twoFactorToken && otpUrl && (
          <div className="mt-4 p-4 border dark:border-gray-700 rounded-md bg-gray-50 dark:bg-gray-700/30 space-y-3">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-200">Scan this QR code with your authenticator app:</p>
            <div className="flex justify-center my-2 p-2 bg-white rounded-md inline-block">
              <QRCodeCanvas value={otpUrl} size={180} bgColor={"#ffffff"} fgColor={"#000000"} level={"M"} />
            </div>
            <p className="text-sm font-medium text-gray-700 dark:text-gray-200">Or manually enter this token:</p>
            <p className="font-mono text-sm bg-gray-100 dark:bg-gray-600 p-2 rounded break-all text-gray-800 dark:text-gray-100">{twoFactorToken}</p>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Important: Store this token securely. If you lose access to your authenticator app, you may lose access to your account.
            </p>
          </div>
        )}
        {twoFactorEnable && !twoFactorToken && (
          <p className="text-sm text-yellow-600 dark:text-yellow-400 mt-2">
            2FA is marked as enabled, but no token is available. Please save settings.
            The panel may need a restart for the token to be generated if this is the first time enabling.
          </p>
        )}
      </div>
    </div>
  );
};
export default UserAccountSettingsForm;
