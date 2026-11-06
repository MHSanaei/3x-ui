import { useMemo } from 'react';
import { Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import ClientColumnPicker from '@/pages/clients/ClientColumnPicker';
import {
  clientColumnOptions,
  hideClientColumns,
  useHiddenClientColumns,
} from '@/pages/clients/columnVisibility';

interface Row {
  email: string;
}

function ClientsTable({ hasGroups = true }: { hasGroups?: boolean }) {
  const [hidden, setHidden] = useHiddenClientColumns();
  const base = useMemo<ColumnsType<Row>>(
    () => [
      { title: 'Client', key: 'email', dataIndex: 'email' },
      { title: 'Group', key: 'group', hidden: !hasGroups, render: () => 'g' },
      { title: 'Speed', key: 'speed', render: () => '—' },
      { title: 'Remaining', key: 'remaining', render: () => '1 GB' },
    ],
    [hasGroups],
  );
  return (
    <>
      <ClientColumnPicker
        options={clientColumnOptions(base)}
        hidden={hidden}
        onChange={setHidden}
      />
      <Table<Row>
        rowKey="email"
        columns={hideClientColumns(base, hidden)}
        dataSource={[{ email: 'alice' }]}
        pagination={false}
      />
    </>
  );
}

function headers(): string[] {
  return Array.from(document.querySelectorAll('thead th')).map((th) => th.textContent ?? '');
}

function openPicker(): HTMLElement {
  fireEvent.click(screen.getByRole('button', { name: /Columns/ }));
  return document.querySelector('.ant-dropdown') as HTMLElement;
}

function tick(menu: HTMLElement, label: string) {
  fireEvent.click(within(menu).getByRole('checkbox', { name: label }));
}

afterEach(() => {
  localStorage.clear();
});

describe('clients column picker', () => {
  it('keeps a column the admin hid hidden on the next visit', () => {
    const first = render(<ClientsTable />);
    tick(openPicker(), 'Speed');
    expect(headers()).toEqual(['Client', 'Group', 'Remaining']);
    first.unmount();

    render(<ClientsTable />);
    expect(headers()).toEqual(['Client', 'Group', 'Remaining']);
  });

  it('stays open while columns are ticked, so several can be changed in one go', () => {
    render(<ClientsTable />);
    const menu = openPicker();
    tick(menu, 'Speed');
    tick(menu, 'Remaining');

    expect(menu.classList.contains('ant-dropdown-hidden')).toBe(false);
    expect(headers()).toEqual(['Client', 'Group']);
  });

  it('never hides or offers the client column, even from a tampered stored value', () => {
    localStorage.setItem('clientsHiddenColumns', JSON.stringify(['email', 'speed', 'gone']));
    render(<ClientsTable />);

    expect(headers()).toEqual(['Client', 'Group', 'Remaining']);
    expect(within(openPicker()).queryByRole('checkbox', { name: 'Client' })).toBeNull();
  });

  it('falls back to all columns when the stored value is not valid JSON', () => {
    localStorage.setItem('clientsHiddenColumns', '{not json');
    render(<ClientsTable />);
    expect(headers()).toEqual(['Client', 'Group', 'Speed', 'Remaining']);
  });

  it('keeps Group hidden through edits made while no groups exist', () => {
    localStorage.setItem('clientsHiddenColumns', JSON.stringify(['group']));
    const noGroups = render(<ClientsTable hasGroups={false} />);
    const menu = openPicker();
    expect(within(menu).queryByRole('checkbox', { name: 'Group' })).toBeNull();
    tick(menu, 'Speed');
    noGroups.unmount();

    render(<ClientsTable hasGroups />);
    expect(headers()).toEqual(['Client', 'Remaining']);
  });
});
