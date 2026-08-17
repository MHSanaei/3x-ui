import { StrictMode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { useServerDraft } from '@/hooks/useServerDraft';

describe('useServerDraft', () => {
  it('keeps an edited draft when the server refetches', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: { value: 'one' } } },
    );

    act(() => result.current.setDraft({ value: 'edited' }));
    rerender({ server: { value: 'two' } });

    expect(result.current.draft).toEqual({ value: 'edited' });
    expect(result.current.isDirty).toBe(true);
  });

  it('accepts a refetch that matches the saved draft', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: { value: 'one' } } },
    );

    act(() => result.current.setDraft({ value: 'saved' }));
    rerender({ server: { value: 'saved' } });

    expect(result.current.draft).toEqual({ value: 'saved' });
    expect(result.current.isDirty).toBe(false);
  });

  it('compares a preserved draft with the latest server value', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: { value: 'one' } } },
    );

    act(() => result.current.setDraft({ value: 'later' }));
    rerender({ server: { value: 'saved' } });
    act(() => result.current.setDraft({ value: 'one' }));

    expect(result.current.isDirty).toBe(true);
  });

  it('hydrates clean drafts under StrictMode', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      {
        initialProps: { server: { value: 'one' } },
        wrapper: StrictMode,
      },
    );

    rerender({ server: { value: 'two' } });

    expect(result.current.draft).toEqual({ value: 'two' });
    expect(result.current.isDirty).toBe(false);
  });

  it('preserves an edit made before the first server response', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: undefined as { value: string } | undefined } },
    );

    act(() => result.current.setDraft({ value: 'edited' }));
    rerender({ server: { value: 'one' } });

    expect(result.current.draft).toEqual({ value: 'edited' });
    expect(result.current.isDirty).toBe(true);
  });

  it('marks a sent draft clean before its refetch arrives', () => {
    const { result } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: { value: 'one' } } },
    );

    act(() => result.current.setDraft({ value: 'saved' }));
    act(() => result.current.markSaved({ value: 'saved' }));

    expect(result.current.isDirty).toBe(false);
  });
});
