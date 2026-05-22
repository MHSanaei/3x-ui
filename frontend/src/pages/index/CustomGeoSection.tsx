import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, message, Modal, Space, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  InboxOutlined,
} from '@ant-design/icons';

import { HttpUtil, ClipboardManager } from '@/utils';
import CustomGeoFormModal from './CustomGeoFormModal';
import type { CustomGeoRecord } from './CustomGeoFormModal';
import './CustomGeoSection.css';

interface CustomGeoSectionProps {
  active: boolean;
}

interface CustomGeoListRecord extends CustomGeoRecord {
  lastUpdatedAt?: number;
}

function formatTime(ts?: number): string {
  if (!ts) return '';
  const d = new Date(ts * 1000);
  if (isNaN(d.getTime())) return String(ts);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function relativeTime(ts?: number): string {
  if (!ts) return '';
  const diff = Math.floor(Date.now() / 1000) - ts;
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)} min ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} h ago`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)} d ago`;
  return formatTime(ts);
}

function extDisplay(record: CustomGeoListRecord): string {
  const fn = record.type === 'geoip'
    ? `geoip_${record.alias}.dat`
    : `geosite_${record.alias}.dat`;
  return `ext:${fn}:tag`;
}

export default function CustomGeoSection({ active }: CustomGeoSectionProps) {
  const { t } = useTranslation();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [list, setList] = useState<CustomGeoListRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [updatingAll, setUpdatingAll] = useState(false);
  const [actionId, setActionId] = useState<number | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [editingRecord, setEditingRecord] = useState<CustomGeoListRecord | null>(null);

  const loadList = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/api/custom-geo/list');
      if (msg?.success && Array.isArray(msg.obj)) setList(msg.obj);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (active) loadList();
  }, [active, loadList]);

  function openAdd() {
    setEditingRecord(null);
    setFormOpen(true);
  }

  function openEdit(record: CustomGeoListRecord) {
    setEditingRecord(record);
    setFormOpen(true);
  }

  async function copyExt(record: CustomGeoListRecord) {
    const text = extDisplay(record);
    const ok = await ClipboardManager.copyText(text);
    if (ok) messageApi.success(`${t('copied')}: ${text}`);
  }

  function confirmDelete(record: CustomGeoListRecord) {
    modal.confirm({
      title: t('pages.index.customGeoDelete'),
      content: t('pages.index.customGeoDeleteConfirm'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/api/custom-geo/delete/${record.id}`);
        if (msg?.success) await loadList();
      },
    });
  }

  async function downloadOne(id: number) {
    setActionId(id);
    try {
      const msg = await HttpUtil.post(`/panel/api/custom-geo/download/${id}`);
      if (msg?.success) await loadList();
    } finally {
      setActionId(null);
    }
  }

  async function updateAll() {
    setUpdatingAll(true);
    try {
      const msg = await HttpUtil.post('/panel/api/custom-geo/update-all');
      const ok = msg?.obj?.succeeded?.length || 0;
      const failed = msg?.obj?.failed?.length || 0;
      if (msg?.success || ok > 0) {
        await loadList();
        if (failed > 0) messageApi.warning(`Updated ${ok}, failed ${failed}`);
      }
    } finally {
      setUpdatingAll(false);
    }
  }

  const columns = useMemo<ColumnsType<CustomGeoListRecord>>(
    () => [
      {
        title: t('pages.index.customGeoAlias'),
        key: 'alias',
        width: 200,
        render: (_v, record) => (
          <div className="custom-geo-alias-cell">
            <Tag color={record.type === 'geoip' ? 'cyan' : 'purple'} className="custom-geo-type-tag">
              {record.type}
            </Tag>
            <span className="custom-geo-alias">{record.alias}</span>
          </div>
        ),
      },
      {
        title: t('pages.index.customGeoUrl'),
        key: 'url',
        ellipsis: true,
        render: (_v, record) => (
          <Tooltip placement="topLeft" title={record.url}>
            <a
              href={record.url}
              target="_blank"
              rel="noopener noreferrer"
              className="custom-geo-url"
            >
              {record.url}
            </a>
          </Tooltip>
        ),
      },
      {
        title: t('pages.index.customGeoExtColumn'),
        key: 'extDat',
        width: 220,
        render: (_v, record) => (
          <Tooltip title={t('copy')}>
            <code
              className="custom-geo-ext-code custom-geo-copyable"
              onClick={() => copyExt(record)}
            >
              {extDisplay(record)}
            </code>
          </Tooltip>
        ),
      },
      {
        title: t('pages.index.customGeoLastUpdated'),
        key: 'lastUpdatedAt',
        width: 140,
        render: (_v, record) =>
          record.lastUpdatedAt ? (
            <Tooltip title={formatTime(record.lastUpdatedAt)}>
              <span>{relativeTime(record.lastUpdatedAt)}</span>
            </Tooltip>
          ) : (
            <span className="custom-geo-muted">—</span>
          ),
      },
      {
        title: t('pages.index.customGeoActions'),
        key: 'action',
        width: 120,
        render: (_v, record) => (
          <Space size="small">
            <Tooltip title={t('pages.index.customGeoEdit')}>
              <Button
                type="link"
                size="small"
                icon={<EditOutlined />}
                onClick={() => openEdit(record)}
              />
            </Tooltip>
            <Tooltip title={t('pages.index.customGeoDownload')}>
              <Button
                type="link"
                size="small"
                loading={actionId === record.id}
                icon={<ReloadOutlined />}
                onClick={() => downloadOne(record.id)}
              />
            </Tooltip>
            <Tooltip title={t('pages.index.customGeoDelete')}>
              <Button
                type="link"
                size="small"
                danger
                icon={<DeleteOutlined />}
                onClick={() => confirmDelete(record)}
              />
            </Tooltip>
          </Space>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, actionId],
  );

  return (
    <div className="custom-geo-section">
      {messageContextHolder}
      {modalContextHolder}
      <Alert
        type="info"
        showIcon
        className="mb-10"
        title={t('pages.index.customGeoRoutingHint')}
      />

      <div className="toolbar">
        <Button type="primary" loading={loading} onClick={openAdd} icon={<PlusOutlined />}>
          {t('pages.index.customGeoAdd')}
        </Button>
        <Button
          loading={updatingAll}
          disabled={list.length === 0}
          onClick={updateAll}
          icon={<ReloadOutlined />}
        >
          {t('pages.index.geofilesUpdateAll')}
        </Button>
        {list.length > 0 && <span className="custom-geo-count">{list.length}</span>}
      </div>

      <Table
        columns={columns}
        dataSource={list}
        pagination={false}
        rowKey={(r) => r.id}
        loading={loading}
        size="small"
        scroll={{ x: 760 }}
        locale={{
          emptyText: (
            <div className="custom-geo-empty">
              <InboxOutlined className="custom-geo-empty-icon" />
              <div>{t('pages.index.customGeoEmpty')}</div>
            </div>
          ),
        }}
      />

      <CustomGeoFormModal
        open={formOpen}
        record={editingRecord}
        onClose={() => setFormOpen(false)}
        onSaved={loadList}
      />
    </div>
  );
}
