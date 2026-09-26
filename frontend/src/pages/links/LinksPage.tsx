import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  ConfigProvider,
  Layout,
  Modal,
  Result,
  Space,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  LinkOutlined,
  PlusOutlined,
  ReloadOutlined,
  ShareAltOutlined,
} from '@ant-design/icons';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useDatepicker } from '@/hooks/useDatepicker';
import { IntlUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { useLinksQuery, useLinkTargetsQuery } from '@/api/queries/useLinksQuery';
import { useLinkMutations } from '@/api/queries/useLinkMutations';
import type { LinkRecord } from '@/schemas/api/link';
import LinkFormModal from './LinkFormModal';
import LinkTargetsModal from './LinkTargetsModal';

export default function LinksPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const { datepicker } = useDatepicker();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { links, loading, fetched, fetchError, refetch } = useLinksQuery();
  const { save, remove, setEnable, reorder, assign, unassign, refresh } = useLinkMutations();

  const [formOpen, setFormOpen] = useState(false);
  const [formLink, setFormLink] = useState<LinkRecord | null>(null);
  const [targetsLink, setTargetsLink] = useState<LinkRecord | null>(null);
  const [busyId, setBusyId] = useState(0);
  const targetsQuery = useLinkTargetsQuery(targetsLink?.id ?? null);

  const pageClass = useMemo(() => {
    const classes = ['links-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const onMove = useCallback(
    async (index: number, delta: number) => {
      const next = [...links];
      const target = index + delta;
      if (target < 0 || target >= next.length) return;
      [next[index], next[target]] = [next[target], next[index]];
      const msg = await reorder(next.map((row) => row.id));
      if (msg?.success) messageApi.success(t('pages.links.reordered'));
    },
    [links, reorder, messageApi, t],
  );

  const onToggle = useCallback(
    async (row: LinkRecord, enable: boolean) => {
      setBusyId(row.id);
      try {
        const msg = await setEnable(row.id, enable);
        if (!msg?.success && msg?.msg) messageApi.error(msg.msg);
      } finally {
        setBusyId(0);
      }
    },
    [setEnable, messageApi],
  );

  const onRefresh = useCallback(
    async (row: LinkRecord) => {
      setBusyId(row.id);
      try {
        const msg = await refresh(row.id);
        if (msg?.success) {
          const obj = msg.obj as { count?: number } | null | undefined;
          messageApi.success(t('pages.links.refreshed', { count: obj?.count ?? 0 }));
        } else if (msg?.msg) {
          messageApi.error(msg.msg);
        }
      } finally {
        setBusyId(0);
      }
    },
    [refresh, messageApi, t],
  );

  const onDelete = useCallback(
    (row: LinkRecord) => {
      modal.confirm({
        title: t('pages.links.deleteConfirmTitle', { value: row.remark || row.value }),
        okText: t('delete'),
        okType: 'danger',
        cancelText: t('cancel'),
        onOk: async () => {
          const msg = await remove(row.id);
          if (msg?.success) messageApi.success(t('pages.links.deleted'));
        },
      });
    },
    [modal, remove, messageApi, t],
  );

  const columns: TableColumnsType<LinkRecord> = useMemo(
    () => [
      {
        title: '',
        key: 'order',
        width: 72,
        render: (_value, _row, index) => (
          <Space size={2}>
            <Button
              size="small"
              type="text"
              aria-label={t('pages.links.moveUp')}
              disabled={index === 0}
              icon={<ArrowUpOutlined />}
              onClick={() => void onMove(index, -1)}
            />
            <Button
              size="small"
              type="text"
              aria-label={t('pages.links.moveDown')}
              disabled={index === links.length - 1}
              icon={<ArrowDownOutlined />}
              onClick={() => void onMove(index, 1)}
            />
          </Space>
        ),
      },
      {
        title: t('pages.links.kind'),
        dataIndex: 'kind',
        width: 130,
        render: (kind: LinkRecord['kind']) => (
          <Tag icon={kind === 'subscription' ? <ShareAltOutlined /> : <LinkOutlined />}>
            {t(kind === 'subscription' ? 'pages.links.kindSubscription' : 'pages.links.kindLink')}
          </Tag>
        ),
      },
      {
        title: t('pages.links.value'),
        dataIndex: 'value',
        render: (value: string) => (
          <Typography.Text ellipsis={{ tooltip: value }} style={{ maxWidth: 320 }}>
            {value}
          </Typography.Text>
        ),
      },
      {
        title: t('remark'),
        dataIndex: 'remark',
        width: 160,
        render: (remark: string) => remark || <Typography.Text type="secondary">-</Typography.Text>,
      },
      {
        title: t('pages.links.clients'),
        dataIndex: 'assignedClients',
        width: 90,
        align: 'center',
        render: (count: number) => count,
      },
      {
        title: t('pages.links.fetch'),
        key: 'fetch',
        width: 220,
        render: (_value, row) => {
          if (row.kind !== 'subscription')
            return <Typography.Text type="secondary">-</Typography.Text>;
          if (!row.lastFetchAt) {
            return (
              <Typography.Text type="secondary">{t('pages.links.fetchNever')}</Typography.Text>
            );
          }
          return row.lastFetchError ? (
            <Tooltip title={row.lastFetchError}>
              <Typography.Text type="danger">
                {t('pages.links.fetchFailed', { error: row.lastFetchError })}
              </Typography.Text>
            </Tooltip>
          ) : (
            <Typography.Text type="secondary">
              {IntlUtil.formatDate(row.lastFetchAt, datepicker)}
            </Typography.Text>
          );
        },
      },
      {
        title: t('status'),
        key: 'enable',
        width: 100,
        render: (_value, row) => (
          <Button
            size="small"
            type={row.enable === false ? 'default' : 'primary'}
            loading={busyId === row.id}
            onClick={() => void onToggle(row, row.enable === false)}
          >
            {t(row.enable === false ? 'disabled' : 'enabled')}
          </Button>
        ),
      },
      {
        title: '',
        key: 'actions',
        width: 190,
        render: (_value, row) => (
          <Space size={4}>
            <Tooltip title={t('pages.links.targets')}>
              <Button
                size="small"
                icon={<ShareAltOutlined />}
                onClick={() => setTargetsLink(row)}
                aria-label={t('pages.links.targets')}
              />
            </Tooltip>
            {row.kind === 'subscription' && (
              <Tooltip title={t('refresh')}>
                <Button
                  size="small"
                  icon={<ReloadOutlined />}
                  loading={busyId === row.id}
                  onClick={() => void onRefresh(row)}
                  aria-label={t('refresh')}
                />
              </Tooltip>
            )}
            <Tooltip title={t('edit')}>
              <Button
                size="small"
                icon={<EditOutlined />}
                aria-label={t('edit')}
                onClick={() => {
                  setFormLink(row);
                  setFormOpen(true);
                }}
              />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button
                size="small"
                danger
                icon={<DeleteOutlined />}
                aria-label={t('delete')}
                onClick={() => onDelete(row)}
              />
            </Tooltip>
          </Space>
        ),
      },
    ],
    [t, links.length, busyId, datepicker, onMove, onToggle, onRefresh, onDelete],
  );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={
                    <Button type="primary" loading={loading} onClick={() => void refetch()}>
                      {t('refresh')}
                    </Button>
                  }
                />
              ) : (
                <Card
                  size="small"
                  hoverable
                  title={t('pages.links.title')}
                  extra={
                    <Button
                      type="primary"
                      icon={<PlusOutlined />}
                      onClick={() => {
                        setFormLink(null);
                        setFormOpen(true);
                      }}
                    >
                      {t('pages.links.add')}
                    </Button>
                  }
                >
                  <Typography.Paragraph type="secondary">
                    {t('pages.links.hint')}
                  </Typography.Paragraph>
                  <Table<LinkRecord>
                    rowKey="id"
                    size="small"
                    loading={loading}
                    dataSource={links}
                    columns={columns}
                    pagination={false}
                    scroll={isMobile ? { x: 900 } : undefined}
                    locale={{ emptyText: t('pages.links.empty') }}
                  />
                </Card>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
      <LinkFormModal
        open={formOpen}
        link={formLink}
        save={save}
        onOpenChange={(open) => {
          setFormOpen(open);
          if (!open) setFormLink(null);
        }}
      />
      <LinkTargetsModal
        open={targetsLink !== null}
        link={targetsLink}
        targets={targetsQuery.targets}
        loading={targetsQuery.loading}
        assign={(payload) => assign(targetsLink?.id ?? 0, payload)}
        unassign={(payload) => unassign(targetsLink?.id ?? 0, payload)}
        onOpenChange={(open) => {
          if (!open) setTargetsLink(null);
        }}
      />
    </ConfigProvider>
  );
}
