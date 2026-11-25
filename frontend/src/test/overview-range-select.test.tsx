import { screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';

import OverviewRangeSelect from '@/pages/index/OverviewRangeSelect';
import { chooseSelectOption, listSelectOptions, renderWithProviders } from './test-utils';

it('shows the supported chart ranges and reports the selected bucket', async () => {
  const onChange = vi.fn();
  renderWithProviders(<OverviewRangeSelect value={2} onChange={onChange} />);

  const input = screen.getByRole('combobox', { name: 'Time range' });
  expect(screen.getByText('2m')).toBeTruthy();
  expect(listSelectOptions(input.id)).toEqual(['2m', '5m', '30m', '1h', '6h', '12h']);
  chooseSelectOption(input.id, '12h');

  await waitFor(() => expect(onChange).toHaveBeenCalledWith(720));
});
