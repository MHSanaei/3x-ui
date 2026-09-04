import { useCallback, useSyncExternalStore } from 'react';

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

  return {
    isOpen: open,
    open: openPalette,
    close: closePalette,
    toggle: togglePalette,
  };
}
