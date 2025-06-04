"use client";

import React, { useEffect, useState, useCallback } from 'react';
import { useAuth } from '@/context/AuthContext';
import { post } from '@/services/api'; // ApiResponse removed
import { InboundFromList } from '@/types/inbound'; // Import the new types
import { formatBytes } from '@/lib/formatters'; // formatUptime removed
import Link from 'next/link'; // For "Add New Inbound" button

// Simple toggle switch component (can be moved to ui components later)
const ToggleSwitch: React.FC<{ enabled: boolean; onChange: (enabled: boolean) => void; disabled?: boolean }> = ({ enabled, onChange, disabled }) => {
  return (
    <button
      type="button"
      disabled={disabled}
      className={`${enabled ? 'bg-primary-500' : 'bg-gray-300 dark:bg-gray-600'} relative inline-flex items-center h-6 rounded-full w-11 transition-colors focus:outline-none disabled:opacity-50`}
      onClick={() => onChange(!enabled)}
    >
      <span className="sr-only">Enable/Disable</span>
      <span
        className={`${enabled ? 'translate-x-6' : 'translate-x-1'} inline-block w-4 h-4 transform bg-white rounded-full transition-transform`}
      />
    </button>
  );
};


const InboundsPage: React.FC = () => {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [inbounds, setInbounds] = useState<InboundFromList[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Action loading states
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [togglingId, setTogglingId] = useState<number | null>(null);


  const fetchInbounds = useCallback(async () => {
    if (!isAuthenticated) return;
    setIsLoading(true);
    try {
      const response = await post<InboundFromList[]>('/inbound/list', {});
      if (response.success && response.data) {
        setInbounds(response.data);
        setError(null);
      } else {
        setError(response.message || 'Failed to fetch inbounds.');
        setInbounds([]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred.');
      setInbounds([]);
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      fetchInbounds();
    } else if (!authLoading && !isAuthenticated) {
        setIsLoading(false);
        setInbounds([]);
    }
  }, [isAuthenticated, authLoading, fetchInbounds]);

  const handleToggleEnable = async (inboundId: number, currentEnableStatus: boolean) => {
    setTogglingId(inboundId);
    // Find the inbound to get all its data for the update
    const inboundToUpdate = inbounds.find(ib => ib.id === inboundId);
    if (!inboundToUpdate) {
        console.error("Inbound not found for toggling");
        setTogglingId(null);
        return;
    }

    // Create a payload that matches the expected structure for update,
    // only changing the 'enable' field.
    const payload = { ...inboundToUpdate, enable: !currentEnableStatus };

    try {
        const response = await post<InboundFromList>(`/inbound/update/${inboundId}`, payload);
        if (response.success) {
            await fetchInbounds(); // Refresh list
        } else {
            setError(response.message || 'Failed to update inbound status.');
        }
    } catch (err) {
        setError(err instanceof Error ? err.message : 'Error updating inbound.');
    } finally {
        setTogglingId(null);
    }
  };

  const handleDeleteInbound = async (inboundId: number) => {
    if (!confirm('Are you sure you want to delete this inbound? This action cannot be undone.')) {
        return;
    }
    setDeletingId(inboundId);
    try {
        const response = await post(`/inbound/del/${inboundId}`, {});
        if (response.success) {
            await fetchInbounds(); // Refresh list
        } else {
            setError(response.message || 'Failed to delete inbound.');
        }
    } catch (err) {
        setError(err instanceof Error ? err.message : 'Error deleting inbound.');
    } finally {
        setDeletingId(null);
    }
  };


  if (authLoading || isLoading) {
    return <div className="p-4 text-gray-800 dark:text-gray-200">Loading inbounds...</div>;
  }

  // If not authenticated and not loading auth, show message (AuthContext should redirect anyway)
  if (!isAuthenticated && !authLoading) {
    return <div className="p-4 text-gray-800 dark:text-gray-200">Please login to view inbounds.</div>;
  }


  return (
    <div className="text-gray-800 dark:text-gray-200 p-2 md:p-0">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl md:text-3xl font-semibold">Inbounds Management</h1>
        <Link href="/inbounds/add" className="px-4 py-2 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 transition-colors">
          Add New Inbound
        </Link>
      </div>

      {error && <div className="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-md">Error: {error}</div>}

      {inbounds.length === 0 && !error && <p>No inbounds found.</p>}

      {inbounds.length > 0 && (
        <div className="overflow-x-auto bg-white dark:bg-gray-800 shadow-lg rounded-lg">
          <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Remark</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Protocol</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Port / Listen</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Traffic (Up/Down)</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Quota</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Expiry</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Status</th>
                <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
              {inbounds.map((inbound) => (
                <tr key={inbound.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100">{inbound.remark || 'N/A'}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{inbound.protocol}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{inbound.port}{inbound.listen ? ` (${inbound.listen})` : ''}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{formatBytes(inbound.up)} / {formatBytes(inbound.down)}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">{inbound.total > 0 ? formatBytes(inbound.total) : 'Unlimited'}</td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300">
                    {inbound.expiryTime === 0 ? 'Never' : new Date(inbound.expiryTime).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <ToggleSwitch
                        enabled={inbound.enable}
                        onChange={() => handleToggleEnable(inbound.id, inbound.enable)}
                        disabled={togglingId === inbound.id}
                    />
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm font-medium space-x-2">
                    <Link href={`/inbounds/edit/${inbound.id}`} className="text-primary-600 hover:text-primary-800 dark:text-primary-400 dark:hover:text-primary-300">Edit</Link>
                    <button
                        onClick={() => handleDeleteInbound(inbound.id)}
                        disabled={deletingId === inbound.id}
                        className="text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-300 disabled:opacity-50"
                    >
                        {deletingId === inbound.id ? 'Deleting...' : 'Delete'}
                    </button>
                    {/* Placeholder for Manage Clients/Details */}
                    <Link href={`/inbounds/${inbound.id}/clients`} className="text-green-600 hover:text-green-800 dark:text-green-400 dark:hover:text-green-300">Clients</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default InboundsPage;
