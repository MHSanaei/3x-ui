import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';

import RemarkTemplateField from '@/components/form/RemarkTemplateField';
import { previewRemark, SUBSCRIPTION_METADATA_VARIABLES } from '@/lib/remark/remarkVariables';

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

  it('supports token insertion in multiline fields', async () => {
    const onChange = vi.fn();
    render(<RemarkTemplateField value="Hello " onChange={onChange} multiline rows={3} />);
    const textarea = screen.getByRole('textbox') as HTMLTextAreaElement;
    textarea.focus();
    textarea.setSelectionRange(textarea.value.length, textarea.value.length);

    fireEvent.click(screen.getByRole('button'));
    fireEvent.click(await screen.findByText('{{SUB_ID}}'));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0][0]).toBe('Hello {{SUB_ID}}');
  });

  it('limits the picker to client identity tokens for metadata fields', async () => {
    render(<RemarkTemplateField value="" onChange={() => {}} metadataOnly />);

    fireEvent.click(screen.getByRole('button'));

    expect(await screen.findByText('{{EMAIL}}')).toBeTruthy();
    expect(screen.queryByText('{{INBOUND}}')).toBeNull();
    expect(screen.queryByText('{{TRAFFIC_LEFT}}')).toBeNull();
  });

  it('previews metadata fields with metadata-safe tokens only', () => {
    expect(previewRemark('{{EMAIL}}/{{TRAFFIC_LEFT}}', SUBSCRIPTION_METADATA_VARIABLES, true)).toBe(
      'john/{{TRAFFIC_LEFT}}',
    );
  });
});
