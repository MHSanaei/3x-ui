import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  Modal,
  Result,
  Row,
  Spin,
  Statistic,
  message,
} from 'antd';

import { setMessageInstance } from '@/utils/messageBus';
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  PieChartOutlined,
  BarsOutlined,
} from '@ant-design/icons';

import { HttpUtil, SizeFormatter, RandomUtil } from '@/utils';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { genInboundLinks, genWireguardLinks, preferPublicHost } from '@/lib/xray/inbound-link';
import { inboundFromDb } from '@/lib/xray/inbound-from-db';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import AppSidebar from '@/layouts/AppSidebar';
const TextModal = lazy(() => import('@/components/feedback/TextModal'));
import type { TextModalTab } from '@/components/feedback/TextModal';
const PromptModal = lazy(() => import('@/components/feedback/PromptModal'));

import { useInbounds } from './useInbounds';
import { InboundList } from './list';
import { LazyMount } from '@/components/utility';
const InboundFormModal = lazy(() => import('./form/InboundFormModal'));
const InboundInfoModal = lazy(() => import('./info/InboundInfoModal'));
const QrCodeModal = lazy(() => import('./qr/QrCodeModal'));
const AttachClientsModal = lazy(() => import('./clients/AttachClientsModal'));
const AttachExistingClientsModal = lazy(() => import('./clients/AttachExistingClientsModal'));
const DetachClientsModal = lazy(() => import('./clients/DetachClientsModal'));
const AddClientsToGroupModal = lazy(() => import('./clients/AddClientsToGroupModal'));

type RowAction =
  | 'edit'
  | 'showInfo'
  | 'qrcode'
  | 'export'
  | 'subs'
  | 'clipboard'
  | 'delete'
  | 'resetTraffic'
  | 'delAllClients'
  | 'attachClients'
  | 'attachExisting'
  | 'detachClients'
  | 'addToGroup'
  | 'clone';

type GeneralAction = 'import' | 'export' | 'subs' | 'resetInbounds';

interface ClientMatchTarget {
  id?: string;
  email?: string;
  password?: string;
}

export default function InboundsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();

  const {
    fetched,
    fetchError,
    dbInbounds,
    clientCount,
    onlineClients,
    lastOnlineMap,
    inboundSpeed,
    totals,
    expireDiff,
    trafficDiff,
    pageSize,
    subSettings,
    tgBotEnable,
    ipLimitEnable,
    refresh,
    hydrateInbound,
    applyTrafficEvent,
    applyClientStatsEvent,
  } = useInbounds();

  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { nodes: nodesList, fetched: nodesFetched } = useNodesQuery();
  const nodesById = useMemo(() => {
    const map = new Map<number, ReturnType<typeof useNodesQuery>['nodes'][number]>();
    for (const n of nodesList || []) map.set(n.id, n);
    return map;
  }, [nodesList]);

  const hasActiveNode = useMemo(
    () => (nodesList || []).some((n) => n.enable && n.status === 'online'),
    [nodesList],
  );
  const hasNodeAttachedInbound = useMemo(
    () => (dbInbounds || []).some((ib) => ib?.nodeId != null),
    [dbInbounds],
  );
  const showNodeInfo = hasNodeAttachedInbound || hasActiveNode;

  useWebSocket({
    traffic: applyTrafficEvent,
    client_stats: applyClientStatsEvent,
  });

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formDbInbound, setFormDbInbound] = useState<DBInbound | null>(null);

  const [infoOpen, setInfoOpen] = useState(false);
  const [infoDbInbound, setInfoDbInbound] = useState<DBInbound | null>(null);
  const [infoClientIndex, setInfoClientIndex] = useState(0);

  const [qrOpen, setQrOpen] = useState(false);
  const [qrDbInbound, setQrDbInbound] = useState<DBInbound | null>(null);

  const [attachOpen, setAttachOpen] = useState(false);
  const [attachSource, setAttachSource] = useState<DBInbound | null>(null);
  const [attachExistingOpen, setAttachExistingOpen] = useState(false);
  const [attachExistingTarget, setAttachExistingTarget] = useState<DBInbound | null>(null);
  const [detachOpen, setDetachOpen] = useState(false);
  const [detachSource, setDetachSource] = useState<DBInbound | null>(null);

  const [groupOpen, setGroupOpen] = useState(false);
  const [groupSource, setGroupSource] = useState<DBInbound | null>(null);

  const [textOpen, setTextOpen] = useState(false);
  const [textTitle, setTextTitle] = useState('');
  const [textContent, setTextContent] = useState('');
  const [textFileName, setTextFileName] = useState('');
  const [textJson, setTextJson] = useState(false);
  const [textTabs, setTextTabs] = useState<TextModalTab[] | undefined>(undefined);

  const [promptOpen, setPromptOpen] = useState(false);
  const [promptTitle, setPromptTitle] = useState('');
  const [promptOkText, setPromptOkText] = useState('OK');
  const [promptType, setPromptType] = useState<'textarea' | 'input'>('textarea');
  const [promptInitial, setPromptInitial] = useState('');
  const [promptJson, setPromptJson] = useState(false);
  const [promptLoading, setPromptLoading] = useState(false);
  const [promptHandler, setPromptHandler] = useState<((value: string) => Promise<boolean | void> | boolean | void) | null>(null);

  const hostOverrideFor = useCallback((dbInbound: DBInbound | null) => {
    if (!dbInbound || dbInbound.nodeId == null) return '';
    return nodesById.get(dbInbound.nodeId)?.address || '';
  }, [nodesById]);

  const infoNodeAddress = useMemo(() => hostOverrideFor(infoDbInbound), [infoDbInbound, hostOverrideFor]);
  const qrNodeAddress = useMemo(() => hostOverrideFor(qrDbInbound), [qrDbInbound, hostOverrideFor]);

  const openText = useCallback((opts: { title: string; content: string; fileName?: string; json?: boolean; tabs?: TextModalTab[] }) => {
    setTextTitle(opts.title);
    setTextContent(opts.content);
    setTextFileName(opts.fileName || '');
    setTextJson(opts.json || false);
    setTextTabs(opts.tabs);
    setTextOpen(true);
  }, []);

  const openPrompt = useCallback((opts: {
    title: string;
    okText?: string;
    type?: 'textarea' | 'input';
    value?: string;
    json?: boolean;
    confirm: (value: string) => Promise<boolean | void> | boolean | void;
  }) => {
    setPromptTitle(opts.title);
    setPromptOkText(opts.okText || t('confirm'));
    setPromptType(opts.type || 'textarea');
    setPromptInitial(opts.value || '');
    setPromptJson(opts.json || false);
    setPromptHandler(() => opts.confirm);
    setPromptOpen(true);
  }, [t]);

  const onPromptConfirm = useCallback(async (value: string) => {
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
  }, [promptHandler]);

  const projectChildThroughMaster = useCallback((child: DBInbound, master: DBInbound): DBInbound => {
    const projected = JSON.parse(JSON.stringify(child)) as DBInbound;
    projected.listen = master.listen;
    projected.port = master.port;
    const masterStream = coerceInboundJsonField(master.streamSettings) as Record<string, unknown>;
    const childStream = { ...(coerceInboundJsonField(child.streamSettings) as Record<string, unknown>) };
    childStream.security = masterStream.security;
    childStream.tlsSettings = masterStream.tlsSettings;
    childStream.realitySettings = masterStream.realitySettings;
    childStream.externalProxy = masterStream.externalProxy;
    projected.streamSettings = JSON.stringify(childStream);
    const Ctor = child.constructor as new (data: DBInbound) => DBInbound;
    return new Ctor(projected);
  }, []);

  const checkFallback = useCallback((dbInbound: DBInbound): DBInbound => {
    const parent = dbInbound?.fallbackParent;
    if (parent?.masterId) {
      const master = dbInbounds.find((ib) => ib.id === parent.masterId);
      if (master) return projectChildThroughMaster(dbInbound, master);
    }
    if (!dbInbound?.listen?.startsWith?.('@')) return dbInbound;
    for (const candidate of dbInbounds) {
      if (candidate.id === dbInbound.id) continue;
      if (!['trojan', 'vless'].includes(candidate.protocol)) continue;
      const candStream = coerceInboundJsonField(candidate.streamSettings) as { network?: string };
      if (candStream.network !== 'tcp') continue;
      const candSettings = coerceInboundJsonField(candidate.settings) as { fallbacks?: { dest?: string }[] };
      const fallbacks = candSettings.fallbacks || [];
      if (!fallbacks.find((f) => f.dest === dbInbound.listen)) continue;
      return projectChildThroughMaster(dbInbound, candidate);
    }
    return dbInbound;
  }, [dbInbounds, projectChildThroughMaster]);

  const findClientIndex = useCallback((dbInbound: DBInbound, client: ClientMatchTarget | null) => {
    if (!client) return 0;
    const settings = coerceInboundJsonField(dbInbound.settings) as { clients?: ClientMatchTarget[] };
    const clients = settings.clients || [];
    const idx = clients.findIndex((c) => {
      if (!c) return false;
      switch (dbInbound.protocol) {
        case 'trojan':
        case 'shadowsocks':
          return c.password === client.password && c.email === client.email;
        default:
          return c.id === client.id && c.email === client.email;
      }
    });
    return idx >= 0 ? idx : 0;
  }, []);

  const exportInboundLinks = useCallback((dbInbound: DBInbound) => {
    const projected = checkFallback(dbInbound);
    const genInput = {
      inbound: inboundFromDb(projected),
      remark: projected.remark,
      hostOverride: hostOverrideFor(dbInbound),
      fallbackHostname: preferPublicHost(window.location.hostname, subSettings.publicHost),
    };
    const content = genInboundLinks(genInput);
    const tabs: TextModalTab[] | undefined = projected.isWireguard
      ? [
        { key: 'config', label: t('pages.clients.config'), content },
        { key: 'links', label: t('pages.clients.tabLinks'), content: genWireguardLinks(genInput) },
      ]
      : undefined;
    openText({
      title: t('pages.inbounds.exportLinksTitle'),
      content,
      fileName: projected.remark || 'inbound',
      tabs,
    });
  }, [checkFallback, hostOverrideFor, subSettings.publicHost, openText, t]);

  const exportInboundClipboard = useCallback((dbInbound: DBInbound) => {
    openText({ title: t('pages.inbounds.inboundJsonTitle'), content: JSON.stringify(dbInbound, null, 2), json: true });
  }, [openText, t]);

  const exportInboundSubs = useCallback((dbInbound: DBInbound) => {
    const settings = coerceInboundJsonField(dbInbound.settings) as { clients?: { subId?: string }[] };
    const clients = settings.clients || [];
    const subLinks: string[] = [];
    for (const c of clients) {
      if (c.subId && subSettings.subURI) {
        subLinks.push(subSettings.subURI + c.subId);
      }
    }
    openText({
      title: t('pages.inbounds.exportSubsTitle'),
      content: [...new Set(subLinks)].join('\n'),
      fileName: `${dbInbound.remark || 'inbound'}-Subs`,
    });
  }, [subSettings, openText, t]);

  const exportAllLinks = useCallback(async () => {
    const msg = await HttpUtil.get('/panel/api/inbounds/allLinks');
    const links = msg?.success && Array.isArray(msg.obj) ? (msg.obj as string[]) : [];
    openText({ title: t('pages.inbounds.exportAllLinksTitle'), content: links.join('\r\n'), fileName: t('pages.inbounds.exportAllLinksFileName') });
  }, [openText, t]);

  const exportAllSubs = useCallback(async () => {
    const hydrated = await Promise.all(
      dbInbounds.map((ib) => hydrateInbound(ib.id).then((r) => r ?? ib)),
    );
    const out: string[] = [];
    for (const ib of hydrated) {
      const settings = coerceInboundJsonField(ib.settings) as { clients?: { subId?: string }[] };
      const clients = settings.clients || [];
      for (const c of clients) {
        if (c.subId && subSettings.subURI) {
          out.push(subSettings.subURI + c.subId);
        }
      }
    }
    openText({ title: t('pages.inbounds.exportAllSubsTitle'), content: [...new Set(out)].join('\r\n'), fileName: t('pages.inbounds.exportAllSubsFileName') });
  }, [dbInbounds, hydrateInbound, subSettings, openText, t]);

  const importInbound = useCallback(() => {
    openPrompt({
      title: t('pages.inbounds.importInbound'),
      okText: t('pages.inbounds.import'),
      type: 'textarea',
      value: '',
      json: true,
      confirm: async (value) => {
        const msg = await HttpUtil.post('/panel/api/inbounds/import', { data: value });
        if (msg?.success) {
          await refresh();
          return true;
        }
        return false;
      },
    });
  }, [openPrompt, refresh, t]);

  const onAddInbound = useCallback(() => {
    setFormMode('add');
    setFormDbInbound(null);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((dbInbound: DBInbound) => {
    setFormMode('edit');
    setFormDbInbound(dbInbound);
    setFormOpen(true);
  }, []);

  const confirmDelete = useCallback((dbInbound: DBInbound) => {
    modal.confirm({
      title: t('pages.inbounds.deleteConfirmTitle', { remark: dbInbound.remark }),
      content: t('pages.inbounds.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/api/inbounds/del/${dbInbound.id}`);
        if (msg?.success) await refresh();
      },
    });
  }, [modal, refresh, t]);

  const confirmBulkDelete = useCallback((ids: number[]) => new Promise<boolean>((resolve) => {
    if (ids.length === 0) {
      resolve(false);
      return;
    }
    modal.confirm({
      title: t('pages.inbounds.bulkDeleteConfirmTitle', { count: ids.length }),
      content: t('pages.inbounds.bulkDeleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post('/panel/api/inbounds/bulkDel', { ids }, { headers: { 'Content-Type': 'application/json' } });
        const obj = (msg?.obj ?? {}) as { deleted?: number; skipped?: { id: number; reason: string }[] };
        const ok = obj.deleted ?? 0;
        const skipped = obj.skipped ?? [];
        if (msg?.success && skipped.length === 0) {
          messageApi.success(t('pages.inbounds.toasts.bulkDeleted', { count: ok }));
        } else {
          const firstError = skipped[0]?.reason ?? msg?.msg ?? '';
          const base = t('pages.inbounds.toasts.bulkDeletedMixed', { ok, failed: skipped.length });
          messageApi.warning(firstError ? `${base} — ${firstError}` : base);
        }
        await refresh();
        resolve(true);
      },
      onCancel: () => resolve(false),
    });
  }), [modal, refresh, t, messageApi]);

  const confirmResetTraffic = useCallback((dbInbound: DBInbound) => {
    modal.confirm({
      title: t('pages.inbounds.resetConfirmTitle', { remark: dbInbound.remark }),
      content: t('pages.inbounds.resetConfirmContent'),
      okText: t('reset'),
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/api/inbounds/${dbInbound.id}/resetTraffic`);
        if (msg?.success) await refresh();
      },
    });
  }, [modal, refresh, t]);

  const confirmDelAllClients = useCallback((dbInbound: DBInbound) => {
    const count = clientCount[dbInbound.id]?.clients || 0;
    modal.confirm({
      title: t('pages.inbounds.delAllClientsConfirmTitle', { remark: dbInbound.remark, count }),
      content: t('pages.inbounds.delAllClientsConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/api/inbounds/${dbInbound.id}/delAllClients`);
        if (msg?.success) await refresh();
      },
    });
  }, [modal, refresh, t, clientCount]);

  const confirmClone = useCallback((dbInbound: DBInbound) => {
    modal.confirm({
      title: t('pages.inbounds.cloneConfirmTitle', { remark: dbInbound.remark }),
      content: t('pages.inbounds.cloneConfirmContent'),
      okText: t('pages.inbounds.clone'),
      cancelText: t('cancel'),
      onOk: async () => {
        let clonedSettings: string;
        try {
          const raw = coerceInboundJsonField(dbInbound.settings);
          raw.clients = [];
          clonedSettings = JSON.stringify(raw);
        } catch {
          const fallback = createDefaultInboundSettings(dbInbound.protocol);
          clonedSettings = fallback ? JSON.stringify(fallback, null, 2) : '{}';
        }
        const streamSettingsString = typeof dbInbound.streamSettings === 'string'
          ? dbInbound.streamSettings
          : JSON.stringify(dbInbound.streamSettings ?? {});
        const sniffingString = typeof dbInbound.sniffing === 'string'
          ? dbInbound.sniffing
          : JSON.stringify(dbInbound.sniffing ?? {});
        const data = {
          up: 0,
          down: 0,
          total: 0,
          remark: `${dbInbound.remark} (clone)`,
          enable: false,
          expiryTime: 0,
          listen: '',
          port: RandomUtil.randomInteger(10000, 60000),
          protocol: dbInbound.protocol,
          settings: clonedSettings,
          streamSettings: streamSettingsString,
          sniffing: sniffingString,
          shareAddrStrategy: dbInbound.shareAddrStrategy,
          shareAddr: dbInbound.shareAddr,
        };
        const msg = await HttpUtil.post('/panel/api/inbounds/add', data);
        if (msg?.success) await refresh();
      },
    });
  }, [modal, refresh, t]);

  const onGeneralAction = useCallback((key: GeneralAction) => {
    switch (key) {
      case 'import': importInbound(); break;
      case 'export': exportAllLinks(); break;
      case 'subs': exportAllSubs(); break;
      case 'resetInbounds':
        modal.confirm({
          title: t('pages.inbounds.resetAllTrafficTitle'),
          okText: t('reset'),
          cancelText: t('cancel'),
          onOk: async () => {
            const msg = await HttpUtil.post('/panel/api/inbounds/resetAllTraffics');
            if (msg?.success) await refresh();
          },
        });
        break;
      default:
        messageApi.info(`General action "${key}" — coming in a later 5f subphase`);
    }
  }, [modal, importInbound, exportAllLinks, exportAllSubs, refresh, messageApi, t]);

  const onRowAction = useCallback(async ({ key, dbInbound }: { key: RowAction; dbInbound: DBInbound }) => {
    // Actions that touch per-client secrets (uuid, password, flow, ...) need
    // the full payload that the slim list view does not ship. Hydrate first
    // and then operate on the rehydrated record.
    const hydratingKeys: RowAction[] = ['edit', 'showInfo', 'qrcode', 'export', 'subs', 'clipboard', 'clone', 'attachClients', 'addToGroup'];
    let target = dbInbound;
    if (hydratingKeys.includes(key)) {
      const hydrated = await hydrateInbound(dbInbound.id);
      if (hydrated) target = hydrated;
    }
    switch (key) {
      case 'edit':
        openEdit(target);
        break;
      case 'showInfo':
        setInfoDbInbound(checkFallback(target));
        setInfoClientIndex(findClientIndex(target, null));
        setInfoOpen(true);
        break;
      case 'qrcode':
        setQrDbInbound(checkFallback(target));
        setQrOpen(true);
        break;
      case 'export':
        exportInboundLinks(target);
        break;
      case 'subs':
        exportInboundSubs(target);
        break;
      case 'clipboard':
        exportInboundClipboard(target);
        break;
      case 'delete':
        confirmDelete(target);
        break;
      case 'resetTraffic':
        confirmResetTraffic(target);
        break;
      case 'delAllClients':
        confirmDelAllClients(target);
        break;
      case 'attachClients':
        setAttachSource(target);
        setAttachOpen(true);
        break;
      case 'attachExisting':
        setAttachExistingTarget(target);
        setAttachExistingOpen(true);
        break;
      case 'detachClients':
        setDetachSource(target);
        setDetachOpen(true);
        break;
      case 'addToGroup':
        setGroupSource(target);
        setGroupOpen(true);
        break;
      case 'clone':
        confirmClone(target);
        break;
      default:
        messageApi.info(`Action "${key}" — coming in a later 5f subphase`);
    }
  }, [hydrateInbound, openEdit, checkFallback, findClientIndex, exportInboundLinks, exportInboundSubs, exportInboundClipboard, confirmDelete, confirmResetTraffic, confirmDelAllClients, confirmClone, messageApi]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={`inbounds-page${isDark ? ' is-dark' : ''}${isUltra ? ' is-ultra' : ''}`}>
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
                  extra={<Button type="primary" onClick={refresh}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.inbounds.totalDownUp')}
                            value={0}
                            formatter={() => (
                              <span>
                                <ArrowUpOutlined /> {SizeFormatter.sizeFormat(totals.up)}
                                {' / '}
                                <ArrowDownOutlined /> {SizeFormatter.sizeFormat(totals.down)}
                              </span>
                            )}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.inbounds.totalUsage')}
                            value={SizeFormatter.sizeFormat(totals.up + totals.down)}
                            prefix={<PieChartOutlined />}
                          />
                        </Col>
                        <Col xs={24} sm={24} md={8}>
                          <Statistic
                            title={t('pages.inbounds.inboundCount')}
                            value={String(dbInbounds.length)}
                            prefix={<BarsOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <InboundList
                      dbInbounds={dbInbounds}
                      clientCount={clientCount}
                      onlineClients={onlineClients}
                      lastOnlineMap={lastOnlineMap}
                      inboundSpeed={inboundSpeed}
                      expireDiff={expireDiff}
                      trafficDiff={trafficDiff}
                      pageSize={pageSize}
                      isMobile={isMobile}
                      subEnable={subSettings.enable}
                      nodesById={nodesById}
                      hasActiveNode={showNodeInfo}
                      onAddInbound={onAddInbound}
                      onGeneralAction={onGeneralAction}
                      onRowAction={({ key, dbInbound }) => onRowAction({ key, dbInbound: dbInbound as unknown as DBInbound })}
                      onBulkDelete={confirmBulkDelete}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <LazyMount when={formOpen}>
          <InboundFormModal
            open={formOpen}
            onClose={() => setFormOpen(false)}
            onSaved={refresh}
            mode={formMode}
            dbInbound={formDbInbound}
            dbInbounds={dbInbounds}
            availableNodes={nodesList}
            availableNodesFetched={nodesFetched}
          />
        </LazyMount>
        <LazyMount when={infoOpen}>
          <InboundInfoModal
            open={infoOpen}
            onClose={() => setInfoOpen(false)}
            dbInbound={infoDbInbound}
            clientIndex={infoClientIndex}
            expireDiff={expireDiff}
            trafficDiff={trafficDiff}
            ipLimitEnable={ipLimitEnable}
            tgBotEnable={tgBotEnable}
            subSettings={subSettings}
            lastOnlineMap={lastOnlineMap}
            nodeAddress={infoNodeAddress}
          />
        </LazyMount>
        <LazyMount when={qrOpen}>
          <QrCodeModal
            open={qrOpen}
            onClose={() => setQrOpen(false)}
            dbInbound={qrDbInbound}
            client={null}
            nodeAddress={qrNodeAddress}
            subSettings={subSettings}
          />
        </LazyMount>
        <LazyMount when={attachOpen}>
          <AttachClientsModal
            open={attachOpen}
            onClose={() => setAttachOpen(false)}
            onAttached={refresh}
            source={attachSource}
            dbInbounds={dbInbounds}
          />
        </LazyMount>
        <LazyMount when={attachExistingOpen}>
          <AttachExistingClientsModal
            open={attachExistingOpen}
            onClose={() => setAttachExistingOpen(false)}
            onAttached={refresh}
            target={attachExistingTarget}
          />
        </LazyMount>
        <LazyMount when={detachOpen}>
          <DetachClientsModal
            open={detachOpen}
            onClose={() => setDetachOpen(false)}
            onDetached={refresh}
            source={detachSource}
          />
        </LazyMount>
        <LazyMount when={groupOpen}>
          <AddClientsToGroupModal
            open={groupOpen}
            onClose={() => setGroupOpen(false)}
            onAdded={refresh}
            source={groupSource}
          />
        </LazyMount>

        <LazyMount when={textOpen}>
          <TextModal
            open={textOpen}
            onClose={() => setTextOpen(false)}
            title={textTitle}
            content={textContent}
            fileName={textFileName}
            json={textJson}
            tabs={textTabs}
          />
        </LazyMount>
        <LazyMount when={promptOpen}>
          <PromptModal
            open={promptOpen}
            onClose={() => setPromptOpen(false)}
            title={promptTitle}
            okText={promptOkText}
            type={promptType}
            initialValue={promptInitial}
            loading={promptLoading}
            json={promptJson}
            onConfirm={onPromptConfirm}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}
