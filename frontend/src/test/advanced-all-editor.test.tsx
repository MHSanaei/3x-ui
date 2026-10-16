import { useEffect } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { AdvancedAllEditor } from '@/pages/inbounds/form/advanced-editors';

vi.mock('@/components/form', () => ({
  JsonEditor: ({ value, onChange }: { value: string; onChange: (next: string) => void }) => (
    <textarea
      aria-label="Advanced inbound JSON"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}));

function Harness({ autoTag = false }: { autoTag?: boolean }) {
  const methods = useForm({
    defaultValues: {
      listen: '',
      port: 443,
      protocol: 'vless',
      tag: autoTag ? 'auto-443' : 'initial',
      settings: { clients: [] },
      sniffing: { enabled: false },
      streamSettings: { network: 'tcp' },
    },
  });
  const port = useWatch({ control: methods.control, name: 'port' });

  useEffect(() => {
    if (autoTag) methods.setValue('tag', `auto-${port}`);
  }, [autoTag, methods, port]);

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

  it('preserves valid edits and reflects external form updates', async () => {
    render(<Harness />);
    const input = editor();
    const next = input.value.replace('"tag": "initial"', '"tag":  "edited"');

    fireEvent.change(input, { target: { value: next } });
    await flushFormEffects();

    expect(input.value).toBe(next);

    fireEvent.click(screen.getByRole('button', { name: 'Update form externally' }));
    await flushFormEffects();

    expect(JSON.parse(input.value).tag).toBe('external');
    expect(input.value).toContain('"tag":  "external"');
  });

  it('does not show a rejected port string when the form still holds the numeric port', async () => {
    render(<Harness />);
    const input = editor();

    fireEvent.change(input, {
      target: { value: input.value.replace('"port": 443', '"port": "8443"') },
    });
    await flushFormEffects();

    expect(JSON.parse(input.value).port).toBe(443);
    expect(input.value).not.toContain('"port": "8443"');
  });

  it('does not silently accept removing a still-enabled form slice', async () => {
    render(<Harness />);
    const input = editor();
    const removed = JSON.parse(input.value) as Record<string, unknown>;
    delete removed.sniffing;

    fireEvent.change(input, { target: { value: JSON.stringify(removed, null, 2) } });
    await flushFormEffects();

    expect(JSON.parse(input.value).sniffing).toEqual({ enabled: false });
  });

  it('keeps whitespace when an auto-tag reacts to a port edit', async () => {
    render(<Harness autoTag />);
    const input = editor();
    const formatted = input.value.replace('"port": 443,', '"port": 443,\n');

    fireEvent.change(input, { target: { value: formatted } });
    await flushFormEffects();
    fireEvent.change(input, {
      target: { value: input.value.replace('"port": 443,', '"port": 8443,') },
    });
    await flushFormEffects();

    expect(JSON.parse(input.value)).toMatchObject({ port: 8443, tag: 'auto-8443' });
    expect(input.value).toContain('"port": 8443,\n\n');
  });
});
