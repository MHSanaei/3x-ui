import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';

import RemarkTemplateField from '@/components/form/RemarkTemplateField';

describe('RemarkTemplateField', () => {
  it('inserts a {{TOKEN}} when a variable chip is clicked', async () => {
    const onChange = vi.fn();
    render(<RemarkTemplateField value="DE " onChange={onChange} maxLength={256} />);

    // Open the variable picker (the only button is the addon trigger).
    fireEvent.click(screen.getByRole('button'));
    fireEvent.click(await screen.findByText('{{EMAIL}}'));

    expect(onChange).toHaveBeenCalledTimes(1);
    const inserted = onChange.mock.calls[0][0] as string;
    expect(inserted).toContain('{{EMAIL}}');
    expect(inserted).toContain('DE');
  });

  it('renders a live preview of the expanded remark', () => {
    render(<RemarkTemplateField value="{{EMAIL}}" onChange={() => {}} />);
    // Sample expansion of {{EMAIL}} is "john".
    expect(screen.getByText('john')).toBeTruthy();
  });
});
