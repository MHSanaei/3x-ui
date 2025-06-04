"use client";
import React from 'react';
import Link from 'next/link';
import { useAuth } from '@/context/AuthContext'; // Import useAuth

interface SidebarProps {
  isOpen: boolean;
  toggleSidebar: () => void;
}

const navItems = [
  { name: 'Dashboard', href: '/dashboard', icon: 'D' },
  { name: 'Inbounds', href: '/inbounds', icon: 'I' },
  { name: 'Settings', href: '/settings', icon: 'S' },
];

const Sidebar: React.FC<SidebarProps> = ({ isOpen, toggleSidebar }) => {
  const { logout, isLoading, user } = useAuth(); // Added user to display username

  const handleLogout = async () => {
    await logout();
  };

  return (
    <>
      {isOpen && (
        <div
          className="fixed inset-0 z-20 bg-black opacity-50 md:hidden"
          onClick={toggleSidebar}
        ></div>
      )}
      <aside
        className={`fixed inset-y-0 left-0 z-30 w-64 bg-gray-100 dark:bg-gray-900 text-gray-800 dark:text-gray-200 transform ${
          isOpen ? 'translate-x-0' : '-translate-x-full'
        } md:translate-x-0 transition-transform duration-300 ease-in-out shadow-lg flex flex-col`}
      >
        <div className="p-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-xl font-bold text-primary-500 dark:text-primary-400">3X-UI Panel</h2>
          {user && <span className="text-sm text-gray-600 dark:text-gray-400">Welcome, {user.username}</span>}
        </div>
        <nav className="flex-grow p-4">
          <ul>
            {navItems.map((item) => (
              <li key={item.name} className="mb-2">
                <Link href={item.href} className="flex items-center p-2 rounded-md hover:bg-primary-100 dark:hover:bg-gray-700 hover:text-primary-600 dark:hover:text-primary-300 transition-colors">
                  <span className="mr-3 w-6 h-6 flex items-center justify-center bg-primary-200 dark:bg-gray-600 rounded-full text-sm">{/* Icon placeholder */} {item.icon}</span>
                  {item.name}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        <div className="p-4 mt-auto border-t border-gray-200 dark:border-gray-700">
          <button
            onClick={handleLogout}
            disabled={isLoading}
            className="w-full flex items-center justify-center p-2 rounded-md bg-red-500 hover:bg-red-600 text-white dark:bg-red-600 dark:hover:bg-red-700 transition-colors disabled:opacity-50"
          >
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 mr-2">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
            </svg>
            {isLoading ? 'Logging out...' : 'Logout'}
          </button>
        </div>
      </aside>
    </>
  );
};
export default Sidebar;
