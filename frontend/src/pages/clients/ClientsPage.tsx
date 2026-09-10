import { lazy, useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
  CheckCircleOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  DownloadOutlined,
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
  StopOutlined,
  TagsOutlined,
  TeamOutlined,
  UploadOutlined,
  UsergroupAddOutlined,
  UsergroupDeleteOutlined,
} from '@ant-design/icons';
import { activateOnKey } from '@/utils/a11y';

import { useTheme } from '@/hooks/useTheme';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useClients } from '@/hooks/useClients';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import { useDatepicker } from '@/hooks/useDatepicker';
import type {
  ClientRecord,
  InboundOption,
  ExternalLink,
  ExternalLinkInput,
} from '@/hooks/useClients';
import ClientTrafficCell from '@/components/clients/ClientTrafficCell';
import ClientSpeedTag, { isActiveSpeed } from '@/components/clients/ClientSpeedTag';
import ClientCardComment from '@/components/clients/ClientCardComment';
import AppSidebar from '@/layouts/AppSidebar';
import { IntlUtil, SizeFormatter } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { LazyMount } from '@/components/utility';
import {
  SPEED_COLUMN_WIDTH,
  SPEED_TAG_CLASS_NAME,
  SPEED_TAG_STYLE,
} from '@/components/utility/speedTagStyle';
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
const TextModal = lazy(() => import('@/components/feedback/TextModal'));
const PromptModal = lazy(() => import('@/components/feedback/PromptModal'));
import { ClientInboundChips, ClientRowActions } from './RowCells';
import { emptyFilters, activeFilterCount } from './filters';
import type { ClientFilters } from './filters';
import './ClientsPage.css';

const FILTER_STATE_KEY = 'clientsFilterState';
const DISABLED_PAGE_SIZE = 200;
const DEFAULT_TABLE_PAGE_SIZE = 25;

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

// The server sends exact counters but caps the email arrays behind them, so a
// panel with thousands of depleted clients neither ships nor renders them all.
// The trailing chip reports what the popover left out.
function ClientEmailList({ emails, total }: { emails: string[]; total: number }) {
  const hidden = total - emails.length;
  return (
    <div className="client-email-list">
      {emails.map((e) => (
        <div key={e}>{e}</div>
      ))}
      {hidden > 0 && <div className="client-email-more">+{hidden}</div>}
    </div>
  );
}

type Bucket = 'active' | 'deactive' | 'depleted' | 'expiring';

interface PersistedFilterState {
  searchKey: string;
  filters: ClientFilters;
  sort: string;
  // The page size resolved on the previous visit. Without it the first list
  // request has to wait for /setting/defaultSettings just to learn how many rows
  // to ask for, which serialises two round trips on every load.
  pageSize: number | null;
}

const INBOUND_PROTOCOL_COLORS: Record<string, string> = {
  vless: 'blue',
  vmess: 'geekblue',
  trojan: 'volcano',
  shadowsocks: 'magenta',
  hysteria: 'cyan',
  hysteria2: 'green',
  wireguard: 'gold',
  amneziawg: 'yellow',
  http: 'purple',
  mixed: 'lime',
  tunnel: 'orange',
};
const INBOUND_CHIP_LIMIT = 1;
// A shared empty array keeps the memoised chip cell from seeing a fresh prop for
// every unattached client on every render.
const EMPTY_INBOUND_IDS: number[] = [];

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
        nodeIds: Array.isArray(fromRaw.nodeIds) ? fromRaw.nodeIds : [],
        groups: Array.isArray(fromRaw.groups) ? fromRaw.groups : [],
      },
      sort: typeof raw.sort === 'string' ? raw.sort : '',
      pageSize: typeof raw.pageSize === 'number' && raw.pageSize > 0 ? raw.pageSize : null,
    };
  } catch {
    return { searchKey: '', filters: emptyFilters(), sort: '', pageSize: null };
  }
}

function gbToBytes(gb: number | undefined): number {
  if (!gb || gb <= 0) return 0;
  return Math.round(gb * 1024 * 1024 * 1024);
}

const SORT_OPTIONS: {
  value: string;
  column: string;
  order: 'ascend' | 'descend';
  labelKey: string;
}[] = [
  {
    value: 'createdAt:ascend',
    column: 'createdAt',
    order: 'ascend',
    labelKey: 'pages.clients.sortOldest',
  },
  {
    value: 'createdAt:descend',
    column: 'createdAt',
    order: 'descend',
    labelKey: 'pages.clients.sortNewest',
  },
  {
    value: 'updatedAt:descend',
    column: 'updatedAt',
    order: 'descend',
    labelKey: 'pages.clients.sortRecentlyUpdated',
  },
  {
    value: 'lastOnline:descend',
    column: 'lastOnline',
    order: 'descend',
    labelKey: 'pages.clients.sortRecentlyOnline',
  },
  {
    value: 'email:ascend',
    column: 'email',
    order: 'ascend',
    labelKey: 'pages.clients.sortEmailAZ',
  },
  {
    value: 'email:descend',
    column: 'email',
    order: 'descend',
    labelKey: 'pages.clients.sortEmailZA',
  },
  {
    value: 'traffic:descend',
    column: 'traffic',
    order: 'descend',
    labelKey: 'pages.clients.sortMostTraffic',
  },
  {
    value: 'remaining:descend',
    column: 'remaining',
    order: 'descend',
    labelKey: 'pages.clients.sortHighestRemaining',
  },
  {
    value: 'expiryTime:ascend',
    column: 'expiryTime',
    order: 'ascend',
    labelKey: 'pages.clients.sortExpiringSoonest',
  },
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
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const {
    clients,
    total,
    filtered,
    summary,
    allGroups,
    setQuery,
    inbounds,
    onlines,
    transitioning,
    fetched,
    fetchError,
    subSettings,
    tgBotEnable,
    expireDiff,
    trafficDiff,
    pageSize,
    settingsReady,
    create,
    update,
    remove,
    bulkDelete,
    bulkAdjust,
    bulkEnable,
    bulkDisable,
    bulkAddToGroup,
    bulkRemoveFromGroup,
    attach,
    setExternalLinks,
    bulkAttach,
    detach,
    bulkDetach,
    resetTraffic,
    resetAllTraffics,
    delDepleted,
    delOrphans,
    exportClients,
    importClients,
    setEnable,
    clientSpeed,
    applyTrafficEvent,
    applyClientStatsEvent,
    refresh,
    hydrate,
  } = useClients();

  useWebSocket({
    traffic: applyTrafficEvent,
    client_stats: applyClientStatsEvent,
  });

  // Node list for the Nodes filter; the section only renders when the panel
  // actually manages nodes (#4997).
  const { nodes } = useNodesQuery();

  const [togglingEmail, setTogglingEmail] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [editingClient, setEditingClient] = useState<ClientRecord | null>(null);
  const [editingAttachedIds, setEditingAttachedIds] = useState<number[]>([]);
  const [editingExternalLinks, setEditingExternalLinks] = useState<ExternalLink[]>([]);
  const [editingTunnelAllowedIPs, setEditingTunnelAllowedIPs] = useState<Record<number, string>>(
    {},
  );
  const [infoOpen, setInfoOpen] = useState(false);
  const [infoClient, setInfoClient] = useState<ClientRecord | null>(null);
  const [qrOpen, setQrOpen] = useState(false);
  const [qrClient, setQrClient] = useState<ClientRecord | null>(null);
  const [viewingTunnelAllowedIPs, setViewingTunnelAllowedIPs] = useState<Record<number, string>>(
    {},
  );
  const [bulkAddOpen, setBulkAddOpen] = useState(false);
  const [bulkAdjustOpen, setBulkAdjustOpen] = useState(false);
  const [subLinksOpen, setSubLinksOpen] = useState(false);
  const [bulkGroupOpen, setBulkGroupOpen] = useState(false);
  const [bulkAttachOpen, setBulkAttachOpen] = useState(false);
  const [bulkDetachOpen, setBulkDetachOpen] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);

  const [textOpen, setTextOpen] = useState(false);
  const [textTitle, setTextTitle] = useState('');
  const [textContent, setTextContent] = useState('');
  const [textFileName, setTextFileName] = useState('');
  const [promptOpen, setPromptOpen] = useState(false);
  const [promptTitle, setPromptTitle] = useState('');
  const [promptOkText, setPromptOkText] = useState('');
  const [promptInitial, setPromptInitial] = useState('');
  const [promptLoading, setPromptLoading] = useState(false);
  const [promptHandler, setPromptHandler] = useState<
    ((value: string) => Promise<boolean | void> | boolean | void) | null
  >(null);

  const initial = readFilterState();
  const [searchKey, setSearchKey] = useState(initial.searchKey);
  const [filters, setFilters] = useState<ClientFilters>(initial.filters);
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false);

  const initialSort = SORT_OPTIONS.find((o) => o.value === initial.sort) ?? DEFAULT_SORT;
  const [sortColumn, setSortColumn] = useState<string | null>(initialSort.column);
  const [sortOrder, setSortOrder] = useState<'ascend' | 'descend' | null>(initialSort.order);
  const [currentPage, setCurrentPage] = useState(1);
  // Derived, not mirrored into state by an effect: an effect lags one render
  // behind the settings arriving, and that lag is what made the page fetch the
  // list once with the placeholder size and again with the real one.
  const [pageSizeChoice, setPageSizeChoice] = useState<number | null>(null);
  const settingsPageSize = settingsReady ? (pageSize > 0 ? pageSize : DISABLED_PAGE_SIZE) : null;
  // Last visit's resolved size stands in until the settings land, so the list
  // request goes out with the page mount instead of queueing behind them. If the
  // admin has since changed the setting the authoritative value replaces it and
  // costs one refetch — only on the load that follows the change. Null means
  // nothing is known yet, which is the one case worth waiting for.
  const resolvedPageSize = pageSizeChoice ?? settingsPageSize ?? initial.pageSize;
  const tablePageSize = resolvedPageSize ?? DEFAULT_TABLE_PAGE_SIZE;
  // debouncedSearch lags behind the input so we don't spam the server on every
  // keystroke; the search box still feels instant locally.
  const [debouncedSearch, setDebouncedSearch] = useState(searchKey);

  useEffect(() => {
    localStorage.setItem(
      FILTER_STATE_KEY,
      JSON.stringify({
        searchKey,
        filters,
        sort: sortValueFor(sortColumn, sortOrder),
        // Only ever persist a size we actually resolved, never the render fallback.
        pageSize: resolvedPageSize,
      }),
    );
  }, [searchKey, filters, sortColumn, sortOrder, resolvedPageSize]);

  useEffect(() => {
    const handle = window.setTimeout(() => setDebouncedSearch(searchKey), 300);
    return () => window.clearTimeout(handle);
  }, [searchKey]);

  useEffect(() => {
    // Reset to page 1 whenever a filter or sort changes — otherwise an empty
    // result set on a high page number leaves the user staring at "no clients".
    setCurrentPage(1);
  }, [debouncedSearch, filters, sortColumn, sortOrder]);

  // The node filter maps onto inbound ids client-side (#4997): the paging API
  // already accepts an inbound CSV, so nodes never have to reach the backend.
  // Sentinel 0 = "local panel" (inbounds without a nodeId).
  const effectiveInboundCsv = useMemo(() => {
    if (!filters.nodeIds.length) return filters.inboundIds.join(',');
    const nodeSet = new Set(filters.nodeIds);
    const nodeInboundIds = inbounds.filter((ib) => nodeSet.has(ib.nodeId ?? 0)).map((ib) => ib.id);
    const pool = filters.inboundIds.length
      ? nodeInboundIds.filter((id) => filters.inboundIds.includes(id))
      : nodeInboundIds;
    // Nothing matches the selected nodes: send an impossible id so the filter
    // yields an honest empty result instead of being silently ignored.
    return pool.length ? pool.join(',') : '-1';
  }, [filters.nodeIds, filters.inboundIds, inbounds]);

  useEffect(() => {
    // With no remembered size and no settings yet, any query we build would be a
    // guess, and issuing it costs a full server round trip that is thrown away as
    // soon as the real size arrives.
    if (resolvedPageSize === null) return;
    setQuery({
      page: currentPage,
      pageSize: tablePageSize,
      search: debouncedSearch,
      filter: filters.buckets.join(','),
      protocol: filters.protocols.join(','),
      inbound: effectiveInboundCsv,
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
  }, [
    setQuery,
    resolvedPageSize,
    currentPage,
    tablePageSize,
    debouncedSearch,
    filters,
    effectiveInboundCsv,
    sortColumn,
    sortOrder,
  ]);

  const activeCount = activeFilterCount(filters);

  // Row handlers take an email and look the row up here at call time. Keying
  // them on the record object instead would defeat the memoised cells: every
  // traffic push replaces the row object of every client whose counters moved,
  // so the memo would miss on exactly the rows that are busy. Reading through
  // the ref also means a modal opened mid-poll shows current usage.
  const rowsByEmail = useRef(new Map<string, ClientRecord>());
  rowsByEmail.current = useMemo(() => {
    const map = new Map<string, ClientRecord>();
    for (const c of clients) map.set(c.email, c);
    return map;
  }, [clients]);

  const onlineSet = useMemo(() => new Set(onlines || []), [onlines]);
  const inboundsById = useMemo(() => {
    const out: Record<number, InboundOption> = {};
    for (const ib of inbounds) out[ib.id] = ib;
    return out;
  }, [inbounds]);

  const protocolOptions = useMemo(() => {
    const values = new Set<string>(
      (inbounds || []).map((i) => i.protocol).filter((x): x is string => !!x),
    );
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
    return formatInboundLabel(ib?.tag, ib?.remark);
  }

  const clientBucket = useCallback(
    (row: ClientRecord | null | undefined): Bucket | null => {
      if (!row) return null;
      const traffic = row.traffic || {};
      const used = (traffic.up || 0) + (traffic.down || 0);
      const total = row.totalGB || 0;
      const now = Date.now();
      const expired = (row.expiryTime ?? 0) > 0 && (row.expiryTime ?? 0) <= now;
      const exhausted = total > 0 && used >= total;
      if (expired || exhausted) return 'depleted';
      if (!row.enable) return 'deactive';
      const nearExpiry =
        (row.expiryTime ?? 0) > 0 && (row.expiryTime ?? 0) - now < (expireDiff || 0);
      const nearLimit = total > 0 && total - used < (trafficDiff || 0);
      if (nearExpiry || nearLimit) return 'expiring';
      return 'active';
    },
    [expireDiff, trafficDiff],
  );

  function bucketBadgeStatus(bucket: Bucket | null): 'success' | 'warning' | 'error' | 'default' {
    switch (bucket) {
      case 'depleted':
        return 'error';
      case 'expiring':
        return 'warning';
      case 'active':
        return 'success';
      default:
        return 'default';
    }
  }

  // The list page renders rows the server already sorted, filtered, and
  // paginated. Local filtering is gone — keep the variable name so the rest
  // of the file (table dataSource, mobile cards, select-all) doesn't need
  // a rename.
  const filteredClients = clients;

  // Sort is server-side now; the page already arrives in the requested
  // order, so we just hand it through.
  const sortedClients = filteredClients;

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
    setEditingExternalLinks([]);
    setEditingTunnelAllowedIPs({});
    setFormOpen(true);
  }

  const onEdit = useCallback(
    async (email: string) => {
      const row = rowsByEmail.current.get(email);
      if (!row) return;
      setFormMode('edit');
      // Paged list omits per-client secrets to keep the row payload tiny;
      // edit needs them, so fetch the full record first.
      const full = await hydrate(row.email);
      const merged: ClientRecord = full ? { ...row, ...full.client } : { ...row };
      setEditingClient(merged);
      const ids = full?.inboundIds ?? (Array.isArray(row.inboundIds) ? row.inboundIds : []);
      setEditingAttachedIds([...ids]);
      setEditingExternalLinks(Array.isArray(full?.externalLinks) ? [...full.externalLinks] : []);
      setEditingTunnelAllowedIPs(full?.tunnelAllowedIPs ?? {});
      setFormOpen(true);
    },
    [hydrate],
  );

  const onDelete = useCallback(
    (email: string) => {
      const row = rowsByEmail.current.get(email);
      if (!row) return;
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
    },
    [modal, t, remove, messageApi],
  );

  const onResetTraffic = useCallback(
    (email: string) => {
      const row = rowsByEmail.current.get(email);
      if (!row?.email) {
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
    },
    [modal, t, resetTraffic, messageApi],
  );

  const onShowInfo = useCallback(
    async (email: string) => {
      const row = rowsByEmail.current.get(email);
      if (!row) return;
      const full = await hydrate(row.email);
      setInfoClient(full ? { ...row, ...full.client, inboundIds: full.inboundIds } : row);
      setViewingTunnelAllowedIPs(full?.tunnelAllowedIPs ?? {});
      setInfoOpen(true);
    },
    [hydrate],
  );

  const onShowQr = useCallback(
    async (email: string) => {
      const row = rowsByEmail.current.get(email);
      if (!row) return;
      const full = await hydrate(row.email);
      setQrClient(full ? { ...row, ...full.client, inboundIds: full.inboundIds } : row);
      setViewingTunnelAllowedIPs(full?.tunnelAllowedIPs ?? {});
      setQrOpen(true);
    },
    [hydrate],
  );

  const [refreshing, setRefreshing] = useState(false);
  const onRefreshClick = useCallback(async () => {
    setRefreshing(true);
    try {
      await refresh();
    } finally {
      setRefreshing(false);
    }
  }, [refresh]);

  const openText = useCallback((opts: { title: string; content: string; fileName?: string }) => {
    setTextTitle(opts.title);
    setTextContent(opts.content);
    setTextFileName(opts.fileName || '');
    setTextOpen(true);
  }, []);

  const openPrompt = useCallback(
    (opts: {
      title: string;
      okText?: string;
      value?: string;
      confirm: (value: string) => Promise<boolean | void> | boolean | void;
    }) => {
      setPromptTitle(opts.title);
      setPromptOkText(opts.okText || t('confirm'));
      setPromptInitial(opts.value || '');
      setPromptHandler(() => opts.confirm);
      setPromptOpen(true);
    },
    [t],
  );

  const onPromptConfirm = useCallback(
    async (value: string) => {
      if (!promptHandler) {
        setPromptOpen(false);
        return;
      }
      setPromptLoading(true);
      try {
        const ok = await promptHandler(value);
        if (ok !== false) setPromptOpen(false);
      } finally {
        setPromptLoading(false);
      }
    },
    [promptHandler],
  );

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

  function onDeleteOrphans() {
    modal.confirm({
      title: t('pages.clients.delOrphansConfirmTitle'),
      content: t('pages.clients.delOrphansConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await delOrphans();
        if (msg?.success) {
          const deleted = msg.obj?.deleted ?? 0;
          messageApi.success(t('pages.clients.toasts.delOrphans', { count: deleted }));
        }
      },
    });
  }

  async function onExportClients() {
    const items = await exportClients();
    if (!items) return;
    openText({
      title: t('pages.clients.exportClients'),
      content: JSON.stringify(items, null, 2),
      fileName: 'clients-export.json',
    });
  }

  function onImportClients() {
    openPrompt({
      title: t('pages.clients.importClients'),
      okText: t('pages.clients.import'),
      value: '',
      confirm: async (value) => {
        const msg = await importClients(value);
        if (!msg?.success) return false;
        const created = msg.obj?.created ?? 0;
        const skipped = msg.obj?.skipped ?? [];
        if (skipped.length === 0) {
          messageApi.success(t('pages.clients.toasts.imported', { count: created }));
        } else {
          const firstError = skipped[0]?.reason ?? '';
          messageApi.warning(
            firstError
              ? `${t('pages.clients.toasts.importedMixed', { ok: created, failed: skipped.length })} — ${firstError}`
              : t('pages.clients.toasts.importedMixed', { ok: created, failed: skipped.length }),
          );
        }
        return true;
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
          const affected =
            (msg.obj as { affected?: number } | undefined)?.affected ?? emails.length;
          messageApi.success(t('pages.clients.ungroupSuccessToast', { count: affected }));
        }
      },
    });
  }

  function onBulkSetEnable(enable: boolean) {
    const emails = [...selectedRowKeys];
    if (emails.length === 0) return;
    modal.confirm({
      title: t(
        enable ? 'pages.clients.bulkEnableConfirmTitle' : 'pages.clients.bulkDisableConfirmTitle',
        { count: emails.length },
      ),
      content: t(
        enable
          ? 'pages.clients.bulkEnableConfirmContent'
          : 'pages.clients.bulkDisableConfirmContent',
      ),
      okText: t('confirm'),
      okType: enable ? 'primary' : 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = enable ? await bulkEnable(emails) : await bulkDisable(emails);
        setSelectedRowKeys([]);
        const changed = msg?.obj?.changed ?? 0;
        const skipped = msg?.obj?.skipped ?? [];
        const failed = skipped.length;
        const firstError = skipped[0]?.reason ?? msg?.msg ?? '';
        const okKey = enable
          ? 'pages.clients.toasts.bulkEnabled'
          : 'pages.clients.toasts.bulkDisabled';
        const mixedKey = enable
          ? 'pages.clients.toasts.bulkEnabledMixed'
          : 'pages.clients.toasts.bulkDisabledMixed';
        if (failed === 0 && msg?.success) {
          messageApi.success(t(okKey, { count: changed }));
        } else {
          messageApi.warning(
            firstError
              ? `${t(mixedKey, { ok: changed, failed })} — ${firstError}`
              : t(mixedKey, { ok: changed, failed }),
          );
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
          messageApi.warning(
            firstError
              ? `${t('pages.clients.toasts.bulkDeletedMixed', { ok, failed })} — ${firstError}`
              : t('pages.clients.toasts.bulkDeletedMixed', { ok, failed }),
          );
        }
      },
    });
  }

  const onSave = useCallback(
    async (
      payload: Record<string, unknown> | { client: Record<string, unknown>; inboundIds: number[] },
      meta:
        | { isEdit: false; email: string; externalLinks: ExternalLinkInput[] }
        | {
            isEdit: true;
            email: string;
            attach: number[];
            detach: number[];
            externalLinks: ExternalLinkInput[];
          },
    ) => {
      if (!meta.isEdit) {
        const createMsg = await create(payload);
        if (!createMsg?.success) return createMsg;
        if (meta.email && meta.externalLinks.length > 0) {
          const r = await setExternalLinks(meta.email, meta.externalLinks);
          if (!r?.success) return r;
        }
        return createMsg;
      }
      const updateMsg = await update(meta.email, payload);
      if (!updateMsg?.success) return updateMsg;
      const rawEmail = (payload as { email?: unknown }).email;
      const emailKey =
        typeof rawEmail === 'string' && rawEmail.trim() ? rawEmail.trim() : meta.email;
      if (Array.isArray(meta.attach) && meta.attach.length > 0) {
        const r = await attach(emailKey, meta.attach);
        if (!r?.success) return r;
      }
      if (Array.isArray(meta.detach) && meta.detach.length > 0) {
        const r = await detach(emailKey, meta.detach);
        if (!r?.success) return r;
      }
      // Always replace the client's external links (an empty set clears them).
      const r = await setExternalLinks(emailKey, meta.externalLinks);
      if (!r?.success) return r;
      return updateMsg;
    },
    [create, update, attach, detach, setExternalLinks],
  );

  const pageClass = useMemo(() => {
    const classes = ['clients-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const onTableChange: NonNullable<TableProps<ClientRecord>['onChange']> = (pag) => {
    if (pag?.current) setCurrentPage(pag.current);
    if (pag?.pageSize) setPageSizeChoice(pag.pageSize);
  };

  const columns = useMemo<ColumnsType<ClientRecord>>(
    () => [
      {
        title: t('pages.clients.actions'),
        key: 'actions',
        width: 200,
        render: (_v, record) => (
          <ClientRowActions
            email={record.email}
            onShowQr={onShowQr}
            onShowInfo={onShowInfo}
            onResetTraffic={onResetTraffic}
            onEdit={onEdit}
            onDelete={onDelete}
          />
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
          const lastSubFetch = record.traffic?.lastSubFetch ?? 0;
          const lastOnlineTitle = `${t('lastOnline')}: ${lastOnline > 0 ? IntlUtil.formatDate(lastOnline, datepicker) : '-'}\n${t('lastSubFetch')}: ${lastSubFetch > 0 ? IntlUtil.formatDate(lastSubFetch, datepicker) : '-'}`;
          if (bucket === 'depleted')
            return (
              <Tooltip title={lastOnlineTitle}>
                <Tag color="red">{t('depleted')}</Tag>
              </Tooltip>
            );
          if (record.enable && isOnline(record.email))
            return (
              <Tag color="green" className="dot-tag">
                <span className="online-dot" />
                {t('pages.clients.online')}
              </Tag>
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
        width: 220,
        render: (_v, record) => (
          <div className="email-cell">
            <span className="email">{record.email}</span>
            {record.subId && (
              <span className="sub" title={record.subId}>
                {record.subId}
              </span>
            )}
            <ClientCardComment comment={record.comment} className="sub" />
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
          return (
            <ClientInboundChips
              ids={record.inboundIds || EMPTY_INBOUND_IDS}
              inboundsById={inboundsById}
              protocolColors={INBOUND_PROTOCOL_COLORS}
              chipLimit={INBOUND_CHIP_LIMIT}
            />
          );
        },
      },
      {
        title: t('pages.clients.traffic'),
        key: 'traffic',
        width: 300,
        render: (_v, record) => (
          <ClientTrafficCell
            up={record.traffic?.up}
            down={record.traffic?.down}
            total={record.totalGB}
            enabled={record.enable}
            trafficDiff={trafficDiff}
          />
        ),
      },
      {
        title: t('pages.clients.speed'),
        key: 'speed',
        width: SPEED_COLUMN_WIDTH,
        align: 'center',
        render: (_v, record) => {
          const speed = clientSpeed[record.email];
          if (!isActiveSpeed(speed)) {
            return (
              <Tag color="default" className={SPEED_TAG_CLASS_NAME} style={SPEED_TAG_STYLE}>
                —
              </Tag>
            );
          }
          return <ClientSpeedTag speed={speed} tableCell />;
        },
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
        width: 130,
        render: (_v, record) => (
          <Tooltip title={expiryLabel(record)}>
            <Tag color={expiryColor(record)}>
              {record.expiryTime ? expiryRelative(record) : '∞'}
            </Tag>
          </Tooltip>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      t,
      togglingEmail,
      clientBucket,
      isOnline,
      inboundsById,
      filters,
      allGroups,
      datepicker,
      trafficDiff,
      clientSpeed,
    ],
  );

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
      if (checked) next.add(email);
      else next.delete(email);
      return Array.from(next);
    });
  }

  function selectAll(checked: boolean) {
    setSelectedRowKeys(checked ? filteredClients.map((c) => c.email) : []);
  }

  const allSelected =
    filteredClients.length > 0 && selectedRowKeys.length === filteredClients.length;
  const someSelected =
    selectedRowKeys.length > 0 && selectedRowKeys.length < filteredClients.length;

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
                  extra={
                    <Button type="primary" loading={refreshing} onClick={onRefreshClick}>
                      {t('refresh')}
                    </Button>
                  }
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, 12]}>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('clients')}
                            value={String(summary.total)}
                            prefix={<TeamOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('online')}
                            open={summary.onlineCount ? undefined : false}
                            content={
                              <ClientEmailList
                                emails={summary.online}
                                total={summary.onlineCount}
                              />
                            }
                          >
                            <Statistic
                              title={t('online')}
                              value={String(summary.onlineCount)}
                              prefix={<span className="dot dot-blue" />}
                            />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('depleted')}
                            open={summary.depletedCount ? undefined : false}
                            content={
                              <ClientEmailList
                                emails={summary.depleted}
                                total={summary.depletedCount}
                              />
                            }
                          >
                            <Statistic
                              title={t('depleted')}
                              value={String(summary.depletedCount)}
                              prefix={<span className="dot dot-red" />}
                            />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('depletingSoon')}
                            open={summary.expiringCount ? undefined : false}
                            content={
                              <ClientEmailList
                                emails={summary.expiring}
                                total={summary.expiringCount}
                              />
                            }
                          >
                            <Statistic
                              title={t('depletingSoon')}
                              value={String(summary.expiringCount)}
                              prefix={<span className="dot dot-orange" />}
                            />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Popover
                            title={t('disabled')}
                            open={summary.deactiveCount ? undefined : false}
                            content={
                              <ClientEmailList
                                emails={summary.deactive}
                                total={summary.deactiveCount}
                              />
                            }
                          >
                            <Statistic
                              title={t('disabled')}
                              value={String(summary.deactiveCount)}
                              prefix={<span className="dot dot-gray" />}
                            />
                          </Popover>
                        </Col>
                        <Col xs={12} sm={8} md={4}>
                          <Statistic
                            title={t('subscription.active')}
                            value={String(summary.active)}
                            prefix={<span className="dot dot-green" />}
                          />
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
                            <Button
                              type="primary"
                              icon={<PlusOutlined />}
                              onClick={onAdd}
                              aria-label={t('pages.clients.addClients')}
                            >
                              {!isMobile && t('pages.clients.addClients')}
                            </Button>
                          ) : (
                            <Tag
                              color="blue"
                              closable
                              onClose={() => setSelectedRowKeys([])}
                              style={{ marginInlineEnd: 0, padding: '4px 8px', fontSize: 13 }}
                            >
                              {t('pages.clients.selectedCount', { count: selectedRowKeys.length })}
                            </Tag>
                          )}
                          <Dropdown
                            trigger={['click']}
                            placement="bottomRight"
                            menu={{
                              items:
                                selectedRowKeys.length > 0
                                  ? [
                                      {
                                        key: 'attach',
                                        icon: <UsergroupAddOutlined />,
                                        label: t('pages.clients.attach'),
                                        onClick: () => setBulkAttachOpen(true),
                                      },
                                      {
                                        key: 'detach',
                                        icon: <UsergroupDeleteOutlined />,
                                        label: t('pages.clients.detach'),
                                        danger: true,
                                        onClick: () => setBulkDetachOpen(true),
                                      },
                                      {
                                        key: 'addToGroup',
                                        icon: <TagsOutlined />,
                                        label: t('pages.clients.addToGroup'),
                                        onClick: () => setBulkGroupOpen(true),
                                      },
                                      {
                                        key: 'ungroup',
                                        icon: <UngroupIcon />,
                                        label: t('pages.clients.ungroup'),
                                        danger: true,
                                        onClick: onBulkUngroup,
                                      },
                                      { type: 'divider' as const },
                                      {
                                        key: 'enable',
                                        icon: <CheckCircleOutlined />,
                                        label: t('pages.clients.enable'),
                                        onClick: () => onBulkSetEnable(true),
                                      },
                                      {
                                        key: 'disable',
                                        icon: <StopOutlined />,
                                        label: t('pages.clients.disable'),
                                        danger: true,
                                        onClick: () => onBulkSetEnable(false),
                                      },
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
                                        key: 'export',
                                        icon: <DownloadOutlined />,
                                        label: t('pages.clients.exportClients'),
                                        onClick: onExportClients,
                                      },
                                      {
                                        key: 'import',
                                        icon: <UploadOutlined />,
                                        label: t('pages.clients.importClients'),
                                        onClick: onImportClients,
                                      },
                                      {
                                        key: 'resetAll',
                                        icon: <RetweetOutlined />,
                                        label: t('pages.clients.resetAllTraffics'),
                                        onClick: onResetAllTraffics,
                                      },
                                      { type: 'divider' as const },
                                      {
                                        key: 'delDepleted',
                                        icon: <RestOutlined />,
                                        label: t('pages.clients.delDepleted'),
                                        danger: true,
                                        onClick: onDelDepleted,
                                      },
                                      {
                                        key: 'delOrphans',
                                        icon: <DisconnectOutlined />,
                                        label: t('pages.clients.delOrphans'),
                                        danger: true,
                                        onClick: onDeleteOrphans,
                                      },
                                    ],
                            }}
                          >
                            <Button icon={<MoreOutlined />} aria-label={t('more')}>
                              {!isMobile && t('more')}
                            </Button>
                          </Dropdown>
                          {selectedRowKeys.length > 0 && (
                            <Button
                              danger
                              icon={<DeleteOutlined />}
                              onClick={onBulkDelete}
                              style={{ marginInlineStart: 'auto' }}
                              aria-label={t('delete')}
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
                          aria-label={t('search')}
                        />
                        <Badge count={activeCount} size="small" offset={[-4, 4]}>
                          <Button
                            icon={<FilterOutlined />}
                            size={isMobile ? 'small' : 'middle'}
                            onClick={() => setFilterDrawerOpen(true)}
                            type={activeCount > 0 ? 'primary' : 'default'}
                            aria-label={t('filter')}
                          >
                            {!isMobile && t('filter')}
                          </Button>
                        </Badge>
                        <Select
                          value={sortValueFor(sortColumn, sortOrder)}
                          aria-label={t('sort')}
                          size={isMobile ? 'small' : 'middle'}
                          suffix={<SortAscendingOutlined />}
                          style={{ minWidth: isMobile ? 130 : 200 }}
                          onChange={(value) => {
                            const opt = SORT_OPTIONS.find((o) => o.value === value);
                            setSortColumn(opt?.column ?? null);
                            setSortOrder(opt?.order ?? null);
                          }}
                          options={SORT_OPTIONS.map((o) => ({
                            value: o.value,
                            label: t(o.labelKey),
                          }))}
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
                              onClose={() =>
                                setFilters({
                                  ...filters,
                                  buckets: filters.buckets.filter((x) => x !== b),
                                })
                              }
                            >
                              {bucketChipLabel(b, t)}
                            </Tag>
                          ))}
                          {filters.protocols.map((p) => (
                            <Tag
                              key={`p-${p}`}
                              closable
                              color="blue"
                              onClose={() =>
                                setFilters({
                                  ...filters,
                                  protocols: filters.protocols.filter((x) => x !== p),
                                })
                              }
                            >
                              {p}
                            </Tag>
                          ))}
                          {filters.inboundIds.map((id) => (
                            <Tag
                              key={`i-${id}`}
                              closable
                              color="cyan"
                              onClose={() =>
                                setFilters({
                                  ...filters,
                                  inboundIds: filters.inboundIds.filter((x) => x !== id),
                                })
                              }
                            >
                              {inboundLabel(id)}
                            </Tag>
                          ))}
                          {filters.groups.map((g) => (
                            <Tag
                              key={`g-${g}`}
                              closable
                              color="geekblue"
                              onClose={() =>
                                setFilters({
                                  ...filters,
                                  groups: filters.groups.filter((x) => x !== g),
                                })
                              }
                            >
                              {t('pages.clients.group')}: {g}
                            </Tag>
                          ))}
                          {(filters.expiryFrom || filters.expiryTo) && (
                            <Tag
                              closable
                              color="purple"
                              onClose={() => clearOneFilter('expiryFrom')}
                            >
                              {t('pages.clients.expiryTime')}:{' '}
                              {filters.expiryFrom
                                ? IntlUtil.formatDate(filters.expiryFrom, datepicker)
                                : '…'}
                              {' → '}
                              {filters.expiryTo
                                ? IntlUtil.formatDate(filters.expiryTo, datepicker)
                                : '…'}
                            </Tag>
                          )}
                          {(filters.usageFromGB || filters.usageToGB) && (
                            <Tag
                              closable
                              color="orange"
                              onClose={() => clearOneFilter('usageFromGB')}
                            >
                              {t('pages.clients.traffic')}: {filters.usageFromGB ?? 0}
                              {filters.usageToGB ? `–${filters.usageToGB}` : '+'} GB
                            </Tag>
                          )}
                          {filters.autoRenew && (
                            <Tag closable color="gold" onClose={() => clearOneFilter('autoRenew')}>
                              {t('pages.clients.renew')}:{' '}
                              {filters.autoRenew === 'on' ? t('enabled') : t('disabled')}
                            </Tag>
                          )}
                          {filters.hasTgId && (
                            <Tag closable onClose={() => clearOneFilter('hasTgId')}>
                              {t('pages.clients.telegramId')}:{' '}
                              {filters.hasTgId === 'yes'
                                ? t('pages.clients.has')
                                : t('pages.clients.hasNot')}
                            </Tag>
                          )}
                          {filters.hasComment && (
                            <Tag closable onClose={() => clearOneFilter('hasComment')}>
                              {t('pages.clients.comment')}:{' '}
                              {filters.hasComment === 'yes'
                                ? t('pages.clients.has')
                                : t('pages.clients.hasNot')}
                            </Tag>
                          )}
                        </div>
                      )}

                      {!isMobile ? (
                        <Table<ClientRecord>
                          columns={columns}
                          dataSource={sortedClients}
                          loading={transitioning}
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
                        <Spin spinning={transitioning}>
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
                                    if (s && s !== tablePageSize) setPageSizeChoice(s);
                                  }}
                                />
                              </div>
                            )}
                            {filteredClients.map((row) => {
                              const bucket = clientBucket(row);
                              return (
                                <div
                                  key={row.email}
                                  className={`client-card${selectedRowKeys.includes(row.email) ? ' is-selected' : ''}`}
                                >
                                  <div className="card-head">
                                    <Checkbox
                                      checked={selectedRowKeys.includes(row.email)}
                                      onChange={(e) => toggleSelect(row.email, e.target.checked)}
                                    />
                                    {row.enable && bucket !== 'depleted' && isOnline(row.email) ? (
                                      <span className="online-dot" style={{ marginInlineEnd: 0 }} />
                                    ) : (
                                      <Badge status={bucketBadgeStatus(bucket)} />
                                    )}
                                    <span className="tag-name">{row.email}</span>
                                    {bucket === 'depleted' && (
                                      <Tag color="red" className="status-tag">
                                        {t('depleted')}
                                      </Tag>
                                    )}
                                    {bucket === 'expiring' && (
                                      <Tag color="orange" className="status-tag">
                                        {t('depletingSoon')}
                                      </Tag>
                                    )}
                                    <div className="card-actions">
                                      <Tooltip title={t('pages.clients.clientInfo')}>
                                        <InfoCircleOutlined
                                          className="row-action-trigger"
                                          role="button"
                                          tabIndex={0}
                                          aria-label={t('pages.clients.clientInfo')}
                                          onClick={() => onShowInfo(row.email)}
                                          onKeyDown={activateOnKey(() => onShowInfo(row.email))}
                                        />
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
                                              label: (
                                                <>
                                                  <QrcodeOutlined /> {t('pages.clients.qrCode')}
                                                </>
                                              ),
                                              onClick: () => onShowQr(row.email),
                                            },
                                            {
                                              key: 'reset',
                                              label: (
                                                <>
                                                  <RetweetOutlined />{' '}
                                                  {t('pages.inbounds.resetTraffic')}
                                                </>
                                              ),
                                              onClick: () => onResetTraffic(row.email),
                                            },
                                            {
                                              key: 'edit',
                                              label: (
                                                <>
                                                  <EditOutlined /> {t('edit')}
                                                </>
                                              ),
                                              onClick: () => onEdit(row.email),
                                            },
                                            {
                                              key: 'delete',
                                              danger: true,
                                              label: (
                                                <>
                                                  <DeleteOutlined /> {t('delete')}
                                                </>
                                              ),
                                              onClick: () => onDelete(row.email),
                                            },
                                          ],
                                        }}
                                      >
                                        <Button
                                          type="text"
                                          size="small"
                                          className="row-action-trigger"
                                          icon={<MoreOutlined />}
                                          aria-label={t('more')}
                                        />
                                      </Dropdown>
                                    </div>
                                  </div>
                                  <ClientCardComment comment={row.comment} />
                                  <ClientTrafficCell
                                    compact
                                    up={row.traffic?.up}
                                    down={row.traffic?.down}
                                    total={row.totalGB}
                                    enabled={row.enable}
                                    trafficDiff={trafficDiff}
                                  />
                                  {(() => {
                                    const speed = clientSpeed[row.email];
                                    if (!isActiveSpeed(speed)) return null;
                                    return (
                                      <div className="client-card-speed">
                                        <ClientSpeedTag speed={speed} />
                                      </div>
                                    );
                                  })()}
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
            attachedExternalLinks={editingExternalLinks}
            tunnelAllowedIPs={editingTunnelAllowedIPs}
            inbounds={inbounds}
            tgBotEnable={tgBotEnable}
            groups={allGroups}
            save={onSave}
            resetTraffic={resetTraffic}
            onOpenChange={setFormOpen}
          />
        </LazyMount>
        <LazyMount when={infoOpen}>
          <ClientInfoModal
            open={infoOpen}
            client={infoClient}
            inboundsById={inboundsById}
            tunnelAllowedIPs={viewingTunnelAllowedIPs}
            isOnline={infoClient ? isOnline(infoClient.email) : false}
            subSettings={subSettings}
            onOpenChange={setInfoOpen}
          />
        </LazyMount>
        <LazyMount when={qrOpen}>
          <ClientQrModal
            open={qrOpen}
            client={qrClient}
            inboundsById={inboundsById}
            tunnelAllowedIPs={viewingTunnelAllowedIPs}
            subSettings={subSettings}
            onOpenChange={setQrOpen}
          />
        </LazyMount>
        <LazyMount when={bulkAddOpen}>
          <ClientBulkAddModal
            open={bulkAddOpen}
            inbounds={inbounds}
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
            onSubmit={async (addDays, addBytes, flow, limitHwid, adTag) => {
              const msg = await bulkAdjust(
                [...selectedRowKeys],
                addDays,
                addBytes,
                flow,
                limitHwid,
                adTag,
              );
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
            nodes={nodes}
          />
        </LazyMount>
        <LazyMount when={textOpen}>
          <TextModal
            open={textOpen}
            onClose={() => setTextOpen(false)}
            title={textTitle}
            content={textContent}
            fileName={textFileName}
            json
          />
        </LazyMount>
        <LazyMount when={promptOpen}>
          <PromptModal
            open={promptOpen}
            onClose={() => setPromptOpen(false)}
            title={promptTitle}
            okText={promptOkText}
            initialValue={promptInitial}
            loading={promptLoading}
            json
            onConfirm={onPromptConfirm}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}

function bucketChipLabel(b: string, t: (k: string) => string): string {
  switch (b) {
    case 'active':
      return t('subscription.active');
    case 'expiring':
      return t('depletingSoon');
    case 'depleted':
      return t('depleted');
    case 'deactive':
      return t('disabled');
    case 'online':
      return t('online');
    default:
      return b;
  }
}
