import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';

import SniffingField from '@/lib/xray/forms/fields/SniffingField';
import { SniffingSchema } from '@/schemas/primitives/sniffing';

describe('SniffingField external re-sync', () => {
  it('reflects an external value change (e.g. an advanced JSON edit) in the friendly form', () => {
    const disabled = SniffingSchema.parse({ enabled: false });
    const enabled = SniffingSchema.parse({ enabled: true });
    const onChange = vi.fn();

    const { rerender, getByRole } = render(
      <SniffingField value={disabled} onChange={onChange} enableLabel="Enable" />,
    );
    expect(getByRole('switch').getAttribute('aria-checked')).toBe('false');

    rerender(<SniffingField value={enabled} onChange={onChange} enableLabel="Enable" />);
    expect(getByRole('switch').getAttribute('aria-checked')).toBe('true');
  });
});
