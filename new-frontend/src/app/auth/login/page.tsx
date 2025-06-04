"use client";

import React, { useState, useEffect, FormEvent } from 'react';
// No longer using useRouter here directly for push, AuthContext handles it.
import { post, ApiResponse } from '@/services/api'; // Added ApiResponse for explicit typing
import { useAuth } from '@/context/AuthContext'; // Import useAuth
import { useRouter, usePathname } from 'next/navigation'; // Still need for potential initial redirect, added usePathname

const LoginPage: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [twoFactorCode, setTwoFactorCode] = useState('');
  const [showTwoFactor, setShowTwoFactor] = useState(false);
  // isLoading and error are now primarily handled by AuthContext, but local form states can still be useful
  const [formIsLoading, setFormIsLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [twoFactorCheckLoading, setTwoFactorCheckLoading] = useState(true);

  const { login, isLoading: authIsLoading, isAuthenticated } = useAuth(); // Use login from context
  const router = useRouter(); // Still need for potential initial redirect if already logged in
  const pathname = usePathname(); // Get current pathname

  useEffect(() => {
    if (isAuthenticated) {
      router.push('/dashboard');
    }
  }, [isAuthenticated, router]);

  useEffect(() => {
    const checkTwoFactorStatus = async () => {
      setTwoFactorCheckLoading(true);
      try {
        const response = await post<boolean>('/getTwoFactorEnable', {});
        if (response.success && typeof response.data === 'boolean') {
          setShowTwoFactor(response.data);
        } else {
          console.warn('Could not determine 2FA status:', response.message);
          setShowTwoFactor(false);
        }
      } catch (err) {
        console.error('Error fetching 2FA status:', err);
        setFormError('Could not check 2FA status.');
        setShowTwoFactor(false);
      } finally {
        setTwoFactorCheckLoading(false);
      }
    };
    if (!isAuthenticated) { // Only check if not already authenticated
        checkTwoFactorStatus();
    } else {
        setTwoFactorCheckLoading(false); // If authenticated, no need to check
    }
  }, [isAuthenticated]); // Rerun if isAuthenticated changes (e.g. logout then back to login)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setFormIsLoading(true);
    setFormError(null);

    const response: ApiResponse<unknown> = await login(username, password, showTwoFactor ? twoFactorCode : undefined);

    if (!response.success) {
      setFormError(response.message || 'Login failed. Please check your credentials.');
    }
    // Redirection is handled by AuthContext or the useEffect above
    setFormIsLoading(false);
  };

  // If already authenticated and effect hasn't redirected yet, show loading or null
  if (isAuthenticated && !pathname.startsWith('/auth')) { // check added for pathname
    return <div className="flex items-center justify-center min-h-screen"><p>Loading dashboard...</p></div>;
  }


  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-100 dark:bg-gray-900">
      <div className="w-full max-w-md p-8 space-y-6 bg-white dark:bg-gray-800 shadow-xl rounded-lg">
        <h1 className="text-3xl font-bold text-center text-primary-600 dark:text-primary-400">
          Login
        </h1>
        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <label
              htmlFor="username"
              className="block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              Username
            </label>
            <input
              id="username"
              name="username"
              type="text"
              autoComplete="username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            />
          </div>

          <div>
            <label
              htmlFor="password"
              className="block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              Password
            </label>
            <input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
            />
          </div>

          {twoFactorCheckLoading && <p className="text-sm text-gray-500 dark:text-gray-400">Checking 2FA status...</p>}

          {!twoFactorCheckLoading && showTwoFactor && (
            <div>
              <label
                htmlFor="twoFactorCode"
                className="block text-sm font-medium text-gray-700 dark:text-gray-300"
              >
                Two-Factor Code
              </label>
              <input
                id="twoFactorCode"
                name="twoFactorCode"
                type="text"
                autoComplete="one-time-code"
                required={showTwoFactor}
                value={twoFactorCode}
                onChange={(e) => setTwoFactorCode(e.target.value)}
                className="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100"
              />
            </div>
          )}

          {formError && (
            <p className="text-sm text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/30 p-3 rounded-md">
              {formError}
            </p>
          )}

          <div>
            <button
              type="submit"
              disabled={formIsLoading || authIsLoading || twoFactorCheckLoading}
              className="w-full flex justify-center py-2 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary-500 dark:focus:ring-offset-gray-800 disabled:opacity-50"
            >
              {formIsLoading || authIsLoading ? 'Logging in...' : 'Login'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default LoginPage;
