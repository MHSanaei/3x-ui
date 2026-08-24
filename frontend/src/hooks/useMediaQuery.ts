import { useCallback, useSyncExternalStore } from 'react';

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

  const subscribe = useCallback(
    (onStoreChange: () => void) => {
      const mql = window.matchMedia(query);
      mql.addEventListener('change', onStoreChange);
      return () => mql.removeEventListener('change', onStoreChange);
    },
    [query],
  );

  const isMobile = useSyncExternalStore(
    subscribe,
    () => window.matchMedia(query).matches,
    () => false,
  );

  return { isMobile };
}
