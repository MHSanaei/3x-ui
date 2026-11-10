import { describe, expect, it, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FormProvider, useForm } from 'react-hook-form';

import { NativeSelect } from '@/components/form/NativeSelect';
import { FormField } from '@/components/form/rhf';

const options = [
  { value: 7, label: 'Number seven' },
  { value: '7', label: 'Text seven' },
  { value: 0, label: 'Zero' },
  { value: '', label: 'None' },
  { value: 9, label: 'Unavailable', disabled: true },
];

function Harness({
  onSubmit,
  disabled = false,
}: {
  onSubmit: (values: { choice: string | number }) => void;
  disabled?: boolean;
}) {
  const methods = useForm({ defaultValues: { choice: 7 as string | number } });
  return (
    <FormProvider {...methods}>
      <form onSubmit={methods.handleSubmit(onSubmit)}>
        <FormField name="choice" label="Choice">
          <NativeSelect<string | number> disabled={disabled} options={options} />
        </FormField>
        <button type="submit">Save</button>
      </form>
    </FormProvider>
  );
}

describe('NativeSelect form values', () => {
  it('preserves numeric, string and empty values through RHF instead of coercing to DOM strings', async () => {
    const onSubmit = vi.fn();
    render(<Harness onSubmit={onSubmit} />);
    const user = userEvent.setup();
    const select = screen.getByRole('combobox', { name: 'Choice' });
    expect(select).toHaveDisplayValue('Number seven');
    for (const [label, value] of [
      ['Text seven', '7'],
      ['Zero', 0],
      ['None', ''],
    ] as const) {
      await user.selectOptions(select, screen.getByRole('option', { name: label }));
      await user.click(screen.getByRole('button', { name: 'Save' }));
      await waitFor(() =>
        expect(onSubmit).toHaveBeenLastCalledWith({ choice: value }, expect.anything()),
      );
    }
    await user.selectOptions(select, screen.getByRole('option', { name: 'Unavailable' }));
    expect(select).toHaveDisplayValue('None');
  });

  it('keeps disabled values intact and skips the control during tab navigation', async () => {
    const onSubmit = vi.fn();
    render(<Harness disabled onSubmit={onSubmit} />);
    const user = userEvent.setup();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Save' })).toHaveFocus();
    await user.keyboard('{Enter}');
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith({ choice: 7 }, expect.anything()));
  });

  it('preserves an unlisted edit value and follows reordered dynamic options without changing it', async () => {
    const onChange = vi.fn();
    const view = render(
      <NativeSelect
        aria-label="Choice"
        value="saved-custom"
        options={[{ value: 'default', label: 'Default' }]}
        onChange={onChange}
      />,
    );
    expect(screen.getByRole('combobox', { name: 'Choice' })).toHaveDisplayValue('saved-custom');
    view.rerender(
      <NativeSelect
        aria-label="Choice"
        value="saved-custom"
        options={[
          { value: 'new', label: 'New' },
          { value: 'saved-custom', label: 'Saved choice' },
        ]}
        onChange={onChange}
      />,
    );
    expect(screen.getByRole('combobox', { name: 'Choice' })).toHaveDisplayValue('Saved choice');
    expect(onChange).not.toHaveBeenCalled();
    await userEvent
      .setup()
      .selectOptions(screen.getByRole('combobox'), screen.getByRole('option', { name: 'New' }));
    expect(onChange).toHaveBeenCalledWith('new');
  });
});
