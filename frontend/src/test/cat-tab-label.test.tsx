import { SettingOutlined } from '@ant-design/icons';
import { screen } from '@testing-library/react';
import { Tabs } from 'antd';
import { describe, expect, it } from 'vitest';

import { catTabLabel } from '@/pages/settings/catTabLabel';

import { renderWithProviders } from './test-utils';

// On phones the tab shows only its icon; callers mark that icon aria-hidden for the
// desktop label, which also hid the name catTabLabel gives it (PR #6907 review).
describe('catTabLabel', () => {
  it.each([true, false])('names the tab with its text (iconsOnly=%s)', (iconsOnly) => {
    renderWithProviders(
      <Tabs
        items={[
          {
            key: 'panel',
            label: catTabLabel(<SettingOutlined aria-hidden />, 'Panel Settings', iconsOnly),
          },
        ]}
      />,
    );

    expect(screen.getByRole('tab', { name: 'Panel Settings' }).getAttribute('aria-selected')).toBe(
      'true',
    );
  });
});
