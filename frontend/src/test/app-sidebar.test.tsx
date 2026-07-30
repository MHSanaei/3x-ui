import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, test, vi } from 'vitest';

import AppSidebar from '@/layouts/AppSidebar';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

afterEach(() => {
  localStorage.clear();
});

function renderSidebar() {
  return renderWithProviders(
    <MemoryRouter>
      <AppSidebar />
    </MemoryRouter>,
  );
}

test('keeps the sidebar expanded after pinning it and restores the choice', () => {
  const first = renderSidebar();
  const sidebar = first.container.querySelector('.ant-layout-sider');

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);

  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.mouseLeave(first.container.querySelector('.ant-sidebar')!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(localStorage.getItem('sidebar-pinned')).toBe('true');

  first.unmount();

  const second = renderSidebar();
  const restoredSidebar = second.container.querySelector('.ant-layout-sider');

  expect(restoredSidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(screen.getByRole('button', { name: 'Unpin sidebar' })).not.toBeNull();
});
