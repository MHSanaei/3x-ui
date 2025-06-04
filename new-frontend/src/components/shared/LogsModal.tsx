"use client";

import React, { useState, useEffect, useCallback } from 'react';
import { post } from '@/services/api';

// Define styles locally
const inputStyles = "mt-1 block w-full px-3 py-1.5 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-primary-500 focus:border-primary-500 sm:text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100";
const btnSecondaryStyles = "px-3 py-1.5 text-sm bg-gray-200 text-gray-800 font-semibold rounded-lg shadow-md hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors";

interface LogsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

type LogLevel = "debug" | "info" | "notice" | "warning" | "error";

const LogLevels: LogLevel[] = ["debug", "info", "notice", "warning", "error"];
const LogCounts: number[] = [20, 50, 100, 200, 500];

const LogsModal: React.FC<LogsModalProps> = ({ isOpen, onClose }) => {
  const [logs, setLogs] = useState<string[]>([]);
  const [count, setCount] = useState<number>(100);
  const [level, setLevel] = useState<LogLevel>('info');
  const [syslog, setSyslog] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchLogs = useCallback(async () => {
    if (!isOpen) return; // Should not fetch if modal is not open
    setIsLoading(true);
    setError(null);
    try {
      const apiLevel = level === 'error' ? 'err' : level;
      const response = await post<string[]>(`/server/logs/${count}`, { level: apiLevel, syslog: syslog.toString() });
      if (response.success && Array.isArray(response.data)) {
        setLogs(response.data);
      } else {
        setError(response.message || 'Failed to fetch logs.');
        setLogs([]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An unknown error occurred.');
      setLogs([]);
    } finally {
      setIsLoading(false);
    }
  }, [isOpen, count, level, syslog]); // Removed fetchLogs from its own dep array

  useEffect(() => {
    if (isOpen) {
      fetchLogs();
    }
  }, [isOpen, fetchLogs]);

  const handleDownload = () => {
    if (logs.length === 0) return;
    const blob = new Blob([logs.join('\n')], { type: 'text/plain;charset=utf-8' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = `xray_logs_${new Date().toISOString().split('T')[0]}.txt`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(link.href);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center p-4 z-50 transition-opacity duration-300 ease-in-out" onClick={onClose}>
      <div className="bg-white dark:bg-gray-800 p-5 rounded-lg shadow-xl w-full max-w-3xl h-[90vh] flex flex-col" onClick={e => e.stopPropagation()}>
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold text-gray-800 dark:text-gray-100">Xray Logs</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 text-2xl leading-none">&times;</button>
        </div>

        <div className="flex flex-wrap gap-2 items-center mb-4 pb-4 border-b border-gray-200 dark:border-gray-700">
          <div>
            <label htmlFor="log-count" className="text-sm mr-1 text-gray-700 dark:text-gray-300">Count:</label>
            <select id="log-count" value={count} onChange={e => setCount(Number(e.target.value))} className={`${inputStyles} text-sm p-1.5`}>
              {LogCounts.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div>
            <label htmlFor="log-level" className="text-sm mr-1 text-gray-700 dark:text-gray-300">Level:</label>
            <select id="log-level" value={level} onChange={e => setLevel(e.target.value as LogLevel)} className={`${inputStyles} text-sm p-1.5`}>
              {LogLevels.map(l => <option key={l} value={l}>{l.charAt(0).toUpperCase() + l.slice(1)}</option>)}
            </select>
          </div>
          <div className="flex items-center">
            <input type="checkbox" id="log-syslog" checked={syslog} onChange={e => setSyslog(e.target.checked)} className="h-4 w-4 text-primary-600 border-gray-300 dark:border-gray-600 rounded focus:ring-primary-500 bg-white dark:bg-gray-700" />
            <label htmlFor="log-syslog" className="text-sm ml-2 text-gray-700 dark:text-gray-300">Use Syslog</label>
          </div>
          <button onClick={fetchLogs} disabled={isLoading} className={`${btnSecondaryStyles} ml-auto`}>
            {isLoading ? 'Refreshing...' : 'Refresh'}
          </button>
           <button onClick={handleDownload} disabled={isLoading || logs.length === 0} className={btnSecondaryStyles}>
            Download Logs
          </button>
        </div>

        {isLoading && <p className="text-center text-gray-700 dark:text-gray-300">Loading logs...</p>}
        {error && <p className="text-red-500 dark:text-red-400 text-center p-2">{error}</p>}

        {!isLoading && !error && logs.length === 0 && <p className="text-center text-gray-500 dark:text-gray-400">No logs to display with current filters.</p>}

        {!isLoading && logs.length > 0 && (
          <div className="flex-grow overflow-auto bg-gray-100 dark:bg-gray-900 p-3 rounded text-xs font-mono leading-relaxed">
            {logs.map((log, index) => (
              <div key={index} className="whitespace-pre-wrap break-all text-gray-700 dark:text-gray-300 border-b border-gray-200 dark:border-gray-700 py-0.5">{log}</div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
export default LogsModal;
