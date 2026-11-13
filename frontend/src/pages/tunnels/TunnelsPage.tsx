import { useState } from 'react';
import {
  Alert,
  Button,
  Card,
  ConfigProvider,
  Input,
  Layout,
  Modal,
  Dropdown,
  Switch,
  Tooltip,
  Typography,
  Space,
  Table,
  Tag,
  message,
} from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import {
  DeleteOutlined,
  EditOutlined,
  KeyOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { HttpUtil, SizeFormatter } from '@/utils';
import { TunnelViewSchema, TunnelTokenSchema } from '@/schemas/tunnel';
import type { TunnelFormValues, TunnelRecord } from '@/schemas/tunnel';
import TunnelForm from './TunnelForm';
import './TunnelsPage.css';

const jsonOptions = { headers: { 'Content-Type': 'application/json' }, silent: true };
const listSchema = z.array(TunnelViewSchema);
const states: Record<string, string> = {
  connected: 'pages.tunnels.connected',
  connecting: 'pages.tunnels.connecting',
  starting: 'pages.tunnels.connecting',
  waiting: 'pages.tunnels.waiting',
  stopped: 'pages.tunnels.stopped',
  pending: 'pages.tunnels.pending',
  error: 'pages.tunnels.error',
  degraded: 'pages.tunnels.error',
  unavailable: 'pages.tunnels.unavailable',
};

export default function TunnelsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const cache = useQueryClient();
  const { nodes } = useNodesQuery();
  const [modal, modalContext] = Modal.useModal();
  const [messages, messageContext] = message.useMessage();
  const [editing, setEditing] = useState<TunnelRecord | null>(null);
  const [open, setOpen] = useState(false);
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const list = useQuery({
    queryKey: ['tunnels', 'list'],
    refetchInterval: 5000,
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/tunnels/list', undefined, { silent: true });
      if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
      return listSchema.parse(msg.obj);
    },
  });
  const action = useMutation({
    mutationFn: async ({ id, operation }: { id: number; operation: string }) => {
      const msg = await HttpUtil.post(`/panel/api/tunnels/${operation}/${id}`, {}, jsonOptions);
      if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
      if (operation === 'export') setToken(TunnelTokenSchema.parse(msg.obj).token);
    },
    onSuccess: () => {
      setError('');
      void messages.success(t('success'));
      void cache.invalidateQueries({ queryKey: ['tunnels'] });
    },
    onError: (e) => {
      setError(e.message);
      void messages.error(e.message);
    },
  });
  const save = async (data: TunnelFormValues) => {
    const path = editing ? `update/${editing.id}` : 'add';
    const msg = await HttpUtil.post(`/panel/api/tunnels/${path}`, data, jsonOptions);
    if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
    await cache.invalidateQueries({ queryKey: ['tunnels'] });
    void messages.success(t('success'));
  };
  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContext}
      {modalContext}
      <Layout className={`tunnels-page${isDark ? ' is-dark' : ''}${isUltra ? ' is-ultra' : ''}`}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Card size="small">
              <div className="tunnel-toolbar">
                <Typography.Text strong>{t('menu.tunnels')}</Typography.Text>
                <Space wrap>
                  <Button
                    icon={<ReloadOutlined aria-hidden />}
                    loading={list.isFetching}
                    onClick={() => void list.refetch()}
                  >
                    {t('refresh')}
                  </Button>
                  <Button
                    type="primary"
                    icon={<PlusOutlined aria-hidden />}
                    onClick={() => {
                      setEditing(null);
                      setOpen(true);
                    }}
                  >
                    {t('add')}
                  </Button>
                </Space>
              </div>
              <div className="tunnel-page-notices">
                <Alert type="info" showIcon title={t('pages.tunnels.flow')} />
                {(error || list.error) && (
                  <Alert type="error" showIcon title={error || list.error?.message} />
                )}
              </div>
              <Table<TunnelRecord>
                rowKey="guid"
                size="middle"
                pagination={{ hideOnSinglePage: true }}
                dataSource={list.data ?? []}
                loading={list.isPending}
                scroll={{ x: 1100 }}
                columns={[
                  {
                    title: t('remark'),
                    width: 180,
                    render: (_, r) => (
                      <div className="tunnel-name">
                        <Typography.Text strong>{r.name}</Typography.Text>
                        <Typography.Text type="secondary">
                          {r.config.role === 'client' ? 'Client' : 'Server'}
                          {r.config.reverse ? ' · Reverse' : ''}
                        </Typography.Text>
                      </div>
                    ),
                  },
                  {
                    title: t('menu.nodes'),
                    width: 150,
                    ellipsis: true,
                    render: (_, r) =>
                      r.nodeId
                        ? (nodes.find((n) => n.id === r.nodeId)?.name ?? r.nodeId)
                        : t('pages.tunnels.local'),
                  },
                  {
                    title: t('enable'),
                    width: 90,
                    render: (_, r) => (
                      <Tooltip title={t(r.enabled ? 'pages.tunnels.stop' : 'pages.tunnels.start')}>
                        <Switch
                          size="small"
                          aria-label={`${t('enable')}: ${r.name}`}
                          checked={r.enabled}
                          disabled={r.managed || action.isPending}
                          onChange={() =>
                            action.mutate({ id: r.id, operation: r.enabled ? 'stop' : 'start' })
                          }
                        />
                      </Tooltip>
                    ),
                  },
                  {
                    title: t('status'),
                    width: 190,
                    render: (_, r) => (
                      <div className="tunnel-status">
                        <Tag
                          color={
                            r.status.state === 'connected'
                              ? 'green'
                              : ['error', 'degraded'].includes(r.status.state)
                                ? 'red'
                                : r.status.state === 'unavailable'
                                  ? 'orange'
                                  : undefined
                          }
                        >
                          {t(states[r.status.state] ?? 'pages.tunnels.error')}
                        </Tag>
                        {r.status.reason && (
                          <Typography.Text type="secondary" className="tunnel-status-reason">
                            {r.status.reason}
                          </Typography.Text>
                        )}
                        {r.managed && (
                          <Typography.Text type="secondary">
                            {t('pages.tunnels.managed')}
                          </Typography.Text>
                        )}
                      </div>
                    ),
                  },
                  {
                    title: t('pages.tunnels.carrier'),
                    width: 130,
                    render: (_, r) =>
                      r.config.carrier === 'auto'
                        ? `AUTO → ${r.status.carrier || '—'}`
                        : r.config.carrier.toUpperCase(),
                  },
                  {
                    title: t('pages.tunnels.endpoint'),
                    width: 210,
                    ellipsis: true,
                    render: (_, r) => r.config.endpoint,
                  },
                  {
                    title: t('pages.tunnels.latencyLoss'),
                    align: 'right',
                    className: 'tunnel-metric',
                    width: 155,
                    render: (_, r) =>
                      r.status.metrics
                        ? `${r.status.metrics.srtt_ms?.toFixed(1) ?? '—'} ms / ${r.status.metrics.loss_pct?.toFixed(2) ?? '—'}%`
                        : '—',
                  },
                  {
                    title: t('pages.tunnels.traffic'),
                    align: 'right',
                    className: 'tunnel-metric',
                    width: 190,
                    render: (_, r) =>
                      r.status.metrics
                        ? `↓ ${SizeFormatter.sizeFormat(r.status.metrics.bytes_in_total)} / ↑ ${SizeFormatter.sizeFormat(r.status.metrics.bytes_out_total)}`
                        : '—',
                  },
                  {
                    title: t('pages.tunnels.errors'),
                    align: 'right',
                    width: 120,
                    render: (_, r) => r.status.metrics?.send_errors ?? '—',
                  },
                  {
                    key: 'actions',
                    width: 48,
                    fixed: 'right',
                    render: (_, r) => (
                      <Dropdown
                        trigger={['click']}
                        menu={{
                          items: [
                            { key: 'edit', label: t('edit'), icon: <EditOutlined aria-hidden /> },
                            {
                              key: 'restart',
                              label: t('pages.tunnels.restart'),
                              icon: <ReloadOutlined aria-hidden />,
                            },
                            {
                              key: 'export',
                              label: t('pages.tunnels.token'),
                              icon: <KeyOutlined aria-hidden />,
                            },
                            { type: 'divider' },
                            {
                              key: 'del',
                              label: t('delete'),
                              danger: true,
                              icon: <DeleteOutlined aria-hidden />,
                            },
                          ],
                          onClick: ({ key }) => {
                            if (key === 'edit') {
                              setEditing(r);
                              setOpen(true);
                            } else if (key === 'del')
                              modal.confirm({
                                title: t('pages.tunnels.deleteConfirm'),
                                content: r.name,
                                okText: t('delete'),
                                okButtonProps: { danger: true },
                                cancelText: t('cancel'),
                                onOk: () => action.mutateAsync({ id: r.id, operation: 'del' }),
                              });
                            else action.mutate({ id: r.id, operation: key });
                          },
                        }}
                      >
                        <Button
                          type="text"
                          icon={<MoreOutlined aria-hidden />}
                          aria-label={`${t('more')}: ${r.name}`}
                          disabled={r.managed || action.isPending}
                        />
                      </Dropdown>
                    ),
                  },
                ]}
              />
            </Card>
          </Layout.Content>
        </Layout>
      </Layout>
      <TunnelForm open={open} record={editing} onClose={() => setOpen(false)} onSave={save} />
      <Modal
        open={!!token}
        title={t('pages.tunnels.token')}
        onCancel={() => setToken('')}
        footer={<Button onClick={() => setToken('')}>{t('close')}</Button>}
        destroyOnHidden
      >
        <div className="tunnel-token-content">
          <Alert type="warning" showIcon title={t('pages.tunnels.tokenWarning')} />
          <Input.TextArea
            aria-label={t('pages.tunnels.token')}
            value={token}
            readOnly
            autoSize={{ minRows: 3, maxRows: 8 }}
            onFocus={(e) => e.currentTarget.select()}
          />
        </div>
      </Modal>
    </ConfigProvider>
  );
}
