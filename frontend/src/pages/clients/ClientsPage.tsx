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
  Result,
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
  LinkOutlined,
  MoreOutlined,
  PlusOutlined,
  QrcodeOutlined,
  RestOutlined,
  RetweetOutlined,
  SearchOutlined,
  SortAscendingOutlined,
  TagsOutlined,
  TeamOutlined,
  UsergroupAddOutlined,
  UsergroupDeleteOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useClients } from '@/hooks/useClients';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import AppSidebar from '@/layouts/AppSidebar';
import { IntlUtil, SizeFormatter } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { LazyMount } from '@/components/utility';
const ClientFormModal = lazy(() => import('./ClientFormModal'));
const ClientInfoModal = lazy(() => import('./ClientInfoModal'));
const ClientQrModal = lazy(() => import('./ClientQrModal'));
const ClientBulkAddModal = lazy(() => import('./ClientBulkAddModal'));
const ClientBulkAdjustModal = lazy(() => import('./ClientBulkAdjustModal'));
const FilterDrawer = lazy(() => import('./FilterDrawer'));
const SubLinksModal = lazy(() => import('./SubLinksModal'));
const BulkAddToGroupModal = lazy(() => import('./BulkAddToGroupModal'));
const BulkAttachInboundsModal = lazy(() => import('./BulkAttachInboundsModal'));
const BulkDetachInboundsModal = lazy(() => import('./BulkDetachInboundsModal'));
import { emptyFilters, activeFilterCount } from './filters';
import type { ClientFilters } from './filters';
import './ClientsPage.css';

const FILTER_STATE_KEY = 'clientsFilterState';

function UngroupIcon() {
  return (
    <span
      style={{
        position: 'relative',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: '1em',
        height: '1em',
      }}
    >
      <TagsOutlined />
      <span
        aria-hidden="true"
        style={{
          position: 'absolute',
          inset: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          pointerEvents: 'none',
        }}
      >
        <span
          style={{
            display: 'block',
            width: '125%',
            height: '1.5px',
            background: 'currentColor',
            transform: 'rotate(-45deg)',
            borderRadius: '1px',
          }}
        />
      </span>
    </span>
  );
}

type Bucket = 'active' | 'deactive' | 'depleted' | 'expiring';

interface PersistedFilterState {
  searchKey: string;
  filters: ClientFilters;
  sort: string;
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

function readFilterState(): PersistedFilterState {
  try {
    const raw = JSON.parse(localStorage.getItem(FILTER_STATE_KEY) || '{}');
    const fromRaw = (raw.filters ?? {}) as Partial<ClientFilters>;
    return {
      searchKey: typeof raw.searchKey === 'string' ? raw.searchKey : '',
      filters: {
        ...emptyFilters(),
        ...fromRaw,
        buckets: Array.isArray(fromRaw.buckets) ? fromRaw.buckets : [],
        protocols: Array.isArray(fromRaw.protocols) ? fromRaw.protocols : [],
        inboundIds: Array.isArray(fromRaw.inboundIds) ? fromRaw.inboundIds : [],
        groups: Array.isArray(fromRaw.groups) ? fromRaw.groups : [],
      },
      sort: typeof raw.sort === 'string' ? raw.sort : '',
    };
  } catch {
    return { searchKey: '', filters: emptyFilters(), sort: '' };
  }
}

function gbToBytes(gb: number | undefined): number {
  if (!gb || gb <= 0) return 0;
  return Math.round(gb * 1024 * 1024 * 1024);
}

const SORT_OPTIONS: { value: string; column: string; order: 'ascend' | 'descend'; labelKey: string }[] = [
  { value: 'createdAt:ascend',    column: 'createdAt',  order: 'ascend',   labelKey: 'pages.clients.sortOldest' },
  { value: 'createdAt:descend',   column: 'createdAt',  order: 'descend',  labelKey: 'pages.clients.sortNewest' },
  { value: 'updatedAt:descend',   column: 'updatedAt',  order: 'descend',  labelKey: 'pages.clients.sortRecentlyUpdated' },
  { value: 'lastOnline:descend',  column: 'lastOnline', order: 'descend',  labelKey: 'pages.clients.sortRecentlyOnline' },
  { value: 'email:ascend',        column: 'email',      order: 'ascend',   labelKey: 'pages.clients.sortEmailAZ' },
  { value: 'email:descend',       column: 'email',      order: 'descend',  labelKey: 'pages.clients.sortEmailZA' },
  { value: 'traffic:descend',     column: 'traffic',    order: 'descend',  labelKey: 'pages.clients.sortMostTraffic' },
  { value: 'remaining:descend',   column: 'remaining',  order: 'descend',  labelKey: 'pages.clients.sortHighestRemaining' },
  { value: 'expiryTime:ascend',   column: 'expiryTime', order: 'ascend',   labelKey: 'pages.clients.sortExpiringSoonest' },
];

const DEFAULT_SORT = SORT_OPTIONS[0];

function sortValueFor(column: string | null, order: 'ascend' | 'descend' | null): string {
  if (!column || !order) return DEFAULT_SORT.value;
  return `${column}:${order}`;
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
    clients, total, filtered,
    summary: serverSummary,
    allGroups,
    setQuery,
    inbounds, onlines, loading, fetched, fetchError, subSettings,
    ipLimitEnable, tgBotEnable, expireDiff, trafficDiff, pageSize,
    create, update, remove, bulkDelete, bulkAdjust, bulkAddToGroup, bulkRemoveFromGroup, attach, bulkAttach, detach, bulkDetach,
    resetTraffic, resetAllTraffics, delDepleted, setEnable,
    applyTrafficEvent, applyClientStatsEvent,
    refresh,
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
  const [subLinksOpen, setSubLinksOpen] = useState(false);
  const [bulkGroupOpen, setBulkGroupOpen] = useState(false);
  const [bulkAttachOpen, setBulkAttachOpen] = useState(false);
  const [bulkDetachOpen, setBulkDetachOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  const initial = readFilterState();
  const [searchKey, setSearchKey] = useState(initial.searchKey);
  const [filters, setFilters] = useState<ClientFilters>(initial.filters);
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false);

  const initialSort = SORT_OPTIONS.find((o) => o.value === initial.sort) ?? DEFAULT_SORT;
  const [sortColumn, setSortColumn] = useState<string | null>(initialSort.column);
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(initialSort.order);
  const [currentPage, setCurrentPage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(25);
  // debouncedSearch lags behind the input so we don't spam the server on every
  // keystroke; the search box still feels instant locally.
  const [debouncedSearch, setDebouncedSearch] = useState(searchKey);

  useEffect(() => {
    localStorage.setItem(FILTER_STATE_KEY, JSON.stringify({ searchKey, filters, sort: sortValueFor(sortColumn, sortOrder) }));
  }, [searchKey, filters, sortColumn, sortOrder]);

  useEffect(() => {
    const handle = window.setTimeout(() => setDebouncedSearch(searchKey), 300);
    return () => window.clearTimeout(handle);
  }, [searchKey]);

  useEffect(() => {
    // Reset to page 1 whenever a filter or sort changes — otherwise an empty
    // result set on a high page number leaves the user staring at "no clients".
    setCurrentPage(1);
  }, [debouncedSearch, filters, sortColumn, sortOrder]);

  useEffect(() => {
    setQuery({
      page: currentPage,
      pageSize: tablePageSize,
      search: debouncedSearch,
      filter: filters.buckets.join(','),
      protocol: filters.protocols.join(','),
      inbound: filters.inboundIds.join(','),
      expiryFrom: filters.expiryFrom,
      expiryTo: filters.expiryTo,
      usageFrom: gbToBytes(filters.usageFromGB),
      usageTo: gbToBytes(filters.usageToGB),
      autoRenew: filters.autoRenew || undefined,
      hasTgId: filters.hasTgId || undefined,
      hasComment: filters.hasComment || undefined,
      group: filters.groups.join(',') || undefined,
      sort: sortColumn || undefined,
      order: sortOrder || undefined,
    });
  }, [setQuery, currentPage, tablePageSize, debouncedSearch, filters, sortColumn, sortOrder]);

  const activeCount = activeFilterCount(filters);

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

  const groupOptions = useMemo(() => {
    const values = new Set<string>(allGroups);
    for (const g of filters.groups) values.add(g);
    return [...values].sort((a, b) => a.localeCompare(b));
  }, [allGroups, filters.groups]);

  const isOnline = useCallback((email: string) => !!email && onlineSet.has(email), [onlineSet]);

  function inboundLabel(id: number) {
    const ib = inboundsById[id];
    return ib?.remark?.trim() || ib?.tag || '';
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

  function onBulkUngroup() {
    const emails = [...selectedRowKeys];
    if (emails.length === 0) return;
    modal.confirm({
      title: t('pages.clients.ungroupConfirmTitle', { count: emails.length }),
      content: t('pages.clients.ungroupConfirmContent'),
      okText: t('confirm'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await bulkRemoveFromGroup(emails);
        if (msg?.success) {
          setSelectedRowKeys([]);
          const affected = (msg.obj as { affected?: number } | undefined)?.affected ?? emails.length;
          messageApi.success(t('pages.clients.ungroupSuccessToast', { count: affected }));
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

  const onTableChange: NonNullable<TableProps<ClientRecord>['onChange']> = (pag) => {
    if (pag?.current) setCurrentPage(pag.current);
    if (pag?.pageSize) setTablePageSize(pag.pageSize);
  };

  const columns = useMemo<ColumnsType<ClientRecord>>(() => [
    {
      title: t('pages.clients.actions'),
      key: 'actions',
      width: 200,
      render: (_v, record) => (
        <Space size={4}>
          <Tooltip title={t('pages.clients.qrCode')}>
            <Button size="small" type="text" icon={<QrcodeOutlined />} onClick={() => onShowQr(record)} />
          </Tooltip>
          <Tooltip title={t('pages.clients.clientInfo')}>
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
    {
      title: t('pages.clients.enabled'),
      key: 'enable',
      width: 80,
      render: (_v, record) => (
        <Switch
          checked={!!record.enable}
          size="small"
          loading={togglingEmail === record.email}
          onChange={(next) => onToggleEnable(record, next)}
        />
      ),
    },
    {
      title: t('pages.clients.online'),
      key: 'online',
      width: 90,
      render: (_v, record) => {
        const bucket = clientBucket(record);
        const lastOnline = record.traffic?.lastOnline ?? 0;
        const lastOnlineTitle = `${t('lastOnline')}: ${lastOnline > 0 ? IntlUtil.formatDate(lastOnline, datepicker) : '-'}`;
        if (bucket === 'depleted') return (
          <Tooltip title={lastOnlineTitle}>
            <Tag color="red">{t('depleted')}</Tag>
          </Tooltip>
        );
        if (record.enable && isOnline(record.email)) return (
          <Tag color="green"><span className="online-dot" />{t('pages.clients.online')}</Tag>
        );
        if (!record.enable) return <Tag>{t('disabled')}</Tag>;
        if (bucket === 'expiring') return <Tag color="orange">{t('depletingSoon')}</Tag>;
        return (
          <Tooltip title={lastOnlineTitle}>
            <Tag>{t('pages.clients.offline')}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: t('pages.clients.client'),
      key: 'email',
      render: (_v, record) => (
        <div className="email-cell">
          <span className="email">{record.email}</span>
          {record.subId && <span className="sub" title={record.subId}>{record.subId}</span>}
          {record.comment && <span className="sub" title={record.comment}>{record.comment}</span>}
        </div>
      ),
    },
    {
      title: t('pages.clients.group'),
      key: 'group',
      width: 130,
      hidden: allGroups.length === 0,
      render: (_v, record) => {
        if (!record.group) return <span style={{ color: 'rgba(0,0,0,0.45)' }}>—</span>;
        const isActive = filters.groups.includes(record.group);
        return (
          <Tag
            color="geekblue"
            style={{ margin: 0, cursor: 'pointer', opacity: isActive ? 0.6 : 1 }}
            onClick={(e) => {
              e.stopPropagation();
              if (!isActive) {
                setFilters({ ...filters, groups: [...filters.groups, record.group!] });
              }
            }}
          >
            {record.group}
          </Tag>
        );
      },
    },
    {
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
          const compactLabel = ib?.remark?.trim() || ib?.tag || '';
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
    },
    {
      title: t('pages.clients.traffic'),
      key: 'traffic',
      render: (_v, record) => trafficLabel(record),
    },
    {
      title: t('pages.clients.remaining'),
      key: 'remaining',
      width: 130,
      render: (_v, record) => <Tag color={remainingColor(record)}>{remainingLabel(record)}</Tag>,
    },
    {
      title: t('pages.clients.duration'),
      key: 'expiryTime',
      render: (_v, record) => (
        <Tooltip title={expiryLabel(record)}>
          <Tag color={expiryColor(record)}>{record.expiryTime ? expiryRelative(record) : '∞'}</Tag>
        </Tooltip>
      ),
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
  ], [t, togglingEmail, clientBucket, isOnline, inboundsById, filters, allGroups, datepicker]);

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

  function clearOneFilter<K extends keyof ClientFilters>(key: K) {
    if (key === 'expiryFrom' || key === 'expiryTo') {
      setFilters({ ...filters, expiryFrom: undefined, expiryTo: undefined });
      return;
    }
    if (key === 'usageFromGB' || key === 'usageToGB') {
      setFilters({ ...filters, usageFromGB: undefined, usageToGB: undefined });
      return;
    }
    setFilters({ ...filters, [key]: emptyFilters()[key] });
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
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" loading={loading} onClick={refresh}>{t('refresh')}</Button>}
                />
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
                          {selectedRowKeys.length === 0 ? (
                            <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
                              {!isMobile && t('pages.clients.addClients')}
                            </Button>
                          ) : (
                            <>
                              <Tag
                                color="blue"
                                closable
                                onClose={() => setSelectedRowKeys([])}
                                style={{ marginInlineEnd: 0, padding: '4px 8px', fontSize: 13 }}
                              >
                                {t('pages.clients.selectedCount', { count: selectedRowKeys.length })}
                              </Tag>
                              <Button icon={<UsergroupAddOutlined />} onClick={() => setBulkAttachOpen(true)}>
                                {!isMobile && t('pages.clients.attach')}
                              </Button>
                              <Button danger icon={<UsergroupDeleteOutlined />} onClick={() => setBulkDetachOpen(true)}>
                                {!isMobile && t('pages.clients.detach')}
                              </Button>
                              <Button icon={<TagsOutlined />} onClick={() => setBulkGroupOpen(true)}>
                                {!isMobile && t('pages.clients.addToGroup')}
                              </Button>
                              <Button danger icon={<UngroupIcon />} onClick={onBulkUngroup}>
                                {!isMobile && t('pages.clients.ungroup')}
                              </Button>
                            </>
                          )}
                          <Dropdown
                            trigger={['click']}
                            placement="bottomRight"
                            menu={{
                              items: selectedRowKeys.length > 0
                                ? [
                                    {
                                      key: 'adjust',
                                      icon: <ClockCircleOutlined />,
                                      label: t('pages.clients.adjust'),
                                      onClick: () => setBulkAdjustOpen(true),
                                    },
                                    {
                                      key: 'subLinks',
                                      icon: <LinkOutlined />,
                                      label: t('pages.clients.subLinks'),
                                      onClick: () => setSubLinksOpen(true),
                                    },
                                  ]
                                : [
                                    {
                                      key: 'bulk',
                                      icon: <UsergroupAddOutlined />,
                                      label: t('pages.clients.bulk'),
                                      onClick: () => setBulkAddOpen(true),
                                    },
                                    {
                                      key: 'resetAll',
                                      icon: <RetweetOutlined />,
                                      label: t('pages.clients.resetAllTraffics'),
                                      onClick: onResetAllTraffics,
                                    },
                                    {
                                      key: 'delDepleted',
                                      icon: <RestOutlined />,
                                      label: t('pages.clients.delDepleted'),
                                      danger: true,
                                      onClick: onDelDepleted,
                                    },
                                  ],
                            }}
                          >
                            <Button icon={<MoreOutlined />}>
                              {!isMobile && t('more')}
                            </Button>
                          </Dropdown>
                          {selectedRowKeys.length > 0 && (
                            <Button
                              danger
                              icon={<DeleteOutlined />}
                              onClick={onBulkDelete}
                              style={{ marginInlineStart: 'auto' }}
                            >
                              {!isMobile && t('delete')}
                            </Button>
                          )}
                        </div>
                      }
                    >
                      <div className={isMobile ? 'filter-bar mobile' : 'filter-bar'}>
                        <Input
                          value={searchKey}
                          onChange={(e) => setSearchKey(e.target.value)}
                          placeholder={t('pages.clients.searchPlaceholder')}
                          allowClear
                          prefix={<SearchOutlined />}
                          size={isMobile ? 'small' : 'middle'}
                          style={{ maxWidth: 320 }}
                        />
                        <Badge count={activeCount} size="small" offset={[-4, 4]}>
                          <Button
                            icon={<FilterOutlined />}
                            size={isMobile ? 'small' : 'middle'}
                            onClick={() => setFilterDrawerOpen(true)}
                            type={activeCount > 0 ? 'primary' : 'default'}
                          >
                            {!isMobile && t('filter')}
                          </Button>
                        </Badge>
                        <Select
                          value={sortValueFor(sortColumn, sortOrder)}
                          size={isMobile ? 'small' : 'middle'}
                          suffixIcon={<SortAscendingOutlined />}
                          style={{ minWidth: isMobile ? 130 : 200 }}
                          onChange={(value) => {
                            const opt = SORT_OPTIONS.find((o) => o.value === value);
                            setSortColumn(opt?.column ?? null);
                            setSortOrder(opt?.order ?? null);
                          }}
                          options={SORT_OPTIONS.map((o) => ({ value: o.value, label: t(o.labelKey) }))}
                        />
                        {activeCount > 0 && (
                          <Button
                            size={isMobile ? 'small' : 'middle'}
                            onClick={() => setFilters(emptyFilters())}
                          >
                            {t('pages.clients.clearAllFilters')}
                          </Button>
                        )}
                        {(activeCount > 0 || debouncedSearch.trim().length > 0) && (
                          <span className="filter-count">
                            {t('pages.clients.showingCount', { shown: filtered, total })}
                          </span>
                        )}
                      </div>

                      {activeCount > 0 && (
                        <div className="filter-chips">
                          {filters.buckets.map((b) => (
                            <Tag
                              key={`b-${b}`}
                              closable
                              onClose={() => setFilters({ ...filters, buckets: filters.buckets.filter((x) => x !== b) })}
                            >
                              {bucketChipLabel(b, t)}
                            </Tag>
                          ))}
                          {filters.protocols.map((p) => (
                            <Tag
                              key={`p-${p}`}
                              closable
                              color="blue"
                              onClose={() => setFilters({ ...filters, protocols: filters.protocols.filter((x) => x !== p) })}
                            >
                              {p}
                            </Tag>
                          ))}
                          {filters.inboundIds.map((id) => (
                            <Tag
                              key={`i-${id}`}
                              closable
                              color="cyan"
                              onClose={() => setFilters({ ...filters, inboundIds: filters.inboundIds.filter((x) => x !== id) })}
                            >
                              {inboundLabel(id)}
                            </Tag>
                          ))}
                          {filters.groups.map((g) => (
                            <Tag
                              key={`g-${g}`}
                              closable
                              color="geekblue"
                              onClose={() => setFilters({ ...filters, groups: filters.groups.filter((x) => x !== g) })}
                            >
                              {t('pages.clients.group')}: {g}
                            </Tag>
                          ))}
                          {(filters.expiryFrom || filters.expiryTo) && (
                            <Tag closable color="purple" onClose={() => clearOneFilter('expiryFrom')}>
                              {t('pages.clients.expiryTime')}: {filters.expiryFrom ? IntlUtil.formatDate(filters.expiryFrom, datepicker) : '…'}
                              {' → '}
                              {filters.expiryTo ? IntlUtil.formatDate(filters.expiryTo, datepicker) : '…'}
                            </Tag>
                          )}
                          {(filters.usageFromGB || filters.usageToGB) && (
                            <Tag closable color="orange" onClose={() => clearOneFilter('usageFromGB')}>
                              {t('pages.clients.traffic')}: {filters.usageFromGB ?? 0}{filters.usageToGB ? `–${filters.usageToGB}` : '+'} GB
                            </Tag>
                          )}
                          {filters.autoRenew && (
                            <Tag closable color="gold" onClose={() => clearOneFilter('autoRenew')}>
                              {t('pages.clients.renew')}: {filters.autoRenew === 'on' ? t('enabled') : t('disabled')}
                            </Tag>
                          )}
                          {filters.hasTgId && (
                            <Tag closable onClose={() => clearOneFilter('hasTgId')}>
                              {t('pages.clients.telegramId')}: {filters.hasTgId === 'yes' ? t('pages.clients.has') : t('pages.clients.hasNot')}
                            </Tag>
                          )}
                          {filters.hasComment && (
                            <Tag closable onClose={() => clearOneFilter('hasComment')}>
                              {t('pages.clients.comment')}: {filters.hasComment === 'yes' ? t('pages.clients.has') : t('pages.clients.hasNot')}
                            </Tag>
                          )}
                        </div>
                      )}

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
                                <TeamOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                                <div>{t('noData')}</div>
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
                                <TeamOutlined style={{ fontSize: 28, opacity: 0.5 }} />
                                <div>{t('noData')}</div>
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
                                      <Tooltip title={t('pages.clients.clientInfo')}>
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
            groups={allGroups}
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
            groups={allGroups}
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
        <LazyMount when={subLinksOpen}>
          <SubLinksModal
            open={subLinksOpen}
            emails={selectedRowKeys}
            clients={clients}
            subSettings={subSettings}
            onOpenChange={setSubLinksOpen}
          />
        </LazyMount>
        <LazyMount when={bulkGroupOpen}>
          <BulkAddToGroupModal
            open={bulkGroupOpen}
            count={selectedRowKeys.length}
            groups={allGroups}
            onOpenChange={setBulkGroupOpen}
            onSubmit={async (group) => {
              const msg = await bulkAddToGroup([...selectedRowKeys], group);
              if (msg?.success) {
                setSelectedRowKeys([]);
                return (msg.obj as { affected?: number } | undefined) ?? { affected: 0 };
              }
              return null;
            }}
          />
        </LazyMount>
        <LazyMount when={bulkAttachOpen}>
          <BulkAttachInboundsModal
            open={bulkAttachOpen}
            count={selectedRowKeys.length}
            inbounds={inbounds}
            onOpenChange={setBulkAttachOpen}
            onSubmit={async (inboundIds) => {
              const msg = await bulkAttach([...selectedRowKeys], inboundIds);
              if (msg?.success) {
                setSelectedRowKeys([]);
                return msg.obj ?? { attached: [], skipped: [], errors: [] };
              }
              return null;
            }}
          />
        </LazyMount>
        <LazyMount when={bulkDetachOpen}>
          <BulkDetachInboundsModal
            open={bulkDetachOpen}
            count={selectedRowKeys.length}
            inbounds={inbounds}
            onOpenChange={setBulkDetachOpen}
            onSubmit={async (inboundIds) => {
              const msg = await bulkDetach([...selectedRowKeys], inboundIds);
              if (msg?.success) {
                setSelectedRowKeys([]);
                return msg.obj ?? { detached: [], skipped: [], errors: [] };
              }
              return null;
            }}
          />
        </LazyMount>
        <LazyMount when={filterDrawerOpen}>
          <FilterDrawer
            open={filterDrawerOpen}
            onOpenChange={setFilterDrawerOpen}
            filters={filters}
            onChange={setFilters}
            inbounds={inbounds}
            protocols={protocolOptions}
            groups={groupOptions}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}

function bucketChipLabel(b: string, t: (k: string) => string): string {
  switch (b) {
    case 'active': return t('subscription.active');
    case 'expiring': return t('depletingSoon');
    case 'depleted': return t('depleted');
    case 'deactive': return t('disabled');
    case 'online': return t('online');
    default: return b;
  }
}
