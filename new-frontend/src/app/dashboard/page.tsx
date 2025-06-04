"use client";

import React, { useEffect, useState, useCallback } from 'react';
import { useAuth } from '@/context/AuthContext';
import { post } from '@/services/api'; // ApiResponse removed
import StatCard from '@/components/dashboard/StatCard';
import ProgressBar from '@/components/ui/ProgressBar';
import XrayStatusIndicator from '@/components/dashboard/XrayStatusIndicator';
import LogsModal from '@/components/shared/LogsModal';
import XrayGeoManagementModal from '@/components/dashboard/XrayGeoManagementModal'; // Import new modal
import { formatBytes, formatUptime, formatPercentage, toFixedIfNecessary } from '@/lib/formatters';

// Define button styles locally
const btnPrimaryStyles = "px-3 py-1.5 text-sm bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 disabled:opacity-50 transition-colors";
const btnDangerStyles = "px-3 py-1.5 text-sm bg-red-500 text-white font-semibold rounded-lg shadow-md hover:bg-red-600 disabled:opacity-50 transition-colors";
const btnSecondaryStyles = "px-3 py-1.5 text-sm bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors";


interface SystemResource { current: number; total: number; }
interface NetIO { up: number; down: number; }
interface NetTraffic { sent: number; recv: number; }
interface PublicIP { ipv4: string; ipv6: string; }
interface AppStats { threads: number; mem: number; uptime: number; }
type XrayState = "running" | "stop" | "error";
interface XrayStatusData { state: XrayState; errorMsg: string; version: string; }
interface ServerStatus {
  cpu: number; cpuCores: number; logicalPro: number; cpuSpeedMhz: number;
  mem: SystemResource; swap: SystemResource; disk: SystemResource;
  xray: XrayStatusData; uptime: number; loads: number[];
  tcpCount: number; udpCount: number; netIO: NetIO; netTraffic: NetTraffic;
  publicIP: PublicIP; appStats: AppStats;
}

const DashboardPage: React.FC = () => {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [isLoadingStatus, setIsLoadingStatus] = useState(true);
  const [pollingError, setPollingError] = useState<string | null>(null);

  const [actionLoading, setActionLoading] = useState<'' | 'restart' | 'stop'>('');
  const [actionMessage, setActionMessage] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const [isLogsModalOpen, setIsLogsModalOpen] = useState(false);
  const [isXrayGeoModalOpen, setIsXrayGeoModalOpen] = useState(false);


  const fetchStatus = useCallback(async (isInitialLoad = false) => {
    if (!isAuthenticated) return;
    if (isInitialLoad) {
      setIsLoadingStatus(true);
      // Clear polling error only on initial load attempt, so subsequent polling errors don't wipe the whole page if stale data is shown
      setPollingError(null);
    }
    try {
      const response = await post<ServerStatus>('/server/status', {});
      if (response.success && response.data) {
        setStatus(response.data);
        if(isInitialLoad) setPollingError(null); // Clear polling error if initial load succeeds
      } else {
        const errorMsg = response.message || 'Failed to fetch server status.';
        if (isInitialLoad) {
            setStatus(null); // Clear status on initial load failure
            setPollingError(errorMsg); // Set polling error to display page-level error
        } else {
            setPollingError(errorMsg); // For subsequent polling, just set the polling error to show warning
        }
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'An unknown error occurred.';
      if (isInitialLoad) {
        setStatus(null);
        setPollingError(errorMessage);
      } else {
        setPollingError(errorMessage);
      }
    } finally {
      if (isInitialLoad) setIsLoadingStatus(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      fetchStatus(true);
      const intervalId = setInterval(() => fetchStatus(false), 5000);
      return () => clearInterval(intervalId);
    } else if (!authLoading && !isAuthenticated) {
        setStatus(null); setIsLoadingStatus(false);
    }
  }, [isAuthenticated, authLoading, fetchStatus]);

  const handleXrayAction = async (action: 'restart' | 'stop') => {
    setActionLoading(action); setActionMessage(null);
    const endpoint = action === 'restart' ? '/server/restartXrayService' : '/server/stopXrayService';
    const successMessageText = action === 'restart' ? 'Xray restarted successfully.' : 'Xray stopped successfully.';
    const errorMessageText = action === 'restart' ? 'Failed to restart Xray.' : 'Failed to stop Xray.';
    let response;
    try {
      response = await post<null>(endpoint, {}); // Explicitly type ApiResponse<null>
      if (response.success) {
        setActionMessage({ type: 'success', message: response.message || successMessageText });
      } else { setActionMessage({ type: 'error', message: response.message || errorMessageText }); }
    } catch (err) { setActionMessage({ type: 'error', message: err instanceof Error ? err.message : `Error ${action}ing Xray.` }); }
    finally {
        setActionLoading('');
        await fetchStatus(false); // Re-fetch status after action
        if(response?.success) setTimeout(() => setActionMessage(null), 5000);
        // Keep error message displayed until next action or refresh
    }
  };

  const openLogsModal = () => { setActionMessage(null); setIsLogsModalOpen(true); };
  const openXrayGeoModal = () => { setActionMessage(null); setIsXrayGeoModalOpen(true); };


  if (authLoading || (isLoadingStatus && !status && !pollingError) ) {
    return <div className="p-4 text-gray-800 dark:text-gray-200 text-center">Loading dashboard data...</div>;
  }
  if (pollingError && !status && !isLoadingStatus) { // Show error prominently if initial load failed and no stale data
    return <div className="p-4 text-red-500 dark:text-red-400 text-center">Error: {pollingError}</div>;
  }

  const getUsageColor = (percentage: number): string => {
    if (percentage > 90) return 'bg-red-500';
    if (percentage > 75) return 'bg-yellow-500';
    return 'bg-green-500';
  };

  return (
    <div className="text-gray-800 dark:text-gray-200 p-2 md:p-0">
      <h1 className="text-2xl md:text-3xl font-semibold mb-6">Dashboard</h1>

      {actionMessage && (
        <div className={`mb-4 p-3 rounded-md ${actionMessage.type === 'success' ? 'bg-green-100 dark:bg-green-800/60 text-green-700 dark:text-green-200' : 'bg-red-100 dark:bg-red-800/60 text-red-700 dark:text-red-200'}`}>
          {actionMessage.message}
        </div>
      )}
      {pollingError && status && ( /* Polling error when data is stale */
         <div className="mb-4 p-3 bg-yellow-100 dark:bg-yellow-800/60 text-yellow-700 dark:text-yellow-200 rounded-md">
            Warning: Could not update status. Displaying last known data. Error: {pollingError}
        </div>)
      }

      {!status && !isLoadingStatus && !pollingError && <p className="text-center text-gray-500 dark:text-gray-400">No data available.</p>}

      {status && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 md:gap-6">
          <StatCard title="System Information">
            <p><strong>OS Uptime:</strong> {formatUptime(status.uptime)}</p>
            <p><strong>App Uptime:</strong> {formatUptime(status.appStats.uptime)}</p>
            <p><strong>Load Avg:</strong> {status.loads?.join(' / ')}</p>
            <p><strong>CPU Cores:</strong> {status.cpuCores} ({status.logicalPro} logical)</p>
            <p><strong>CPU Speed:</strong> {toFixedIfNecessary(status.cpuSpeedMhz,0)} MHz</p>
          </StatCard>

          <StatCard title="Xray Status" actions={
            <div className="flex flex-wrap gap-2">
              <button onClick={() => handleXrayAction('restart')} disabled={actionLoading !== ''} className={btnPrimaryStyles}>
                {actionLoading === 'restart' ? 'Restarting...' : 'Restart Xray'}
              </button>
              <button onClick={() => handleXrayAction('stop')} disabled={actionLoading !== '' || status.xray?.state === 'stop'} className={btnDangerStyles}>
                {actionLoading === 'stop' ? 'Stopping...' : 'Stop Xray'}
              </button>
              <button onClick={openLogsModal} disabled={actionLoading !== ''} className={btnSecondaryStyles}>View Logs</button>
              <button onClick={openXrayGeoModal} disabled={actionLoading !== ''} className={btnSecondaryStyles}>Manage Xray/Geo</button>
            </div>
          }>
            <XrayStatusIndicator state={status.xray?.state} version={status.xray?.version} errorMsg={status.xray?.errorMsg} />
          </StatCard>

          <StatCard title="CPU Usage"><div className="flex items-center justify-between"><span>{toFixedIfNecessary(status.cpu,1)}%</span></div><ProgressBar percentage={status.cpu} color={getUsageColor(status.cpu)} /></StatCard>
          <StatCard title="Memory Usage"><div className="flex items-center justify-between"><span>{formatBytes(status.mem.current)} / {formatBytes(status.mem.total)}</span><span>{formatPercentage(status.mem.current, status.mem.total)}%</span></div><ProgressBar percentage={formatPercentage(status.mem.current, status.mem.total)} color={getUsageColor(formatPercentage(status.mem.current, status.mem.total))} /></StatCard>
          <StatCard title="Swap Usage">{status.swap.total > 0 ? (<><div className="flex items-center justify-between"><span>{formatBytes(status.swap.current)} / {formatBytes(status.swap.total)}</span><span>{formatPercentage(status.swap.current, status.swap.total)}%</span></div><ProgressBar percentage={formatPercentage(status.swap.current, status.swap.total)} color={getUsageColor(formatPercentage(status.swap.current, status.swap.total))} /></>) : <p className="text-gray-500 dark:text-gray-400">Not available</p>}</StatCard>
          <StatCard title="Disk Usage (/)"><div className="flex items-center justify-between"><span>{formatBytes(status.disk.current)} / {formatBytes(status.disk.total)}</span><span>{formatPercentage(status.disk.current, status.disk.total)}%</span></div><ProgressBar percentage={formatPercentage(status.disk.current, status.disk.total)} color={getUsageColor(formatPercentage(status.disk.current, status.disk.total))} /></StatCard>
          <StatCard title="Network I/O"><p><strong>Upload:</strong> {formatBytes(status.netIO.up)}/s</p><p><strong>Download:</strong> {formatBytes(status.netIO.down)}/s</p><p className="mt-2"><strong>Total Sent:</strong> {formatBytes(status.netTraffic.sent)}</p><p><strong>Total Received:</strong> {formatBytes(status.netTraffic.recv)}</p></StatCard>
          <StatCard title="Connections"><p><strong>TCP:</strong> {status.tcpCount}</p><p><strong>UDP:</strong> {status.udpCount}</p></StatCard>
          <StatCard title="Public IP"><p><strong>IPv4:</strong> {status.publicIP.ipv4 || 'N/A'}</p><p><strong>IPv6:</strong> {status.publicIP.ipv6 || 'N/A'}</p></StatCard>
        </div>
      )}
      <LogsModal isOpen={isLogsModalOpen} onClose={() => setIsLogsModalOpen(false)} />
      {/* Conditional rendering for XrayGeoManagementModal to ensure 'status' and 'status.xray' are available */}
      {isXrayGeoModalOpen && status?.xray && <XrayGeoManagementModal
        isOpen={isXrayGeoModalOpen}
        onClose={() => setIsXrayGeoModalOpen(false)}
        currentXrayVersion={status.xray.version} // Pass version directly
        onActionComplete={() => fetchStatus(true)} // Re-fetch status on completion
      />}
    </div>
  );
};
export default DashboardPage;
