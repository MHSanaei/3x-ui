import React from 'react';

type ProcessState = "running" | "stop" | "error" | undefined;

interface XrayStatusIndicatorProps {
  state?: ProcessState;
  version?: string;
  errorMsg?: string;
}

const XrayStatusIndicator: React.FC<XrayStatusIndicatorProps> = ({ state, version, errorMsg }) => {
  let color = 'bg-gray-400'; // Default for undefined or unknown state
  let text = 'Unknown';

  switch (state) {
    case 'running':
      color = 'bg-green-500';
      text = 'Running';
      break;
    case 'stop':
      color = 'bg-yellow-500';
      text = 'Stopped';
      break;
    case 'error':
      color = 'bg-red-500';
      text = 'Error';
      break;
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center space-x-2">
        <span className={`w-3 h-3 rounded-full ${color}`}></span>
        <span className="text-gray-800 dark:text-gray-200">{text}</span>
      </div>
      {version && <p className="text-sm text-gray-600 dark:text-gray-400">Version: {version}</p>}
      {state === 'error' && errorMsg && (
        <p className="text-sm text-red-500 dark:text-red-400 mt-1 bg-red-100 dark:bg-red-900/20 p-2 rounded">
          Error: {errorMsg.length > 100 ? errorMsg.substring(0, 97) + "..." : errorMsg}
        </p>
      )}
    </div>
  );
};
export default XrayStatusIndicator;
