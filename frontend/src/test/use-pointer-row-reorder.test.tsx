import { useRef } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { usePointerRowReorder } from '@/hooks/usePointerRowReorder';
import type { PointerRowDrop } from '@/hooks/usePointerRowReorder';
import { installPointerEvents, pointAt } from './pointer-events';

type Key = string | number;

function Harness({
  onDrop,
  disabled = false,
  keys = ['a', 'b'],
}: {
  onDrop: (drop: PointerRowDrop<Key>) => void;
  disabled?: boolean;
  keys?: Key[];
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { drag, onPointerDown } = usePointerRowReorder({
    containerRef,
    getRowKey: (row) => keys.find((key) => String(key) === row.dataset.rowKey) ?? null,
    disabled,
    onDrop,
  });
  return (
    <>
      <div ref={containerRef}>
        {keys.map((key) => (
          <div key={key} data-row-key={key} data-testid={`row-${key}`}>
            <button onPointerDown={(event) => onPointerDown(key, event)}>{key}</button>
          </div>
        ))}
      </div>
      <div data-row-key="b" data-testid="outside" />
      <output data-testid="drag">{drag ? JSON.stringify(drag) : 'idle'}</output>
    </>
  );
}

const pointer = { pointerId: 7, button: 0, clientX: 10, clientY: 32 };
const dragState = () => screen.getByTestId('drag').textContent;

function start(key: Key = 'a', init: PointerEventInit = {}) {
  fireEvent.pointerDown(screen.getByRole('button', { name: String(key) }), {
    ...pointer,
    clientY: 30,
    ...init,
  });
}

describe('usePointerRowReorder', () => {
  beforeEach(installPointerEvents);
  afterEach(() => vi.unstubAllGlobals());

  it('activates at five vertical pixels and stays active when returning near the origin', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 34 });
    expect(dragState()).toBe('idle');
    fireEvent.pointerMove(document, { ...pointer, clientY: 35 });
    expect(dragState()).toBe('{"from":"a","to":"b","after":false}');
    fireEvent.pointerMove(document, { ...pointer, clientY: 31 });
    expect(dragState()).toBe('{"from":"a","to":"b","after":false}');
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).toHaveBeenCalledExactlyOnceWith({ from: 'a', to: 'b', after: false });
  });

  it.each([{ button: 2 }, { isPrimary: false }])('rejects invalid pointers %j', (init) => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start('a', init);
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.pointerUp(document, pointer);
    expect(dragState()).toBe('idle');
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('ignores move, up and cancel from other pointers', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, pointerId: 8, clientY: 48 });
    expect(dragState()).toBe('idle');
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.pointerUp(document, { ...pointer, pointerId: 8 });
    fireEvent.pointerCancel(document, { ...pointer, pointerId: 8 });
    expect(onDrop).not.toHaveBeenCalled();
    expect(dragState()).toBe('{"from":"a","to":"b","after":true}');
    fireEvent.pointerUp(document, { ...pointer, clientY: 48 });
    fireEvent.pointerUp(document, { ...pointer, clientY: 48 });
    expect(onDrop).toHaveBeenCalledExactlyOnceWith({ from: 'a', to: 'b', after: true });
  });

  it('cancels without committing and safely releases pointer capture', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    const handle = screen.getByRole('button', { name: 'a' });
    handle.setPointerCapture = vi.fn();
    handle.hasPointerCapture = vi.fn(() => true);
    handle.releasePointerCapture = vi.fn();
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.pointerCancel(document, pointer);
    fireEvent.pointerUp(document, pointer);
    expect(handle.setPointerCapture).toHaveBeenCalledWith(7);
    expect(handle.releasePointerCapture).toHaveBeenCalledWith(7);
    expect(dragState()).toBe('idle');
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('does not reuse a previous valid target when released outside the container', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    pointAt(screen.getByTestId('outside'));
    fireEvent.pointerMove(document, { ...pointer, clientY: 50 });
    expect(dragState()).toBe('{"from":"a","to":null,"after":false}');
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).not.toHaveBeenCalled();
    expect(dragState()).toBe('idle');
  });

  it('rechecks the release target even when no final move event arrived', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    pointAt(null);
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('cancels when pointer capture is lost', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.lostPointerCapture(screen.getByRole('button', { name: 'a' }), pointer);
    fireEvent.pointerUp(document, pointer);
    expect(dragState()).toBe('idle');
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('does not drop on the source row or on a click below threshold', () => {
    const onDrop = vi.fn();
    render(<Harness onDrop={onDrop} />);
    pointAt(screen.getByTestId('row-a'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.pointerUp(document, pointer);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('uses the latest committed callback during a drag', () => {
    const oldDrop = vi.fn();
    const newDrop = vi.fn();
    const view = render(<Harness onDrop={oldDrop} />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    view.rerender(<Harness onDrop={newDrop} />);
    fireEvent.pointerUp(document, pointer);
    expect(oldDrop).not.toHaveBeenCalled();
    expect(newDrop).toHaveBeenCalledExactlyOnceWith({ from: 'a', to: 'b', after: false });
  });

  it('blocks disabled gestures and cancels when disabled during a drag', () => {
    const onDrop = vi.fn();
    const view = render(<Harness onDrop={onDrop} disabled />);
    pointAt(screen.getByTestId('row-b'));
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    expect(dragState()).toBe('idle');
    view.rerender(<Harness onDrop={onDrop} />);
    start();
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    view.rerender(<Harness onDrop={onDrop} disabled />);
    fireEvent.pointerUp(document, pointer);
    expect(dragState()).toBe('idle');
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('removes all gesture listeners and releases capture on unmount', () => {
    const onDrop = vi.fn();
    const view = render(<Harness onDrop={onDrop} />);
    const handle = screen.getByRole('button', { name: 'a' });
    handle.setPointerCapture = vi.fn();
    handle.hasPointerCapture = vi.fn(() => true);
    handle.releasePointerCapture = vi.fn();
    const add = vi.spyOn(document, 'addEventListener');
    const remove = vi.spyOn(document, 'removeEventListener');
    const removeCapture = vi.spyOn(handle, 'removeEventListener');
    start();
    const registrations = add.mock.calls.filter(([type]) =>
      ['pointermove', 'pointerup', 'pointercancel'].includes(type),
    );
    expect(registrations.map(([type]) => type).sort()).toEqual([
      'pointercancel',
      'pointermove',
      'pointerup',
    ]);
    view.unmount();
    for (const [type, listener] of registrations)
      expect(remove).toHaveBeenCalledWith(type, listener);
    expect(removeCapture).toHaveBeenCalledWith('lostpointercapture', expect.any(Function));
    expect(handle.releasePointerCapture).toHaveBeenCalledWith(7);
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).not.toHaveBeenCalled();
  });

  it('supports numeric keys and touch when capture APIs throw', () => {
    const onDrop = vi.fn();
    render(<Harness keys={[0, 1]} onDrop={onDrop} />);
    const handle = screen.getByRole('button', { name: '0' });
    handle.setPointerCapture = () => {
      throw new DOMException('inactive pointer');
    };
    pointAt(screen.getByTestId('row-1'));
    start(0, { pointerType: 'touch' });
    fireEvent.pointerMove(document, { ...pointer, clientY: 48 });
    fireEvent.pointerUp(document, pointer);
    expect(onDrop).toHaveBeenCalledExactlyOnceWith({ from: 0, to: 1, after: false });
  });
});
