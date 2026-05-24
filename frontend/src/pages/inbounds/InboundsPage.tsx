/* eslint-disable @typescript-eslint/no-explicit-any */
import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Card,
  Col,
  ConfigProvider,
  Layout,
  Modal,
  Row,
  Spin,
  message,
} from 'antd';

import { setMessageInstance } from '@/utils/messageBus';
import {
  SwapOutlined,
  PieChartOutlined,
  BarsOutlined,
} from '@ant-design/icons';

import { HttpUtil, SizeFormatter, RandomUtil } from '@/utils';
import { Inbound } from '@/models/inbound.js';
import { coerceInboundJsonField } from '@/models/dbinbound.js';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import AppSidebar from '@/components/AppSidebar';
import CustomStatistic from '@/components/CustomStatistic';
const TextModal = lazy(() => import('@/components/TextModal'));
const PromptModal = lazy(() => import('@/components/PromptModal'));

import { useInbounds } from './useInbounds';
import InboundList from './InboundList';
import LazyMount from '@/components/LazyMount';
const InboundFormModal = lazy(() => import('./InboundFormModal'));
const InboundInfoModal = lazy(() => import('./InboundInfoModal'));
const QrCodeModal = lazy(() => import('./QrCodeModal'));
import '@/styles/page-cards.css';
import './InboundsPage.css';

type RowAction =
  | 'edit'
  | 'showInfo'
  | 'qrcode'
  | 'export'
  | 'subs'
  | 'clipboard'
  | 'delete'
  | 'resetTraffic'
  | 'clone';

type GeneralAction = 'import' | 'export' | 'subs' | 'resetInbounds';

export default function InboundsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();

  const {
    fetched,
    dbInbounds,
    clientCount,
    onlineClients,
    lastOnlineMap,
    totals,
    expireDiff,
    trafficDiff,
    pageSize,
    subSettings,
    tgBotEnable,
    ipLimitEnable,
    remarkModel,
    refresh,
    hydrateInbound,
    applyTrafficEvent,
    applyClientStatsEvent,
  } = useInbounds();

  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { nodes: nodesList } = useNodesQuery();
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
    () => (dbInbounds || []).some((ib: any) => ib?.nodeId != null),
    [dbInbounds],
  );
  const showNodeInfo = hasNodeAttachedInbound || hasActiveNode;

  useWebSocket({
    traffic: applyTrafficEvent,
    client_stats: applyClientStatsEvent,
  });

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formDbInbound, setFormDbInbound] = useState<any>(null);

  const [infoOpen, setInfoOpen] = useState(false);
  const [infoDbInbound, setInfoDbInbound] = useState<any>(null);
  const [infoClientIndex, setInfoClientIndex] = useState(0);

  const [qrOpen, setQrOpen] = useState(false);
  const [qrDbInbound, setQrDbInbound] = useState<any>(null);

  const [textOpen, setTextOpen] = useState(false);
  const [textTitle, setTextTitle] = useState('');
  const [textContent, setTextContent] = useState('');
  const [textFileName, setTextFileName] = useState('');

  const [promptOpen, setPromptOpen] = useState(false);
  const [promptTitle, setPromptTitle] = useState('');
  const [promptOkText, setPromptOkText] = useState('OK');
  const [promptType, setPromptType] = useState<'textarea' | 'input'>('textarea');
  const [promptInitial, setPromptInitial] = useState('');
  const [promptLoading, setPromptLoading] = useState(false);
  const [promptHandler, setPromptHandler] = useState<((value: string) => Promise<boolean | void> | boolean | void) | null>(null);

  const hostOverrideFor = useCallback((dbInbound: any) => {
    if (!dbInbound || dbInbound.nodeId == null) return '';
    return nodesById.get(dbInbound.nodeId)?.address || '';
  }, [nodesById]);

  const infoNodeAddress = useMemo(() => hostOverrideFor(infoDbInbound), [infoDbInbound, hostOverrideFor]);
  const qrNodeAddress = useMemo(() => hostOverrideFor(qrDbInbound), [qrDbInbound, hostOverrideFor]);

  const openText = useCallback((opts: { title: string; content: string; fileName?: string }) => {
    setTextTitle(opts.title);
    setTextContent(opts.content);
    setTextFileName(opts.fileName || '');
    setTextOpen(true);
  }, []);

  const openPrompt = useCallback((opts: {
    title: string;
    okText?: string;
    type?: 'textarea' | 'input';
    value?: string;
    confirm: (value: string) => Promise<boolean | void> | boolean | void;
  }) => {
    setPromptTitle(opts.title);
    setPromptOkText(opts.okText || 'OK');
    setPromptType(opts.type || 'textarea');
    setPromptInitial(opts.value || '');
    setPromptHandler(() => opts.confirm);
    setPromptOpen(true);
  }, []);

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

  const projectChildThroughMaster = useCallback((child: any, master: any) => {
    const projected = JSON.parse(JSON.stringify(child));
    projected.listen = master.listen;
    projected.port = master.port;
    const masterStream = master.toInbound().stream;
    const childInbound = child.toInbound();
    childInbound.stream.security = masterStream.security;
    childInbound.stream.tls = masterStream.tls;
    childInbound.stream.reality = masterStream.reality;
    childInbound.stream.externalProxy = masterStream.externalProxy;
    projected.streamSettings = childInbound.stream.toString();
    return new child.constructor(projected);
  }, []);

  const checkFallback = useCallback((dbInbound: any) => {
    const parent = dbInbound?.fallbackParent;
    if (parent?.masterId) {
      const master = (dbInbounds as any[]).find((ib: any) => ib.id === parent.masterId);
      if (master) return projectChildThroughMaster(dbInbound, master);
    }
    if (!(dbInbound?.listen as string | undefined)?.startsWith?.('@')) return dbInbound;
    for (const candidate of dbInbounds as any[]) {
      if (candidate.id === dbInbound.id) continue;
      const parsed = candidate.toInbound();
      if (!parsed.isTcp) continue;
      if (!['trojan', 'vless'].includes(parsed.protocol)) continue;
      const fallbacks = parsed.settings.fallbacks || [];
      if (!fallbacks.find((f: { dest?: string }) => f.dest === dbInbound.listen)) continue;
      return projectChildThroughMaster(dbInbound, candidate);
    }
    return dbInbound;
  }, [dbInbounds, projectChildThroughMaster]);

  const findClientIndex = useCallback((dbInbound: any, client: any) => {
    if (!client) return 0;
    const inbound = dbInbound.toInbound();
    const clients = inbound?.clients || [];
    const idx = clients.findIndex((c: any) => {
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

  const exportInboundLinks = useCallback((dbInbound: any) => {
    const projected = checkFallback(dbInbound);
    openText({
      title: t('pages.inbounds.exportLinksTitle'),
      content: projected.genInboundLinks(remarkModel, hostOverrideFor(dbInbound)),
      fileName: projected.remark || 'inbound',
    });
  }, [checkFallback, remarkModel, hostOverrideFor, openText, t]);

  const exportInboundClipboard = useCallback((dbInbound: any) => {
    openText({ title: t('pages.inbounds.inboundJsonTitle'), content: JSON.stringify(dbInbound, null, 2) });
  }, [openText, t]);

  const exportInboundSubs = useCallback((dbInbound: any) => {
    const inbound = dbInbound.toInbound();
    const clients = inbound?.clients || [];
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
    const hydrated = await Promise.all(
      (dbInbounds as any[]).map((ib) => hydrateInbound(ib.id).then((r) => r ?? ib)),
    );
    const out: string[] = [];
    for (const ib of hydrated) {
      const projected = checkFallback(ib);
      out.push(projected.genInboundLinks(remarkModel, hostOverrideFor(ib)));
    }
    openText({ title: t('pages.inbounds.exportAllLinksTitle'), content: out.join('\r\n'), fileName: 'All-Inbounds' });
  }, [dbInbounds, hydrateInbound, checkFallback, remarkModel, hostOverrideFor, openText, t]);

  const exportAllSubs = useCallback(async () => {
    const hydrated = await Promise.all(
      (dbInbounds as any[]).map((ib) => hydrateInbound(ib.id).then((r) => r ?? ib)),
    );
    const out: string[] = [];
    for (const ib of hydrated) {
      const inbound = ib.toInbound();
      const clients = inbound?.clients || [];
      for (const c of clients) {
        if (c.subId && subSettings.subURI) {
          out.push(subSettings.subURI + c.subId);
        }
      }
    }
    openText({ title: t('pages.inbounds.exportAllSubsTitle'), content: [...new Set(out)].join('\r\n'), fileName: 'All-Inbounds-Subs' });
  }, [dbInbounds, hydrateInbound, subSettings, openText, t]);

  const importInbound = useCallback(() => {
    openPrompt({
      title: 'Import inbound',
      okText: 'Import',
      type: 'textarea',
      value: '',
      confirm: async (value) => {
        const msg = await HttpUtil.post('/panel/api/inbounds/import', { data: value });
        if (msg?.success) {
          await refresh();
          return true;
        }
        return false;
      },
    });
  }, [openPrompt, refresh]);

  const onAddInbound = useCallback(() => {
    setFormMode('add');
    setFormDbInbound(null);
    setFormOpen(true);
  }, []);

  const openEdit = useCallback((dbInbound: any) => {
    setFormMode('edit');
    setFormDbInbound(dbInbound);
    setFormOpen(true);
  }, []);

  const confirmDelete = useCallback((dbInbound: any) => {
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

  const confirmResetTraffic = useCallback((dbInbound: any) => {
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

  const confirmClone = useCallback((dbInbound: any) => {
    modal.confirm({
      title: t('pages.inbounds.cloneConfirmTitle', { remark: dbInbound.remark }),
      content: t('pages.inbounds.cloneConfirmContent'),
      okText: t('pages.inbounds.clone'),
      cancelText: t('cancel'),
      onOk: async () => {
        const baseInbound = dbInbound.toInbound();
        let clonedSettings: string;
        try {
          const raw = coerceInboundJsonField(dbInbound.settings);
          raw.clients = [];
          clonedSettings = JSON.stringify(raw);
        } catch {
          clonedSettings = (Inbound as any).Settings.getSettings(baseInbound.protocol).toString();
        }
        const data = {
          up: 0,
          down: 0,
          total: 0,
          remark: `${dbInbound.remark} (clone)`,
          enable: false,
          expiryTime: 0,
          listen: '',
          port: RandomUtil.randomInteger(10000, 60000),
          protocol: baseInbound.protocol,
          settings: clonedSettings,
          streamSettings: baseInbound.stream.toString(),
          sniffing: baseInbound.sniffing.toString(),
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
          title: 'Reset all inbound traffic?',
          okText: 'Reset',
          cancelText: 'Cancel',
          onOk: async () => {
            const msg = await HttpUtil.post('/panel/api/inbounds/resetAllTraffics');
            if (msg?.success) await refresh();
          },
        });
        break;
      default:
        messageApi.info(`General action "${key}" — coming in a later 5f subphase`);
    }
  }, [modal, importInbound, exportAllLinks, exportAllSubs, refresh, messageApi]);

  const onRowAction = useCallback(async ({ key, dbInbound }: { key: RowAction; dbInbound: any }) => {
    // Actions that touch per-client secrets (uuid, password, flow, ...) need
    // the full payload that the slim list view does not ship. Hydrate first
    // and then operate on the rehydrated record.
    const hydratingKeys: RowAction[] = ['edit', 'showInfo', 'qrcode', 'export', 'subs', 'clipboard', 'clone'];
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
      case 'clone':
        confirmClone(target);
        break;
      default:
        messageApi.info(`Action "${key}" — coming in a later 5f subphase`);
    }
  }, [hydrateInbound, openEdit, checkFallback, findClientIndex, exportInboundLinks, exportInboundSubs, exportInboundClipboard, confirmDelete, confirmResetTraffic, confirmClone, messageApi]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={`inbounds-page${isDark ? ' is-dark' : ''}${isUltra ? ' is-ultra' : ''}`}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description="Loading…" size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <CustomStatistic
                            title={t('pages.inbounds.totalDownUp')}
                            value={`${SizeFormatter.sizeFormat(totals.up)} / ${SizeFormatter.sizeFormat(totals.down)}`}
                            prefix={<SwapOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <CustomStatistic
                            title={t('pages.inbounds.totalUsage')}
                            value={SizeFormatter.sizeFormat(totals.up + totals.down)}
                            prefix={<PieChartOutlined />}
                          />
                        </Col>
                        <Col xs={24} sm={24} md={8}>
                          <CustomStatistic
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
                      dbInbounds={dbInbounds as any}
                      clientCount={clientCount}
                      onlineClients={onlineClients}
                      lastOnlineMap={lastOnlineMap}
                      expireDiff={expireDiff}
                      trafficDiff={trafficDiff}
                      pageSize={pageSize}
                      isMobile={isMobile}
                      subEnable={subSettings.enable}
                      nodesById={nodesById}
                      hasActiveNode={showNodeInfo}
                      onAddInbound={onAddInbound}
                      onGeneralAction={onGeneralAction}
                      onRowAction={onRowAction}
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
            dbInbounds={dbInbounds as any[]}
            availableNodes={nodesList}
          />
        </LazyMount>
        <LazyMount when={infoOpen}>
          <InboundInfoModal
            open={infoOpen}
            onClose={() => setInfoOpen(false)}
            dbInbound={infoDbInbound}
            clientIndex={infoClientIndex}
            remarkModel={remarkModel}
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
            remarkModel={remarkModel}
            nodeAddress={qrNodeAddress}
            subSettings={subSettings}
          />
        </LazyMount>

        <LazyMount when={textOpen}>
          <TextModal
            open={textOpen}
            onClose={() => setTextOpen(false)}
            title={textTitle}
            content={textContent}
            fileName={textFileName}
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
            onConfirm={onPromptConfirm}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}
