import { useEffect, useState } from 'react';

export const MOBILE_BREAKPOINT_PX = 768;

/**
 * Tracks whether the viewport is narrower than `breakpoint`.
 *
 * Uses the native `matchMedia` change event instead of the `resize` event so
 * that state updates fire only when the query actually flips, not on every
 * pixel change during a window drag.
 */
export function useMediaQuery(breakpoint: number = MOBILE_BREAKPOINT_PX) {
  const query = `(max-width: ${breakpoint}px)`;
  const [isMobile, setIsMobile] = useState<boolean>(() =>
    typeof window !== 'undefined' ? window.matchMedia(query).matches : false,
  );

  useEffect(() => {
    const mql = window.matchMedia(query);
    const onChange = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mql.addEventListener('change', onChange);
    setIsMobile(mql.matches);
    return () => mql.removeEventListener('change', onChange);
  }, [query]);

  return { isMobile };
}
