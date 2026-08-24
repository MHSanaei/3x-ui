import { Suspense, useState, type ReactNode } from 'react';
import { Spin } from 'antd';

interface LazyMountProps {
  when: boolean;
  fallback?: ReactNode;
  children: ReactNode;
}

// Mounts children only after `when` first becomes true and keeps them mounted
// thereafter, so React.lazy modals get loaded on demand but their close
// animations still play out. Pair with `lazy(() => import(...))` modal imports
// on heavy list pages to keep the initial bundle small.
export default function LazyMount({ when, fallback = <Spin />, children }: LazyMountProps) {
  const [mounted, setMounted] = useState(when);
  if (when && !mounted) setMounted(true);
  if (!mounted) return null;
  return <Suspense fallback={fallback}>{children}</Suspense>;
}
