import { useCallback, useEffect, useSyncExternalStore } from 'react';

let isOpen = false;
const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((listener) => listener());
}

export const commandPaletteStore = {
  getSnapshot: () => isOpen,
  subscribe: (listener: () => void) => {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
  open: () => {
    if (!isOpen) {
      isOpen = true;
      notify();
    }
  },
  close: () => {
    if (isOpen) {
      isOpen = false;
      notify();
    }
  },
  toggle: () => {
    isOpen = !isOpen;
    notify();
  },
};

export function useCommandPalette() {
  const open = useSyncExternalStore(commandPaletteStore.subscribe, commandPaletteStore.getSnapshot);

  const openPalette = useCallback(() => commandPaletteStore.open(), []);
  const closePalette = useCallback(() => commandPaletteStore.close(), []);
  const togglePalette = useCallback(() => commandPaletteStore.toggle(), []);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        commandPaletteStore.toggle();
      } else if (e.key === 'Escape' && isOpen) {
        e.preventDefault();
        commandPaletteStore.close();
      }
    }

    window.addEventListener('keydown', handleKeyDown, { capture: true });
    return () => {
      window.removeEventListener('keydown', handleKeyDown, { capture: true });
    };
  }, []);

  return {
    isOpen: open,
    open: openPalette,
    close: closePalette,
    toggle: togglePalette,
  };
}
