import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Dropdown, Input, InputNumber, Space } from 'antd';
import { MoreOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { addrFor, domainsFor, expectedIPsFor } from './helpers';
import type { DnsServerValue } from './DnsServerModal';

export type DnsServerRow = { key: number; server: DnsServerValue };
export type FakednsTableRow = { key: number; ipPool: string; poolSize: number };

export function useDnsServerColumns({
  openEditServer,
  deleteServer,
}: {
  openEditServer: (idx: number) => void;
  deleteServer: (idx: number) => void;
}): ColumnsType<DnsServerRow> {
  const { t } = useTranslation();
  return useMemo(
    () => [
      {
        title: '#',
        key: 'action',
        align: 'center',
        width: 60,
        render: (_v, _record, index) => (
          <Space size={6}>
            <span className="row-index">{index + 1}</span>
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  { key: 'edit', label: <><EditOutlined /> {t('edit')}</>, onClick: () => openEditServer(index) },
                  { key: 'del', danger: true, label: <><DeleteOutlined /> {t('delete')}</>, onClick: () => deleteServer(index) },
                ],
              }}
            >
              <Button aria-label={t('more')} shape="circle" size="small" icon={<MoreOutlined />} />
            </Dropdown>
          </Space>
        ),
      },
      {
        title: t('pages.inbounds.address'),
        key: 'address',
        align: 'left',
        render: (_v, record) => addrFor(record.server),
      },
      {
        title: t('pages.xray.dns.domains'),
        key: 'domains',
        align: 'left',
        render: (_v, record) => <span className="muted">{domainsFor(record.server)}</span>,
      },
      {
        title: t('pages.xray.dns.expectIPs'),
        key: 'expectedIPs',
        align: 'left',
        render: (_v, record) => <span className="muted">{expectedIPsFor(record.server)}</span>,
      },
    ],
    [t, openEditServer, deleteServer],
  );
}

export function useFakednsColumns({
  deleteFakedns,
  updateFakednsField,
}: {
  deleteFakedns: (idx: number) => void;
  updateFakednsField: (idx: number, field: 'ipPool' | 'poolSize', value: string | number) => void;
}): ColumnsType<FakednsTableRow> {
  const { t } = useTranslation();
  return useMemo(
    () => [
      {
        title: '#',
        key: 'action',
        align: 'center',
        width: 60,
        render: (_v, _record, index) => (
          <Space size={6}>
            <span className="row-index">{index + 1}</span>
            <Button aria-label={t('delete')} shape="circle" size="small" danger icon={<DeleteOutlined />} onClick={() => deleteFakedns(index)} />
          </Space>
        ),
      },
      {
        title: 'IP pool',
        dataIndex: 'ipPool',
        key: 'ipPool',
        align: 'left',
        render: (_v, record, index) => (
          <Input
            value={record.ipPool}
            aria-label={t('pages.xray.fakedns.ipPool')}
            size="small"
            onChange={(e) => updateFakednsField(index, 'ipPool', e.target.value)}
          />
        ),
      },
      {
        title: 'Pool size',
        dataIndex: 'poolSize',
        key: 'poolSize',
        align: 'right',
        width: 120,
        render: (_v, record, index) => (
          <InputNumber
            value={record.poolSize}
            aria-label={t('pages.xray.fakedns.poolSize')}
            min={1}
            size="small"
            onChange={(v) => updateFakednsField(index, 'poolSize', Number(v) || 0)}
          />
        ),
      },
    ],
    [t, deleteFakedns, updateFakednsField],
  );
}
