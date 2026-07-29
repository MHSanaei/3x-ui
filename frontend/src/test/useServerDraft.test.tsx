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

  it('hydrates a clean draft and can explicitly discard edits', () => {
    const { result, rerender } = renderHook(
      ({ server }) => useServerDraft(server, (value) => ({ ...value }), (left, right) => left.value === right.value),
      { initialProps: { server: { value: 'one' } } },
    );

    rerender({ server: { value: 'two' } });
    expect(result.current.draft).toEqual({ value: 'two' });
    act(() => result.current.setDraft({ value: 'edited' }));
    act(() => result.current.discard());

    expect(result.current.draft).toEqual({ value: 'two' });
    expect(result.current.isDirty).toBe(false);
  });
});
