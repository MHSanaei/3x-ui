import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { FormProvider, useForm } from 'react-hook-form';

import { AdvancedAllEditor } from '@/pages/inbounds/form/advanced-editors';

vi.mock('@/components/form', () => ({
  JsonEditor: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (next: string) => void;
  }) => (
    <textarea
      aria-label="Advanced inbound JSON"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}));

function Harness() {
  const methods = useForm({
    defaultValues: {
      listen: '',
      port: 443,
      protocol: 'vless',
      tag: 'initial',
      settings: { clients: [] },
      sniffing: { enabled: false },
      streamSettings: { network: 'tcp' },
    },
  });

  return (
    <FormProvider {...methods}>
      <AdvancedAllEditor streamEnabled sniffingEnabled />
      <button type="button" onClick={() => methods.setValue('tag', 'external')}>
        Update form externally
      </button>
    </FormProvider>
  );
}

async function flushFormEffects() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

function editor(): HTMLTextAreaElement {
  return screen.getByRole('textbox', { name: 'Advanced inbound JSON' }) as HTMLTextAreaElement;
}

describe('AdvancedAllEditor JSON buffer (#6854)', () => {
  it('preserves a valid formatting-only edit instead of removing an entered newline', async () => {
    render(<Harness />);
    const input = editor();
    const next = input.value.replace('"port": 443,', '"port": 443,\n');

    fireEvent.change(input, { target: { value: next } });
    await flushFormEffects();

    expect(input.value).toBe(next);
    expect(input.value).toContain('"port": 443,\n\n');
  });

  it('preserves valid user edits and still reflects external form updates', async () => {
    render(<Harness />);
    const input = editor();
    const next = input.value.replace('"tag": "initial"', '"tag":  "edited"');

    fireEvent.change(input, { target: { value: next } });
    await flushFormEffects();

    expect(input.value).toBe(next);

    fireEvent.click(screen.getByRole('button', { name: 'Update form externally' }));
    await flushFormEffects();

    expect(JSON.parse(input.value).tag).toBe('external');
  });

  it('keeps temporarily invalid JSON in the local editor buffer', async () => {
    render(<Harness />);
    const input = editor();
    const incomplete = input.value.slice(0, -1);

    fireEvent.change(input, { target: { value: incomplete } });
    await flushFormEffects();

    expect(input.value).toBe(incomplete);
  });
});
