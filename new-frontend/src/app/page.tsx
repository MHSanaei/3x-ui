import Link from 'next/link';

export default function HomePage() {
  return (
    <div className="flex flex-col items-center justify-center min-h-screen text-gray-800 dark:text-gray-200">
      <h1 className="text-4xl font-bold mb-6">Welcome to the New 3X-UI Panel</h1>
      <p className="text-lg mb-8">Experience a fresh new look and feel.</p>
      <Link href="/dashboard" className="px-6 py-3 bg-primary-500 text-white font-semibold rounded-lg shadow-md hover:bg-primary-600 transition-colors">
        Go to Dashboard
      </Link>
    </div>
  );
}
