import { vi } from 'vitest';

export function installPointerEvents() {
  class BrowserPointerEvent extends MouseEvent {
    pointerId: number;
    isPrimary: boolean;
    pointerType: string;
    constructor(type: string, init: PointerEventInit = {}) {
      super(type, init);
      this.pointerId = init.pointerId ?? 1;
      this.isPrimary = init.isPrimary ?? true;
      this.pointerType = init.pointerType ?? 'mouse';
    }
  }
  vi.stubGlobal('PointerEvent', BrowserPointerEvent);
  Object.defineProperty(document, 'elementFromPoint', { configurable: true, value: () => null });
}

export function pointAt(row: Element | null) {
  vi.spyOn(document, 'elementFromPoint').mockReturnValue(row);
  if (row)
    vi.spyOn(row, 'getBoundingClientRect').mockReturnValue({ top: 30, height: 20 } as DOMRect);
}
