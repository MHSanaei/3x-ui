import { useCallback, useLayoutEffect, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent, RefObject } from 'react';

export interface PointerRowDrop<TKey extends string | number> {
  from: TKey;
  to: TKey;
  after: boolean;
}

interface PointerRowDrag<TKey extends string | number> {
  from: TKey;
  to: TKey | null;
  after: boolean;
}

interface PointerRowReorderOptions<TKey extends string | number> {
  containerRef: RefObject<HTMLElement | null>;
  getRowKey: (row: HTMLElement) => TKey | null;
  disabled?: boolean;
  isNoop?: (drop: PointerRowDrop<TKey>) => boolean;
  onDrop: (drop: PointerRowDrop<TKey>) => void;
}

// Rows expose data-row-key; consumers retain their own insertion and persistence rules.
export function usePointerRowReorder<TKey extends string | number>(
  options: PointerRowReorderOptions<TKey>,
) {
  const [drag, setDrag] = useState<PointerRowDrag<TKey> | null>(null);
  const latest = useRef(options);
  const cleanupRef = useRef<(() => void) | null>(null);

  useLayoutEffect(() => {
    latest.current = options;
    if (cleanupRef.current && options.disabled) {
      cleanupRef.current();
      setDrag(null);
    }
  }, [options]);
  useLayoutEffect(() => () => cleanupRef.current?.(), []);

  const onPointerDown = useCallback((from: TKey, event: ReactPointerEvent<Element>) => {
    const current = latest.current;
    if (current.disabled || cleanupRef.current || event.isPrimary === false || event.button !== 0)
      return;
    event.preventDefault();
    const handle = event.currentTarget;
    const doc = handle.ownerDocument;
    const pointerId = event.pointerId;
    const startY = event.clientY;
    let active = false;

    const targetAt = (e: PointerEvent): PointerRowDrop<TKey> | null => {
      const row = doc
        .elementFromPoint(e.clientX, e.clientY)
        ?.closest<HTMLElement>('[data-row-key]');
      const config = latest.current;
      if (!row || !config.containerRef.current?.contains(row)) return null;
      const to = config.getRowKey(row);
      if (to === null) return null;
      const rect = row.getBoundingClientRect();
      return { from, to, after: e.clientY > rect.top + rect.height / 2 };
    };
    const move = (e: PointerEvent) => {
      if (e.pointerId !== pointerId || (!active && Math.abs(e.clientY - startY) < 5)) return;
      active = true;
      setDrag(targetAt(e) ?? { from, to: null, after: false });
    };
    const cleanup = () => {
      doc.removeEventListener('pointermove', move);
      doc.removeEventListener('pointerup', up);
      doc.removeEventListener('pointercancel', cancel);
      handle.removeEventListener('lostpointercapture', cancel);
      cleanupRef.current = null;
      try {
        if (handle.hasPointerCapture?.(pointerId)) handle.releasePointerCapture(pointerId);
      } catch {
        // Capture may already be released when the handle is detached.
      }
    };
    const cancel = (e: Event) => {
      if (!('pointerId' in e) || e.pointerId !== pointerId) return;
      cleanup();
      setDrag(null);
    };
    const up = (e: PointerEvent) => {
      if (e.pointerId !== pointerId) return;
      const drop =
        active && latest.current.containerRef.current?.contains(handle) ? targetAt(e) : null;
      cleanup();
      setDrag(null);
      const config = latest.current;
      if (!drop || config.disabled || drop.from === drop.to || config.isNoop?.(drop)) return;
      config.onDrop(drop);
    };

    cleanupRef.current = cleanup;
    doc.addEventListener('pointermove', move);
    doc.addEventListener('pointerup', up);
    doc.addEventListener('pointercancel', cancel);
    handle.addEventListener('lostpointercapture', cancel);
    try {
      handle.setPointerCapture?.(pointerId);
    } catch {
      // Detached handles and synthetic pointers may not support capture.
    }
  }, []);

  return { drag, onPointerDown };
}
