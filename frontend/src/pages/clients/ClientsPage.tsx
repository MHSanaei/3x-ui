import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Checkbox,
  Col,
  ConfigProvider,
  Dropdown,
  Input,
  Layout,
  Modal,
  Pagination,
  Popover,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import type { ColumnsType, TableProps } from 'antd/es/table';
import {
  ClockCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  FilterOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  PlusOutlined,
  QrcodeOutlined,
  RestOutlined,
  RetweetOutlined,
  SearchOutlined,
  TeamOutlined,
  UserOutlined,
  UsergroupAddOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useClients } from '@/hooks/useClients';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import AppSidebar from '@/components/AppSidebar';
import { IntlUtil, SizeFormatter } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import LazyMount from '@/components/LazyMount';
const ClientFormModal = lazy(() => import('./ClientFormModal'));
const ClientInfoModal = lazy(() => import('./ClientInfoModal'));
const ClientQrModal = lazy(() => import('./ClientQrModal'));
const ClientBulkAddModal = lazy(() => import('./ClientBulkAddModal'));
const ClientBulkAdjustModal = lazy(() => import('./ClientBulkAdjustModal'));
import './ClientsPage.css';

const FILTER_STATE_KEY = 'clientsFilterState';

type Bucket = 'active' | 'deactive' | 'depleted' | 'expiring';

interface FilterState {
  enableFilter: boolean;
  searchKey: string;
  filterBy: string;
  protocolFilter?: string;
  inboundFilter?: number;
}

const INBOUND_PROTOCOL_COLORS: Record<string, string> = {
  vless: 'blue',
  vmess: 'geekblue',
  trojan: 'volcano',
  shadowsocks: 'magenta',
  hysteria: 'cyan',
  hysteria2: 'green',
  wireguard: 'gold',
  http: 'purple',
  mixed: 'lime',
  tunnel: 'orange',
};
const INBOUND_CHIP_LIMIT = 1;

function readFilterState(): FilterState {
  try {
    const raw = JSON.parse(localStorage.getItem(FILTER_STATE_KEY) || '{}');
    const inb = typeof raw.inboundFilter === 'number' && raw.inboundFilter > 0 ? raw.inboundFilter : undefined;
    return {
      enableFilter: !!raw.enableFilter,
      searchKey: raw.searchKey || '',
      filterBy: raw.filterBy || '',
      protocolFilter: raw.protocolFilter,
      inboundFilter: inb,
    };
  } catch {
    return { enableFilter: false, searchKey: '', filterBy: '', protocolFilter: undefined, inboundFilter: undefined };
  }
}

export default function ClientsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const {
    clients, filtered,
    summary: serverSummary,
    setQuery,
    inbounds, onlines, loading, fetched, subSettings,
    ipLimitEnable, tgBotEnable, expireDiff, trafficDiff, pageSize,
    create, update, remove, bulkDelete, bulkAdjust, attach, detach,
    resetTraffic, resetAllTraffics, delDepleted, setEnable,
    applyTrafficEvent, applyClientStatsEvent,
    hydrate,
  } = useClients();

  useWebSocket({
    traffic: applyTrafficEvent,
    client_stats: applyClientStatsEvent,
  });

  const [togglingEmail, setTogglingEmail] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [editingClient, setEditingClient] = useState<ClientRecord | null>(null);
  const [editingAttachedIds, setEditingAttachedIds] = useState<number[]>([]);
  const [infoOpen, setInfoOpen] = useState(false);
  const [infoClient, setInfoClient] = useState<ClientRecord | null>(null);
  const [qrOpen, setQrOpen] = useState(false);
  const [qrClient, setQrClient] = useState<ClientRecord | null>(null);
  const [bulkAddOpen, setBulkAddOpen] = useState(false);
  const [bulkAdjustOpen, setBulkAdjustOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  const initial = readFilterState();
  const [enableFilter, setEnableFilter] = useState(initial.enableFilter);
  const [searchKey, setSearchKey] = useState(initial.searchKey);
  const [filterBy, setFilterBy] = useState(initial.filterBy);
  const [protocolFilter, setProtocolFilter] = useState<string | undefined>(initial.protocolFilter);
  const [inboundFilter, setInboundFilter] = useState<number | undefined>(initial.inboundFilter);

  const [sortColumn, setSortColumn] = useState<string | null>(null);
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(25);
  // debouncedSearch lags behind the input so we don't spam the server on every
  // keystroke; the search box still feels instant locally.
  const [debouncedSearch, setDebouncedSearch] = useState(searchKey);

  useEffect(() => {
    localStorage.setItem(FILTER_STATE_KEY, JSON.stringify({
      enableFilter, searchKey, filterBy, protocolFilter, inboundFilter,
    }));
  }, [enableFilter, searchKey, filterBy, protocolFilter, inboundFilter]);

  useEffect(() => {
    const handle = window.setTimeout(() => setDebouncedSearch(searchKey), 300);
    return () => window.clearTimeout(handle);
  }, [searchKey]);

  useEffect(() => {
    // Reset to page 1 whenever a filter or sort changes — otherwise an empty
    // result set on a high page number leaves the user staring at "no clients".
    setCurrentPage(1);
  }, [debouncedSearch, enableFilter, filterBy, protocolFilter, inboundFilter, sortColumn, sortOrder]);

  useEffect(() => {
    setQuery({
      page: currentPage,
      pageSize: tablePageSize,
      search: enableFilter ? '' : debouncedSearch,
      filter: enableFilter ? (filterBy || '') : '',
      protocol: protocolFilter || '',
      inbound: inboundFilter,
      sort: sortColumn || undefined,
      order: sortOrder || undefined,
    });
  }, [setQuery, currentPage, tablePageSize, enableFilter, debouncedSearch, filterBy, protocolFilter, inboundFilter, sortColumn, sortOrder]);

  useEffect(() => {
    if (pageSize > 0) {

      setTablePageSize(pageSize);
    }
  }, [pageSize]);

  const onlineSet = useMemo(() => new Set(onlines || []), [onlines]);
  const inboundsById = useMemo(() => {
    const out: Record<number, InboundOption> = {};
    for (const ib of inbounds) out[ib.id] = ib;
    return out;
  }, [inbounds]);

  const protocolOptions = useMemo(() => {
    const values = new Set<string>((inbounds || []).map((i) => i.protocol).filter((x): x is string => !!x));
    return [...values].sort();
  }, [inbounds]);

  const isOnline = useCallback((email: string) => !!email && onlineSet.has(email), [onlineSet]);

  function inboundLabel(id: number) {
    const ib = inboundsById[id];
    if (!ib) return `#${id}`;
    return ib.remark ? `${ib.remark} (${ib.protocol}:${ib.port})` : `${ib.protocol}:${ib.port}`;
  }

  const clientBucket = useCallback((row: ClientRecord | null | undefined): Bucket | null => {
    if (!row) return null;
    const traffic = row.traffic || {};
    const used = (traffic.up || 0) + (traffic.down || 0);
    const total = row.totalGB || 0;
    const now = Date.now();
    const expired = (row.expiryTime ?? 0) > 0 && (row.expiryTime ?? 0) <= now;
    const exhausted = total > 0 && used >= total;
    if (expired || exhausted) return 'depleted';
    if (!row.enable) return 'deactive';
    const nearExpiry = (row.expiryTime ?? 0) > 0 && (row.expiryTime ?? 0) - now < (expireDiff || 0);
    const nearLimit = total > 0 && total - used < (trafficDiff || 0);
    if (nearExpiry || nearLimit) return 'expiring';
    return 'active';
  }, [expireDiff, trafficDiff]);

  function bucketBadgeStatus(bucket: Bucket | null): 'success' | 'warning' | 'error' | 'default' {
    switch (bucket) {
      case 'depleted': return 'error';
      case 'expiring': return 'warning';
      case 'active': return 'success';
      default: return 'default';
    }
  }

  // The list page renders rows the server already sorted, filtered, and
  // paginated. Local filtering is gone — keep the variable name so the rest
  // of the file (table dataSource, mobile cards, select-all) doesn't need
  // a rename.
  const filteredClients = clients;

  // Server-computed counts that stay stable as the user paginates/filters.
  const summary = serverSummary;

  // Sort is server-side now; the page already arrives in the requested
  // order, so we just hand it through.
  const sortedClients = filteredClients;

  function trafficLabel(row: ClientRecord) {
    const t0 = row.traffic;
    if (!t0) return '-';
    const used = (t0.up || 0) + (t0.down || 0);
    const total = row.totalGB || 0;
    if (total <= 0) return `${SizeFormatter.sizeFormat(used)} / ∞`;
    return `${SizeFormatter.sizeFormat(used)} / ${SizeFormatter.sizeFormat(total)}`;
  }

  function remainingLabel(row: ClientRecord) {
    const total = row.totalGB || 0;
    if (total <= 0) return '∞';
    const used = (row.traffic?.up || 0) + (row.traffic?.down || 0);
    const r = total - used;
    return r > 0 ? SizeFormatter.sizeFormat(r) : '0';
  }

  function remainingColor(row: ClientRecord): string {
    const total = row.totalGB || 0;
    if (total <= 0) return 'purple';
    const used = (row.traffic?.up || 0) + (row.traffic?.down || 0);
    const ratio = used / total;
    if (ratio >= 1) return 'red';
    if (ratio >= 0.85) return 'orange';
    return 'green';
  }

  function expiryLabel(row: ClientRecord) {
    if (!row.expiryTime) return '∞';
    if (row.expiryTime < 0) {
      const days = Math.round(row.expiryTime / -86400000);
      return `${t('pages.clients.delayedStart')}: ${days}d`;
    }
    return IntlUtil.formatDate(row.expiryTime, datepicker);
  }

  function expiryRelative(row: ClientRecord) {
    if (!row.expiryTime) return '';
    if (row.expiryTime < 0) {
      const days = Math.round(row.expiryTime / -86400000);
      return `${days}d`;
    }
    return IntlUtil.formatRelativeTime(row.expiryTime);
  }

  function expiryColor(row: ClientRecord): string {
    if (!row.expiryTime) return 'purple';
    if (row.expiryTime < 0) return 'blue';
    const now = Date.now();
    if (row.expiryTime <= now) return 'red';
    if (row.expiryTime - now < 86400 * 1000 * 3) return 'orange';
    return 'green';
  }

  async function onToggleEnable(row: ClientRecord, next: boolean) {
    setTogglingEmail(row.email);
    try {
      const msg = await setEnable(row, next);
      if (!msg?.success) {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setTogglingEmail(null);
    }
  }

  function onAdd() {
    setFormMode('add');
    setEditingClient(null);
    setEditingAttachedIds([]);
    setFormOpen(true);
  }

  async function onEdit(row: ClientRecord) {
    setFormMode('edit');
    // Paged list omits per-client secrets to keep the row payload tiny;
    // edit needs them, so fetch the full record first.
    const full = await hydrate(row.email);
    const merged: ClientRecord = full ? { ...row, ...full.client } : { ...row };
    setEditingClient(merged);
    const ids = full?.inboundIds ?? (Array.isArray(row.inboundIds) ? row.inboundIds : []);
    setEditingAttachedIds([...ids]);
    setFormOpen(true);
  }

  function onDelete(row: ClientRecord) {
    modal.confirm({
      title: t('pages.clients.deleteConfirmTitle', { email: row.email }),
      content: t('pages.clients.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(row.email);
        if (msg?.success) messageApi.success(t('pages.clients.toasts.deleted'));
      },
    });
  }

  function onResetTraffic(row: ClientRecord) {
    if (!row?.email || !Array.isArray(row.inboundIds) || row.inboundIds.length === 0) {
      messageApi.warning(t('pages.clients.resetNotPossible'));
      return;
    }
    modal.confirm({
      title: `${t('pages.inbounds.resetTraffic')} — ${row.email}`,
      content: t('pages.inbounds.resetTrafficContent'),
      okText: t('reset'),
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await resetTraffic(row);
        if (msg?.success) messageApi.success(t('pages.clients.toasts.trafficReset'));
      },
    });
  }

  async function onShowInfo(row: ClientRecord) {
    const full = await hydrate(row.email);
    setInfoClient(full ? { ...row, ...full.client, inboundIds: full.inboundIds } : row);
    setInfoOpen(true);
  }

  async function onShowQr(row: ClientRecord) {
    const full = await hydrate(row.email);
    setQrClient(full ? { ...row, ...full.client, inboundIds: full.inboundIds } : row);
    setQrOpen(true);
  }

  function onResetAllTraffics() {
    modal.confirm({
      title: t('pages.clients.resetAllTrafficsTitle'),
      content: t('pages.clients.resetAllTrafficsContent'),
      okText: t('reset'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await resetAllTraffics();
        if (msg?.success) messageApi.success(t('pages.clients.toasts.allTrafficsReset'));
      },
    });
  }

  function onDelDepleted() {
    modal.confirm({
      title: t('pages.clients.delDepletedConfirmTitle'),
      content: t('pages.clients.delDepletedConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await delDepleted();
        if (msg?.success) {
          const deleted = msg.obj?.deleted ?? 0;
          messageApi.success(t('pages.clients.toasts.delDepleted', { count: deleted }));
        }
      },
    });
  }

  function onBulkDelete() {
    const emails = [...selectedRowKeys];
    if (emails.length === 0) return;
    modal.confirm({
      title: t('pages.clients.bulkDeleteConfirmTitle', { count: emails.length }),
      content: t('pages.clients.bulkDeleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await bulkDelete(emails);
        setSelectedRowKeys([]);
        const ok = msg?.obj?.deleted ?? 0;
        const skipped = msg?.obj?.skipped ?? [];
        const failed = skipped.length;
        const firstError = skipped[0]?.reason ?? msg?.msg ?? '';
        if (failed === 0 && msg?.success) {
          messageApi.success(t('pages.clients.toasts.bulkDeleted', { count: ok }));
        } else {
          messageApi.warning(firstError
            ? `${t('pages.clients.toasts.bulkDeletedMixed', { ok, failed })} — ${firstError}`
            : t('pages.clients.toasts.bulkDeletedMixed', { ok, failed }));
        }
      },
    });
  }

  const onSave = useCallback(async (
    payload: Record<string, unknown> | { client: Record<string, unknown>; inboundIds: number[] },
    meta: { isEdit: false } | { isEdit: true; email: string; attach: number[]; detach: number[] },
  ) => {
    if (!meta.isEdit) {
      return create(payload);
    }
    const updateMsg = await update(meta.email, payload);
    if (!updateMsg?.success) return updateMsg;
    if (Array.isArray(meta.attach) && meta.attach.length > 0) {
      const r = await attach(meta.email, meta.attach);
      if (!r?.success) return r;
    }
    if (Array.isArray(meta.detach) && meta.detach.length > 0) {
      const r = await detach(meta.email, meta.detach);
      if (!r?.success) return r;
    }
    return updateMsg;
  }, [create, update, attach, detach]);

  const pageClass = useMemo(() => {
    const classes = ['clients-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const onTableChange: NonNullable<TableProps<ClientRecord>['onChange']> = (pag, _filters, sorter) => {
    if (pag?.current) setCurrentPage(pag.current);
    if (pag?.pageSize) setTablePageSize(pag.pageSize);
    const s = Array.isArray(sorter) ? sorter[0] : sorter;
    setSortColumn((s?.columnKey as string) || (s?.field as string) || null);
    setSortOrder((s?.order as 'ascend' | 'descend' | null) || null);
  };

  const columns = useMemo<ColumnsType<ClientRecord>>(() => {
    function sortableCol<T extends ColumnsType<ClientRecord>[number]>(col: T, key: string): T {
      return {
        ...col,
        sorter: true,
        showSorterTooltip: false,
        sortOrder: sortColumn === key ? sortOrder : null,
        sortDirections: ['ascend', 'descend'],
      };
    }
    return [
      {
        title: t('pages.clients.actions'),
        key: 'actions',
        width: 200,
        render: (_v, record) => (
          <Space size={4}>
            <Tooltip title={t('pages.clients.qrCode')}>
              <Button size="small" type="text" icon={<QrcodeOutlined />} onClick={() => onShowQr(record)} />
            </Tooltip>
            <Tooltip title={t('pages.clients.moreInformation')}>
              <Button size="small" type="text" icon={<InfoCircleOutlined />} onClick={() => onShowInfo(record)} />
            </Tooltip>
            <Tooltip title={t('pages.inbounds.resetTraffic')}>
              <Button size="small" type="text" icon={<RetweetOutlined />} onClick={() => onResetTraffic(record)} />
            </Tooltip>
            <Tooltip title={t('edit')}>
              <Button size="small" type="text" icon={<EditOutlined />} onClick={() => onEdit(record)} />
            </Tooltip>
            <Tooltip title={t('delete')}>
              <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={() => onDelete(record)} />
            </Tooltip>
          </Space>
        ),
      },
      sortableCol({
        title: t('pages.clients.enabled'), key: 'enable', width: 80,
        render: (_v, record) => (
          <Switch
            checked={!!record.enable}
            size="small"
            loading={togglingEmail === record.email}
            onChange={(next) => onToggleEnable(record, next)}
          />
        ),
      }, 'enable'),
      {
        title: t('pages.clients.online'),
        key: 'online',
        width: 90,
        render: (_v, record) => {
          const bucket = clientBucket(record);
          if (bucket === 'depleted') return <Tag color="red">{t('depleted')}</Tag>;
          if (record.enable && isOnline(record.email)) return <Tag color="green">{t('pages.clients.online')}</Tag>;
          if (!record.enable) return <Tag>{t('disabled')}</Tag>;
          if (bucket === 'expiring') return <Tag color="orange">{t('depletingSoon')}</Tag>;
          return <Tag>{t('pages.clients.offline')}</Tag>;
        },
      },
      sortableCol({
        title: t('pages.clients.client'),
        key: 'email',
        render: (_v, record) => (
          <div className="email-cell">
            <span className="email">{record.email}</span>
            {record.subId && <span className="sub" title={record.subId}>{record.subId}</span>}
            {record.comment && <span className="sub" title={record.comment}>{record.comment}</span>}
          </div>
        ),
      }, 'email'),
      sortableCol({
        title: t('pages.clients.attachedInbounds'),
        key: 'inboundIds',
        width: 170,
        render: (_v, record) => {
          const ids = record.inboundIds || [];
          if (ids.length === 0) return <span style={{ color: 'rgba(0,0,0,0.45)' }}>—</span>;
          const visible = ids.slice(0, INBOUND_CHIP_LIMIT);
          const overflow = ids.slice(INBOUND_CHIP_LIMIT);
          const chip = (id: number, compact: boolean) => {
            const ib = inboundsById[id];
            const proto = (ib?.protocol || '').toLowerCase();
            const color = INBOUND_PROTOCOL_COLORS[proto] ?? 'default';
            const compactLabel = ib ? `${ib.protocol}:${ib.port}` : `#${id}`;
            return (
              <Tooltip key={id} title={inboundLabel(id)}>
                <Tag color={color} style={{ margin: 2 }}>
                  {compact ? compactLabel : inboundLabel(id)}
                </Tag>
              </Tooltip>
            );
          };
          return (
            <>
              {visible.map((id) => chip(id, true))}
              {overflow.length > 0 && (
                <Popover
                  trigger="click"
                  placement="bottomRight"
                  content={
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxWidth: 280, maxHeight: 280, overflowY: 'auto' }}>
                      {overflow.map((id) => chip(id, false))}
                    </div>
                  }
                >
                  <Tag color="default" style={{ margin: 2, cursor: 'pointer' }}>
                    +{overflow.length}
                  </Tag>
                </Popover>
              )}
            </>
          );
        },
      }, 'inboundIds'),
      sortableCol({
        title: t('pages.clients.traffic'),
        key: 'traffic',
        render: (_v, record) => trafficLabel(record),
      }, 'traffic'),
      sortableCol({
        title: t('pages.clients.remaining'),
        key: 'remaining',
        width: 130,
        render: (_v, record) => <Tag color={remainingColor(record)}>{remainingLabel(record)}</Tag>,
      }, 'remaining'),
      sortableCol({
        title: t('pages.clients.duration'),
        key: 'expiryTime',
        render: (_v, record) => (
          <Tooltip title={expiryLabel(record)}>
            <Tag color={expiryColor(record)}>{record.expiryTime ? expiryRelative(record) : '∞'}</Tag>
          </Tooltip>
        ),
      }, 'expiryTime'),
    ];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [t, togglingEmail, sortColumn, sortOrder, clientBucket, isOnline, inboundsById]);

  const tablePagination = {
    current: currentPage,
    pageSize: tablePageSize,
    total: filtered,
    showSizeChanger: filtered > 10,
    pageSizeOptions: ['10', '25', '50', '100', '200'],
    hideOnSinglePage: filtered <= tablePageSize,
    showTotal: (n: number) => `${n}`,
  };

  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => setSelectedRowKeys(keys as string[]),
  };

  function toggleSelect(email: string, checked: boolean) {
    setSelectedRowKeys((prev) => {
      const next = new Set(prev);
      if (checked) next.add(email); else next.delete(email);
      return Array.from(next);
    });
  }

  function selectAll(checked: boolean) {
    setSelectedRowKeys(checked ? filteredClients.map((c) => c.email) : []);
  }

  const allSelected = filteredClients.length > 0 && selectedRowKeys.length === filteredClients.length;
  const someSelected = selectedRowKeys.length > 0 && selectedRowKeys.length < filteredClients.length;

  function onToggleFilter(checked: boolean) {
    setEnableFilter(checked);
    if (checked) setSearchKey('');
    else setFilterBy('');
  }

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
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, 12]}>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic title={t('clients')} value={String(summary.total)} prefix={<TeamOutlined />} />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('online')}
                            open={summary.online.length ? undefined : false}
                            content={<div className="client-email-list">{summary.online.map((e) => <div key={e}>{e}</div>)}</div>}
                          >
                            <Statistic title={t('online')} value={String(summary.online.length)} prefix={<span className="dot dot-blue" />} />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('depleted')}
                            open={summary.depleted.length ? undefined : false}
                            content={<div className="client-email-list">{summary.depleted.map((e) => <div key={e}>{e}</div>)}</div>}
                          >
                            <Statistic title={t('depleted')} value={String(summary.depleted.length)} prefix={<span className="dot dot-red" />} />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('depletingSoon')}
                            open={summary.expiring.length ? undefined : false}
                            content={<div className="client-email-list">{summary.expiring.map((e) => <div key={e}>{e}</div>)}</div>}
                          >
                            <Statistic title={t('depletingSoon')} value={String(summary.expiring.length)} prefix={<span className="dot dot-orange" />} />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('disabled')}
                            open={summary.deactive.length ? undefined : false}
                            content={<div className="client-email-list">{summary.deactive.map((e) => <div key={e}>{e}</div>)}</div>}
                          >
                            <Statistic title={t('disabled')} value={String(summary.deactive.length)} prefix={<span className="dot dot-gray" />} />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic title={t('subscription.active')} value={String(summary.active)} prefix={<span className="dot dot-green" />} />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={
                        <div className="card-toolbar">
                          <Button type="primary" size="small" icon={<PlusOutlined />} onClick={onAdd}>
                            {!isMobile && t('pages.clients.addClients')}
                          </Button>
                          <Button size="small" icon={<UsergroupAddOutlined />} onClick={() => setBulkAddOpen(true)}>
                            {!isMobile && t('pages.clients.bulk')}
                          </Button>
                          {selectedRowKeys.length > 0 && (
                            <>
                              <Button size="small" icon={<ClockCircleOutlined />} onClick={() => setBulkAdjustOpen(true)}>
                                {t('pages.clients.adjustSelected', { count: selectedRowKeys.length })}
                              </Button>
                              <Button danger size="small" icon={<DeleteOutlined />} onClick={onBulkDelete}>
                                {t('pages.clients.deleteSelected', { count: selectedRowKeys.length })}
                              </Button>
                            </>
                          )}
                          <Button size="small" icon={<RetweetOutlined />} onClick={onResetAllTraffics}>
                            {!isMobile && t('pages.clients.resetAllTraffics')}
                          </Button>
                          <Button size="small" danger icon={<RestOutlined />} onClick={onDelDepleted}>
                            {!isMobile && t('pages.clients.delDepleted')}
                          </Button>
                        </div>
                      }
                    >
                      <div className={isMobile ? 'filter-bar mobile' : 'filter-bar'}>
                        <Switch
                          checked={enableFilter}
                          onChange={onToggleFilter}
                          checkedChildren={<SearchOutlined />}
                          unCheckedChildren={<FilterOutlined />}
                        />
                        {!enableFilter && (
                          <Input
                            value={searchKey}
                            onChange={(e) => setSearchKey(e.target.value)}
                            placeholder={t('search')}
                            autoFocus
                            size={isMobile ? 'small' : 'middle'}
                            style={{ maxWidth: 300 }}
                          />
                        )}
                        {enableFilter && (
                          <Radio.Group
                            value={filterBy}
                            onChange={(e) => setFilterBy(e.target.value)}
                            optionType="button"
                            buttonStyle="solid"
                            size={isMobile ? 'small' : 'middle'}
                          >
                            <Radio.Button value="">{t('none')}</Radio.Button>
                            <Radio.Button value="active">{t('subscription.active')}</Radio.Button>
                            <Radio.Button value="deactive">{t('disabled')}</Radio.Button>
                            <Radio.Button value="depleted">{t('depleted')}</Radio.Button>
                            <Radio.Button value="expiring">{t('depletingSoon')}</Radio.Button>
                            <Radio.Button value="online">{t('online')}</Radio.Button>
                          </Radio.Group>
                        )}
                        <Select
                          value={protocolFilter}
                          onChange={(v) => {
                            setProtocolFilter(v);
                            if (v && inboundFilter) {
                              const ib = inbounds.find((x) => x.id === inboundFilter);
                              if (!ib || ib.protocol !== v) setInboundFilter(undefined);
                            }
                          }}
                          allowClear
                          placeholder={t('pages.inbounds.protocol')}
                          size={isMobile ? 'small' : 'middle'}
                          style={{ width: 150 }}
                          options={protocolOptions.map((p) => ({ value: p, label: p }))}
                        />
                        <Select
                          value={inboundFilter}
                          onChange={(v) => setInboundFilter(v)}
                          allowClear
                          showSearch={{ optionFilterProp: 'label' }}
                          placeholder={t('inbounds')}
                          size={isMobile ? 'small' : 'middle'}
                          style={{ minWidth: 160, maxWidth: 240 }}
                          options={inbounds
                            .filter((ib) => !protocolFilter || ib.protocol === protocolFilter)
                            .map((ib) => ({
                              value: ib.id,
                              label: ib.remark
                                ? `${ib.remark} (${ib.protocol || ''}${ib.port ? `:${ib.port}` : ''})`
                                : `#${ib.id} ${ib.protocol || ''}${ib.port ? `:${ib.port}` : ''}`,
                            }))}
                        />
                      </div>

                      {!isMobile ? (
                        <Table<ClientRecord>
                          columns={columns}
                          dataSource={sortedClients}
                          loading={loading}
                          rowKey="email"
                          rowSelection={rowSelection}
                          pagination={tablePagination}
                          size="small"
                          scroll={{ x: 1200 }}
                          onChange={onTableChange}
                          locale={{
                            emptyText: (
                              <div className="clients-empty">
                                <UserOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                                <div>{t('pages.clients.empty')}</div>
                              </div>
                            ),
                          }}
                        />
                      ) : (
                        <Spin spinning={loading}>
                          <div className="client-cards">
                            {filteredClients.length > 0 && (
                              <div className="card-bulk-bar">
                                <Checkbox
                                  checked={allSelected}
                                  indeterminate={someSelected}
                                  onChange={(e) => selectAll(e.target.checked)}
                                >
                                  {t('pages.clients.selectAll')}
                                </Checkbox>
                                {selectedRowKeys.length > 0 && (
                                  <span className="bulk-count">{selectedRowKeys.length}</span>
                                )}
                              </div>
                            )}
                            {filteredClients.length === 0 && (
                              <div className="card-empty">
                                <UserOutlined style={{ fontSize: 28, opacity: 0.5 }} />
                                <div>{t('pages.clients.empty')}</div>
                              </div>
                            )}
                            {filteredClients.length > 0 && (
                              <div className="card-pagination">
                                <Pagination
                                  current={currentPage}
                                  pageSize={tablePageSize}
                                  total={filtered}
                                  showSizeChanger={filtered > 10}
                                  pageSizeOptions={['10', '25', '50', '100', '200']}
                                  hideOnSinglePage={filtered <= tablePageSize}
                                  size="small"
                                  showTotal={(n) => `${n}`}
                                  onChange={(p, s) => {
                                    setCurrentPage(p);
                                    if (s && s !== tablePageSize) setTablePageSize(s);
                                  }}
                                />
                              </div>
                            )}
                            {filteredClients.map((row) => {
                              const bucket = clientBucket(row);
                              return (
                                <div key={row.email} className={`client-card${selectedRowKeys.includes(row.email) ? ' is-selected' : ''}`}>
                                  <div className="card-head">
                                    <Checkbox
                                      checked={selectedRowKeys.includes(row.email)}
                                      onChange={(e) => toggleSelect(row.email, e.target.checked)}
                                    />
                                    <Badge status={bucketBadgeStatus(bucket)} />
                                    <span className="tag-name">{row.email}</span>
                                    {bucket === 'depleted' && <Tag color="red" className="status-tag">{t('depleted')}</Tag>}
                                    {bucket === 'expiring' && <Tag color="orange" className="status-tag">{t('depletingSoon')}</Tag>}
                                    <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                                      <Tooltip title={t('pages.clients.moreInformation')}>
                                        <InfoCircleOutlined className="row-action-trigger" onClick={() => onShowInfo(row)} />
                                      </Tooltip>
                                      <Switch
                                        checked={!!row.enable}
                                        size="small"
                                        loading={togglingEmail === row.email}
                                        onChange={(next) => onToggleEnable(row, next)}
                                      />
                                      <Dropdown
                                        trigger={['click']}
                                        placement="bottomRight"
                                        menu={{
                                          items: [
                                            {
                                              key: 'qr',
                                              label: <><QrcodeOutlined /> {t('pages.clients.qrCode')}</>,
                                              onClick: () => onShowQr(row),
                                            },
                                            {
                                              key: 'reset',
                                              label: <><RetweetOutlined /> {t('pages.inbounds.resetTraffic')}</>,
                                              onClick: () => onResetTraffic(row),
                                            },
                                            {
                                              key: 'edit',
                                              label: <><EditOutlined /> {t('edit')}</>,
                                              onClick: () => onEdit(row),
                                            },
                                            {
                                              key: 'delete',
                                              danger: true,
                                              label: <><DeleteOutlined /> {t('delete')}</>,
                                              onClick: () => onDelete(row),
                                            },
                                          ],
                                        }}
                                      >
                                        <MoreOutlined className="row-action-trigger" />
                                      </Dropdown>
                                    </div>
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        </Spin>
                      )}
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <LazyMount when={formOpen}>
          <ClientFormModal
            open={formOpen}
            mode={formMode}
            client={editingClient}
            attachedIds={editingAttachedIds}
            inbounds={inbounds}
            ipLimitEnable={ipLimitEnable}
            tgBotEnable={tgBotEnable}
            save={onSave}
            onOpenChange={setFormOpen}
          />
        </LazyMount>
        <LazyMount when={infoOpen}>
          <ClientInfoModal
            open={infoOpen}
            client={infoClient}
            inboundsById={inboundsById}
            isOnline={infoClient ? isOnline(infoClient.email) : false}
            subSettings={subSettings}
            onOpenChange={setInfoOpen}
          />
        </LazyMount>
        <LazyMount when={qrOpen}>
          <ClientQrModal
            open={qrOpen}
            client={qrClient}
            subSettings={subSettings}
            onOpenChange={setQrOpen}
          />
        </LazyMount>
        <LazyMount when={bulkAddOpen}>
          <ClientBulkAddModal
            open={bulkAddOpen}
            inbounds={inbounds}
            ipLimitEnable={ipLimitEnable}
            onOpenChange={setBulkAddOpen}
            onSaved={() => setBulkAddOpen(false)}
          />
        </LazyMount>
        <LazyMount when={bulkAdjustOpen}>
          <ClientBulkAdjustModal
            open={bulkAdjustOpen}
            count={selectedRowKeys.length}
            onOpenChange={setBulkAdjustOpen}
            onSubmit={async (addDays, addBytes) => {
              const msg = await bulkAdjust([...selectedRowKeys], addDays, addBytes);
              if (msg?.success) {
                setSelectedRowKeys([]);
                return msg.obj ?? { adjusted: 0 };
              }
              return null;
            }}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}
