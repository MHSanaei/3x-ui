import React, { ReactNode } from 'react';

interface StatCardProps {
  title: string;
  children: ReactNode;
  className?: string;
  actions?: ReactNode;
}

const StatCard: React.FC<StatCardProps> = ({ title, children, className = '', actions }) => {
  return (
    <div className={`bg-white dark:bg-gray-800 shadow-lg rounded-lg p-4 md:p-6 ${className}`}>
      <h3 className="text-lg font-semibold text-gray-700 dark:text-gray-300 mb-3">{title}</h3>
      <div className="space-y-2">
        {children}
      </div>
      {actions && <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">{actions}</div>}
    </div>
  );
};
export default StatCard;
