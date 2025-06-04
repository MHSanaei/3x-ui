import React from 'react';

interface ProgressBarProps {
  percentage: number;
  color?: string; // Tailwind color class e.g., 'bg-blue-500'
}

const ProgressBar: React.FC<ProgressBarProps> = ({ percentage, color = 'bg-primary-500' }) => {
  const safePercentage = Math.max(0, Math.min(100, percentage));
  return (
    <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
      <div
        className={`${color} h-2.5 rounded-full transition-all duration-300 ease-out`}
        style={{ width: `${safePercentage}%` }}
      ></div>
    </div>
  );
};
export default ProgressBar;
