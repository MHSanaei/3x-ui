import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Space, Tabs, Tag, Tooltip } from 'antd';
import { getMessage } from '@/utils/messageBus';
import { CopyOutlined, SyncOutlined, DeleteOutlined, DownloadOutlined } from '@ant-design/icons';

import {
  HttpUtil,
  IntlUtil,
  SizeFormatter,
  ColorUtils,
  ClipboardManager,
  FileManager,
} from '@/utils';
import { Protocols } from '@/schemas/primitives';
import InfinityIcon from '@/components/InfinityIcon';
import { useDatepicker } from '@/hooks/useDatepicker';
import { coerceInboundJsonField } from '@/models/dbinbound';
import {
  canEnableTlsFlow,
  isSS2022 as isSS2022Helper,
  isSSMultiUser as isSSMultiUserHelper,
} from '@/lib/xray/protocol-capabilities';
import {
  genAllLinks,
  genWireguardConfigs,
  genWireguardLinks,
} from '@/lib/xray/inbound-link';
import { inboundFromDb } from '@/lib/xray/inbound-from-db';
import type { SubSettings } from './useInbounds';
import './InboundInfoModal.css';

const LINK_PROTOCOLS: ReadonlySet<string> = new Set([
  Protocols.VMESS,
  Protocols.VLESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
]);

function hasShareLink(protocol: string): boolean {
  return LINK_PROTOCOLS.has(protocol);
}

function readHeader(headers: unknown, name: string): string {
  const needle = name.toLowerCase();
  if (Array.isArray(headers)) {
    for (const h of headers) {
      if (h && typeof h === 'object' && String((h as { name?: string }).name ?? '').toLowerCase() === needle) {
        return String((h as { value?: unknown }).value ?? '');
      }
    }
    return '';
  }
  if (headers && typeof headers === 'object') {
    for (const [k, v] of Object.entries(headers as Record<string, unknown>)) {
      if (k.toLowerCase() === needle) {
        return Array.isArray(v) ? String(v[0] ?? '') : String(v ?? '');
      }
    }
  }
  return '';
}

function readNetworkHost(stream: Record<string, unknown>, network: string): string | null {
  switch (network) {
    case 'tcp': {
      const tcp = stream.tcpSettings as { header?: { request?: { headers?: unknown } } } | undefined;
      return readHeader(tcp?.header?.request?.headers, 'host');
    }
    case 'ws': {
      const ws = stream.wsSettings as { host?: string; headers?: unknown } | undefined;
      return (ws?.host && ws.host.length > 0) ? ws.host : readHeader(ws?.headers, 'host');
    }
    case 'httpupgrade': {
      const hu = stream.httpupgradeSettings as { host?: string; headers?: unknown } | undefined;
      return (hu?.host && hu.host.length > 0) ? hu.host : readHeader(hu?.headers, 'host');
    }
    case 'xhttp': {
      const xh = stream.xhttpSettings as { host?: string; headers?: unknown } | undefined;
      return (xh?.host && xh.host.length > 0) ? xh.host : readHeader(xh?.headers, 'host');
    }
    default:
      return null;
  }
}

function readNetworkPath(stream: Record<string, unknown>, network: string): string | null {
  switch (network) {
    case 'tcp': {
      const tcp = stream.tcpSettings as { header?: { request?: { path?: string[] } } } | undefined;
      return tcp?.header?.request?.path?.[0] ?? null;
    }
    case 'ws':
      return (stream.wsSettings as { path?: string } | undefined)?.path ?? null;
    case 'httpupgrade':
      return (stream.httpupgradeSettings as { path?: string } | undefined)?.path ?? null;
    case 'xhttp':
      return (stream.xhttpSettings as { path?: string } | undefined)?.path ?? null;
    default:
      return null;
  }
}

interface ClientStats {
  email: string;
  up: number;
  down: number;
  total: number;
  expiryTime: number;
  enable?: boolean;
}

interface ClientSetting {
  email?: string;
  id?: string;
  security?: string;
  password?: string;
  flow?: string;
  subId?: string;
  totalGB?: number;
  expiryTime?: number;
  comment?: string;
  tgId?: string;
  enable?: boolean;
  limitIp?: number;
  created_at?: number;
  updated_at?: number;
}

interface InboundInfo {
  protocol: string;
  clients: ClientSetting[];
  settings: Record<string, unknown>;
  isTcp: boolean;
  isWs: boolean;
  isHttpupgrade: boolean;
  isXHTTP: boolean;
  isGrpc: boolean;
  isSSMultiUser: boolean;
  isSS2022: boolean;
  isVlessTlsFlow: boolean;
  host: string | null;
  path: string | null;
  serviceName: string;
  serverName: string;
  stream: {
    network: string;
    security: string;
    xhttp?: { mode?: string };
    grpc?: { multiMode?: boolean };
  };
}

interface DBInboundLike {
  id: number;
  address: string;
  port: number;
  listen: string;
  protocol: string;
  remark: string;
  enable?: boolean;
  isVMess?: boolean;
  isVLess?: boolean;
  isTrojan?: boolean;
  isSS?: boolean;
  isMixed?: boolean;
  isHTTP?: boolean;
  isWireguard?: boolean;
  settings: unknown;
  streamSettings: unknown;
  sniffing: unknown;
  clientStats?: ClientStats[];
}

function buildInboundInfo(dbInbound: DBInboundLike): InboundInfo {
  const settings = coerceInboundJsonField(dbInbound.settings) as Record<string, unknown>;
  const stream = coerceInboundJsonField(dbInbound.streamSettings) as Record<string, unknown>;
  const network = (stream.network as string | undefined) ?? '';
  const security = (stream.security as string | undefined) ?? 'none';
  const clients = Array.isArray(settings.clients) ? (settings.clients as ClientSetting[]) : [];
  const xhttpSettings = stream.xhttpSettings as { mode?: string } | undefined;
  const grpcSettings = stream.grpcSettings as { multiMode?: boolean; serviceName?: string } | undefined;
  let serverName = '';
  if (security === 'tls') {
    const tls = stream.tlsSettings as { sni?: string; serverName?: string } | undefined;
    serverName = tls?.sni ?? tls?.serverName ?? '';
  } else if (security === 'reality') {
    const reality = stream.realitySettings as { serverNames?: string[]; serverName?: string } | undefined;
    if (Array.isArray(reality?.serverNames)) {
      serverName = reality.serverNames.join(', ');
    } else if (reality?.serverName) {
      serverName = reality.serverName;
    }
  }
  return {
    protocol: dbInbound.protocol,
    clients,
    settings,
    isTcp: network === 'tcp',
    isWs: network === 'ws',
    isHttpupgrade: network === 'httpupgrade',
    isXHTTP: network === 'xhttp',
    isGrpc: network === 'grpc',
    isSSMultiUser: isSSMultiUserHelper({
      protocol: dbInbound.protocol,
      settings: settings as { method?: string },
    }),
    isSS2022: isSS2022Helper({
      protocol: dbInbound.protocol,
      settings: settings as { method?: string },
    }),
    isVlessTlsFlow: canEnableTlsFlow({
      protocol: dbInbound.protocol,
      streamSettings: { network, security },
    }),
    host: readNetworkHost(stream, network),
    path: readNetworkPath(stream, network),
    serviceName: grpcSettings?.serviceName ?? '',
    serverName,
    stream: {
      network,
      security,
      xhttp: xhttpSettings ? { mode: xhttpSettings.mode } : undefined,
      grpc: grpcSettings ? { multiMode: grpcSettings.multiMode } : undefined,
    },
  };
}

interface InboundInfoModalProps {
  open: boolean;
  onClose: () => void;
  dbInbound: DBInboundLike | null;
  clientIndex?: number;
  remarkModel?: string;
  expireDiff?: number;
  trafficDiff?: number;
  ipLimitEnable?: boolean;
  tgBotEnable?: boolean;
  nodeAddress?: string;
  subSettings?: SubSettings;
  lastOnlineMap?: Record<string, number>;
}

function copyText(value: unknown, t: (k: string) => string) {
  ClipboardManager.copyText(String(value ?? '')).then((ok) => {
    if (ok) getMessage().success(t('copied'));
  });
}

function downloadText(content: string, filename: string) {
  FileManager.downloadTextFile(content, filename);
}

function statsColor(stats: ClientStats, trafficDiff: number) {
  return ColorUtils.usageColor(stats.up + stats.down, trafficDiff, stats.total);
}

function formatIpInfo(record: unknown) {
  if (record == null) return '';
  if (typeof record === 'string' || typeof record === 'number') return String(record);
  const r = record as { ip?: string; IP?: string; timestamp?: number | string; Timestamp?: number | string };
  const ip = r.ip || r.IP || '';
  const ts = r.timestamp || r.Timestamp || 0;
  if (!ip) return String(record);
  if (!ts) return String(ip);
  const date = new Date(Number(ts) * 1000);
  const timeStr = date
    .toLocaleString('en-GB', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
      hour12: false,
    })
    .replace(',', '');
  return `${ip} (${timeStr})`;
}

export default function InboundInfoModal({
  open,
  onClose,
  dbInbound,
  clientIndex = 0,
  remarkModel = '-io',
  expireDiff = 0,
  trafficDiff = 0,
  ipLimitEnable = false,
  tgBotEnable = false,
  nodeAddress = '',
  subSettings,
  lastOnlineMap = {},
}: InboundInfoModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();

  const [inbound, setInbound] = useState<InboundInfo | null>(null);
  const [clientSettings, setClientSettings] = useState<ClientSetting | null>(null);
  const [clientStats, setClientStats] = useState<ClientStats | null>(null);
  const [links, setLinks] = useState<{ remark?: string; link: string }[]>([]);
  const [wireguardConfigs, setWireguardConfigs] = useState<string[]>([]);
  const [wireguardLinks, setWireguardLinks] = useState<string[]>([]);
  const [subLink, setSubLink] = useState('');
  const [subJsonLink, setSubJsonLink] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [clientIpsArray, setClientIpsArray] = useState<string[]>([]);
  const [clientIpsText, setClientIpsText] = useState('');
  const [activeTab, setActiveTab] = useState('client');

  const loadClientIps = useCallback(async () => {
    if (!clientStats?.email) return;
    setRefreshing(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/ips/${clientStats.email}`);
      if (!msg?.success) {
        setClientIpsText((msg?.obj as string) || 'No IP record');
        setClientIpsArray([]);
        return;
      }
      let ips: unknown = msg.obj;
      if (typeof ips === 'string') {
        try {
          ips = JSON.parse(ips);
        } catch {
          setClientIpsText(String(ips));
          setClientIpsArray([String(ips)]);
          return;
        }
      }
      if (ips && !Array.isArray(ips) && typeof ips === 'object') ips = [ips];
      if (Array.isArray(ips) && ips.length > 0) {
        const arr = (ips as unknown[]).map(formatIpInfo).filter(Boolean) as string[];
        setClientIpsArray(arr);
        setClientIpsText(arr.join(' | '));
      } else {
        setClientIpsArray([]);
        setClientIpsText(String(ips || t('tgbot.noIpRecord')));
      }
    } finally {
      setRefreshing(false);
    }
  }, [clientStats, t]);

  const clearClientIps = useCallback(async () => {
    if (!clientStats?.email) return;
    const msg = await HttpUtil.post(`/panel/api/clients/clearIps/${clientStats.email}`);
    if (msg?.success) {
      setClientIpsArray([]);
      setClientIpsText(t('tgbot.noIpRecord'));
    }
  }, [clientStats, t]);

  useEffect(() => {
    if (!open || !dbInbound) return;
    const info = buildInboundInfo(dbInbound);
    setInbound(info);
    setActiveTab(info.clients.length > 0 ? 'client' : 'inbound');

    const idx = clientIndex ?? 0;
    const clientSet = info.clients.length > 0 ? (info.clients[idx] || null) : null;
    setClientSettings(clientSet);
    const stats = clientSet
      ? (dbInbound.clientStats || []).find((s) => s.email === clientSet.email) || null
      : null;
    setClientStats(stats);

    const inboundForLinks = inboundFromDb(dbInbound);
    const fallbackHostname = window.location.hostname;
    if (info.protocol === Protocols.WIREGUARD) {
      setWireguardConfigs(
        genWireguardConfigs({
          inbound: inboundForLinks,
          remark: dbInbound.remark,
          remarkModel: '-io',
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setWireguardLinks(
        genWireguardLinks({
          inbound: inboundForLinks,
          remark: dbInbound.remark,
          remarkModel: '-io',
          hostOverride: nodeAddress,
          fallbackHostname,
        }).split('\r\n'),
      );
      setLinks([]);
    } else {
      setLinks(
        genAllLinks({
          inbound: inboundForLinks,
          remark: dbInbound.remark,
          remarkModel,
          client: (clientSet ?? {}) as Parameters<typeof genAllLinks>[0]['client'],
          hostOverride: nodeAddress,
          fallbackHostname,
        }),
      );
      setWireguardConfigs([]);
      setWireguardLinks([]);
    }

    if (clientSet?.subId) {
      setSubLink((subSettings?.subURI || '') + clientSet.subId);
      setSubJsonLink(
        subSettings?.subJsonEnable ? (subSettings?.subJsonURI || '') + clientSet.subId : '',
      );
    } else {
      setSubLink('');
      setSubJsonLink('');
    }

    setClientIpsArray([]);
    setClientIpsText('');

    if (ipLimitEnable && (clientSet?.limitIp ?? 0) > 0 && stats?.email) {
      void HttpUtil.post(`/panel/api/clients/ips/${stats.email}`).then((msg) => {
        if (!msg?.success) {
          setClientIpsText((msg?.obj as string) || 'No IP record');
          return;
        }
        let ips: unknown = msg.obj;
        if (typeof ips === 'string') {
          try {
            ips = JSON.parse(ips);
          } catch {
            setClientIpsText(String(ips));
            setClientIpsArray([String(ips)]);
            return;
          }
        }
        if (ips && !Array.isArray(ips) && typeof ips === 'object') ips = [ips];
        if (Array.isArray(ips) && ips.length > 0) {
          const arr = (ips as unknown[]).map(formatIpInfo).filter(Boolean) as string[];
          setClientIpsArray(arr);
          setClientIpsText(arr.join(' | '));
        } else {
          setClientIpsText(String(ips || t('tgbot.noIpRecord')));
        }
      });
    }
  }, [open, dbInbound, clientIndex, remarkModel, nodeAddress, subSettings, ipLimitEnable, t]);

  const isEnable = useMemo(() => {
    if (clientSettings) return !!clientSettings.enable;
    return dbInbound?.enable ?? true;
  }, [clientSettings, dbInbound]);

  const isDepleted = useMemo(() => {
    if (!clientStats || !clientSettings) return false;
    const total = clientStats.total ?? 0;
    const used = (clientStats.up ?? 0) + (clientStats.down ?? 0);
    if (total > 0 && used >= total) return true;
    const expiry = clientSettings.expiryTime ?? 0;
    if (expiry > 0 && Date.now() >= expiry) return true;
    return false;
  }, [clientStats, clientSettings]);

  const remainingStats = useMemo(() => {
    if (!clientStats || !clientSettings) return '-';
    const remained = clientStats.total - clientStats.up - clientStats.down;
    return remained > 0 ? SizeFormatter.sizeFormat(remained) : '-';
  }, [clientStats, clientSettings]);

  const formatLastOnline = useCallback(
    (email: string) => {
      const ts = lastOnlineMap[email];
      if (!ts) return '-';
      return IntlUtil.formatDate(ts, datepicker);
    },
    [lastOnlineMap, datepicker],
  );

  const networkLabel = inbound?.stream?.network || '';
  const securityLabel = inbound?.stream?.security || 'none';
  const securityColor = securityLabel === 'none' ? 'red' : 'green';
  const encryptionLabel = (inbound?.settings?.encryption as string) || '';
  const serverNameLabel = inbound?.serverName || '';
  const showClientTab = !!clientSettings;
  const showSubscriptionTab = !!(subSettings?.enable && clientSettings?.subId);

  if (!dbInbound || !inbound) {
    return (
      <Modal open={open} onCancel={onClose} title={t('pages.inbounds.inboundData')} footer={null} width={640} />
    );
  }

  const clientTab = (
    <>
      <table className="info-table block">
        <tbody>
          <tr>
            <td>{t('pages.inbounds.email')}</td>
            <td>
              {clientSettings?.email ? (
                <Tag color="green">{clientSettings.email}</Tag>
              ) : (
                <Tag color="red">{t('none')}</Tag>
              )}
            </td>
          </tr>
          {clientSettings?.id && (
            <tr><td>ID</td><td><Tag>{clientSettings.id}</Tag></td></tr>
          )}
          {dbInbound.isVMess && (
            <tr><td>{t('security')}</td><td><Tag>{clientSettings?.security}</Tag></td></tr>
          )}
          {inbound.isVlessTlsFlow && (
            <tr>
              <td>{t('pages.clients.flow')}</td>
              <td>
                {clientSettings?.flow ? <Tag>{clientSettings.flow}</Tag> : <Tag color="orange">{t('none')}</Tag>}
              </td>
            </tr>
          )}
          {clientSettings?.password && (
            <tr>
              <td>{t('password')}</td>
              <td><Tag className="info-large-tag">{clientSettings.password}</Tag></td>
            </tr>
          )}
          <tr>
            <td>{t('status')}</td>
            <td>
              {isDepleted ? (
                <Tag color="red">{t('depleted')}</Tag>
              ) : isEnable ? (
                <Tag color="green">{t('enabled')}</Tag>
              ) : (
                <Tag>{t('disabled')}</Tag>
              )}
            </td>
          </tr>
          {clientStats && (
            <tr>
              <td>{t('usage')}</td>
              <td>
                <Tag color="green">{SizeFormatter.sizeFormat(clientStats.up + clientStats.down)}</Tag>
                <Tag>
                  ↑ {SizeFormatter.sizeFormat(clientStats.up)} /
                  {' '}{SizeFormatter.sizeFormat(clientStats.down)} ↓
                </Tag>
              </td>
            </tr>
          )}
          <tr>
            <td>{t('pages.inbounds.createdAt')}</td>
            <td>
              {clientSettings?.created_at ? (
                <Tag>{IntlUtil.formatDate(clientSettings.created_at, datepicker)}</Tag>
              ) : <Tag>-</Tag>}
            </td>
          </tr>
          <tr>
            <td>{t('pages.inbounds.updatedAt')}</td>
            <td>
              {clientSettings?.updated_at ? (
                <Tag>{IntlUtil.formatDate(clientSettings.updated_at, datepicker)}</Tag>
              ) : <Tag>-</Tag>}
            </td>
          </tr>
          <tr>
            <td>{t('lastOnline')}</td>
            <td><Tag>{formatLastOnline(clientSettings?.email || '')}</Tag></td>
          </tr>
          {clientSettings?.comment && (
            <tr><td>{t('comment')}</td><td><Tag className="info-large-tag">{clientSettings.comment}</Tag></td></tr>
          )}
          {ipLimitEnable && (
            <tr><td>{t('pages.inbounds.IPLimit')}</td><td><Tag>{clientSettings?.limitIp ?? 0}</Tag></td></tr>
          )}
          {ipLimitEnable && (clientSettings?.limitIp ?? 0) > 0 && (
            <tr>
              <td>{t('pages.inbounds.IPLimitlog')}</td>
              <td>
                <div className="ip-log">
                  {clientIpsArray.length > 0 ? (
                    <div>
                      {clientIpsArray.map((item, idx) => (
                        <Tag color="blue" className="ip-log-row" key={idx}>{item}</Tag>
                      ))}
                    </div>
                  ) : (
                    <Tag>{clientIpsText || t('tgbot.noIpRecord')}</Tag>
                  )}
                </div>
                <div className="ip-log-actions">
                  <SyncOutlined spin={refreshing} onClick={() => loadClientIps()} />
                  <Tooltip title={t('pages.inbounds.IPLimitlogclear')}>
                    <DeleteOutlined onClick={() => clearClientIps()} />
                  </Tooltip>
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>

      <table className="info-table summary-table">
        <thead>
          <tr>
            <th>{t('remained')}</th>
            <th>{t('pages.inbounds.totalUsage')}</th>
            <th>{t('pages.inbounds.expireDate')}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>
              {clientStats && (clientSettings?.totalGB ?? 0) > 0 ? (
                <Tag color={statsColor(clientStats, trafficDiff)}>{remainingStats}</Tag>
              ) : !clientSettings?.totalGB || clientSettings.totalGB <= 0 ? (
                <Tag color="purple"><InfinityIcon /></Tag>
              ) : null}
            </td>
            <td>
              {(clientSettings?.totalGB ?? 0) > 0 ? (
                <Tag color={clientStats ? statsColor(clientStats, trafficDiff) : 'default'}>
                  {SizeFormatter.sizeFormat(clientSettings!.totalGB!)}
                </Tag>
              ) : (
                <Tag color="purple"><InfinityIcon /></Tag>
              )}
            </td>
            <td>
              {(clientSettings?.expiryTime ?? 0) > 0 ? (
                <Tag color={ColorUtils.usageColor(Date.now(), expireDiff, clientSettings!.expiryTime!)}>
                  {IntlUtil.formatDate(clientSettings!.expiryTime!, datepicker)}
                </Tag>
              ) : (clientSettings?.expiryTime ?? 0) < 0 ? (
                <Tag color="green">{clientSettings!.expiryTime! / -86400000} {t('day')}</Tag>
              ) : (
                <Tag color="purple"><InfinityIcon /></Tag>
              )}
            </td>
          </tr>
        </tbody>
      </table>

      {tgBotEnable && clientSettings?.tgId && (
        <>
          <Divider>Telegram</Divider>
          <div className="tg-row">
            <Tag color="blue">{clientSettings.tgId}</Tag>
            <Tooltip title={t('copy')}>
              <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(clientSettings.tgId, t)} />
            </Tooltip>
          </div>
        </>
      )}

      {hasShareLink(dbInbound.protocol) && links.length > 0 && (
        <>
          <Divider>{t('pages.inbounds.copyLink')}</Divider>
          {links.map((link, idx) => (
            <div key={idx} className="link-panel">
              <div className="link-panel-header">
                <Tag color="green">{link.remark || `Link ${idx + 1}`}</Tag>
                <Tooltip title={t('copy')}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(link.link, t)} />
                </Tooltip>
              </div>
              <code className="link-panel-text">{link.link}</code>
            </div>
          ))}
        </>
      )}

      {showSubscriptionTab && (
        <>
          <Divider>{t('subscription.title')}</Divider>
          <div className="link-panel">
            <div className="link-panel-header">
              <Tag color="green">{t('subscription.title')}</Tag>
              <Tooltip title={t('copy')}>
                <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(subLink, t)} />
              </Tooltip>
            </div>
            <a href={subLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">{subLink}</a>
          </div>
          {subSettings?.subJsonEnable && subJsonLink && (
            <div className="link-panel">
              <div className="link-panel-header">
                <Tag color="green">JSON</Tag>
                <Tooltip title={t('copy')}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(subJsonLink, t)} />
                </Tooltip>
              </div>
              <a href={subJsonLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">{subJsonLink}</a>
            </div>
          )}
        </>
      )}
    </>
  );

  const inboundTab = (
    <>
      <dl className="info-list">
        <div className="info-row">
          <dt>{t('pages.inbounds.protocol')}</dt>
          <dd><Tag color="purple">{dbInbound.protocol}</Tag></dd>
        </div>
        <div className="info-row">
          <dt>{t('pages.inbounds.address')}</dt>
          <dd><Tag className="value-tag">{dbInbound.address}</Tag></dd>
        </div>
        <div className="info-row">
          <dt>{t('pages.inbounds.port')}</dt>
          <dd><Tag>{dbInbound.port}</Tag></dd>
        </div>

        {(dbInbound.isVMess || dbInbound.isVLess || dbInbound.isTrojan || dbInbound.isSS) && (
          <>
            <div className="info-row">
              <dt>{t('transmission')}</dt>
              <dd><Tag color="green">{networkLabel}</Tag></dd>
            </div>
            {(inbound.isTcp || inbound.isWs || inbound.isHttpupgrade || inbound.isXHTTP) && (
              <>
                <div className="info-row">
                  <dt>{t('host')}</dt>
                  <dd>{inbound.host ? <Tag className="value-tag">{inbound.host}</Tag> : <Tag color="orange">{t('none')}</Tag>}</dd>
                </div>
                <div className="info-row">
                  <dt>{t('path')}</dt>
                  <dd>{inbound.path ? <Tag className="value-tag">{inbound.path}</Tag> : <Tag color="orange">{t('none')}</Tag>}</dd>
                </div>
              </>
            )}
            {inbound.isXHTTP && (
              <div className="info-row">
                <dt>{t('pages.inbounds.info.mode')}</dt>
                <dd><Tag>{inbound.stream?.xhttp?.mode}</Tag></dd>
              </div>
            )}
            {inbound.isGrpc && (
              <>
                <div className="info-row">
                  <dt>{t('pages.inbounds.info.grpcServiceName')}</dt>
                  <dd><Tag className="value-tag">{inbound.serviceName}</Tag></dd>
                </div>
                <div className="info-row">
                  <dt>{t('pages.inbounds.info.grpcMultiMode')}</dt>
                  <dd><Tag>{String(inbound.stream?.grpc?.multiMode)}</Tag></dd>
                </div>
              </>
            )}
          </>
        )}

        {hasShareLink(dbInbound.protocol) && (
          <>
            <div className="info-row">
              <dt>{t('security')}</dt>
              <dd><Tag color={securityColor}>{securityLabel}</Tag></dd>
            </div>
            {encryptionLabel && (
              <div className="info-row">
                <dt>{t('encryption')}</dt>
                <dd className="value-block">
                  <code className="value-code">{encryptionLabel}</code>
                  <Tooltip title={t('copy')}>
                    <Button size="small" className="value-copy" icon={<CopyOutlined />} onClick={() => copyText(encryptionLabel, t)} />
                  </Tooltip>
                </dd>
              </div>
            )}
            {securityLabel !== 'none' && (
              <div className="info-row">
                <dt>{t('domainName')}</dt>
                <dd>
                  {serverNameLabel ? (
                    <Tag color="green" className="value-tag">{serverNameLabel}</Tag>
                  ) : (
                    <Tag color="orange">{t('none')}</Tag>
                  )}
                </dd>
              </div>
            )}
          </>
        )}
      </dl>

      {dbInbound.isSS && inbound.settings && (
        <table className="info-table block">
          <tbody>
            <tr>
              <td>{t('encryption')}</td>
              <td><Tag color="green">{inbound.settings.method as string}</Tag></td>
            </tr>
            {inbound.isSS2022 && (
              <tr>
                <td>{t('password')}</td>
                <td><Tag className="info-large-tag">{inbound.settings.password as string}</Tag></td>
              </tr>
            )}
            <tr>
              <td>{t('pages.inbounds.network')}</td>
              <td><Tag color="green">{inbound.settings.network as string}</Tag></td>
            </tr>
          </tbody>
        </table>
      )}

      {inbound.protocol === Protocols.TUN && inbound.settings && (
        <dl className="info-list info-list-block">
          <div className="info-row">
            <dt>{t('pages.inbounds.info.interfaceName')}</dt>
            <dd><Tag color="green" className="value-tag">{inbound.settings.name as string}</Tag></dd>
          </div>
          <div className="info-row">
            <dt>{t('pages.inbounds.info.mtu')}</dt>
            <dd><Tag color="green">{inbound.settings.mtu as number}</Tag></dd>
          </div>
          {Array.isArray(inbound.settings.gateway) && (inbound.settings.gateway as string[]).length > 0 && (
            <div className="info-row">
              <dt>{t('pages.inbounds.info.gateway')}</dt>
              <dd>
                {(inbound.settings.gateway as string[]).map((ip, j) => (
                  <Tag key={`tun-gw-${j}`} color="green" className="value-tag">{ip}</Tag>
                ))}
              </dd>
            </div>
          )}
          {Array.isArray(inbound.settings.dns) && (inbound.settings.dns as string[]).length > 0 && (
            <div className="info-row">
              <dt>{t('pages.inbounds.info.dns')}</dt>
              <dd>
                {(inbound.settings.dns as string[]).map((ip, j) => (
                  <Tag key={`tun-dns-${j}`} color="green">{ip}</Tag>
                ))}
              </dd>
            </div>
          )}
          <div className="info-row">
            <dt>{t('pages.inbounds.info.outboundsInterface')}</dt>
            <dd><Tag color="green">{(inbound.settings.autoOutboundsInterface as string) || 'auto'}</Tag></dd>
          </div>
          {Array.isArray(inbound.settings.autoSystemRoutingTable) && (inbound.settings.autoSystemRoutingTable as string[]).length > 0 && (
            <div className="info-row">
              <dt>{t('pages.inbounds.info.autoSystemRoutes')}</dt>
              <dd>
                {(inbound.settings.autoSystemRoutingTable as string[]).map((cidr, j) => (
                  <Tag key={`tun-rt-${j}`} color="green">{cidr}</Tag>
                ))}
              </dd>
            </div>
          )}
        </dl>
      )}

      {inbound.protocol === Protocols.TUNNEL && inbound.settings && (
        <dl className="info-list info-list-block">
          <div className="info-row">
            <dt>{t('pages.inbounds.targetAddress')}</dt>
            <dd><Tag color="green" className="value-tag">{inbound.settings.rewriteAddress as string}</Tag></dd>
          </div>
          <div className="info-row">
            <dt>{t('pages.inbounds.destinationPort')}</dt>
            <dd><Tag color="green">{inbound.settings.rewritePort as number}</Tag></dd>
          </div>
          <div className="info-row">
            <dt>{t('pages.inbounds.network')}</dt>
            <dd><Tag color="green">{inbound.settings.allowedNetwork as string}</Tag></dd>
          </div>
          <div className="info-row">
            <dt>{t('pages.inbounds.info.followRedirect')}</dt>
            <dd>
              <Tag color={inbound.settings.followRedirect ? 'green' : 'red'}>
                {inbound.settings.followRedirect ? t('enabled') : t('disabled')}
              </Tag>
            </dd>
          </div>
        </dl>
      )}

      {dbInbound.isMixed && inbound.settings && (
        <dl className="info-list info-list-block">
          <div className="info-row">
            <dt>{t('pages.inbounds.info.auth')}</dt>
            <dd>
              <Tag color={inbound.settings.auth === 'password' ? 'green' : 'orange'}>
                {inbound.settings.auth as string}
              </Tag>
            </dd>
          </div>
          <div className="info-row">
            <dt>UDP</dt>
            <dd>
              <Tag color={inbound.settings.udp ? 'green' : 'red'}>
                {inbound.settings.udp ? t('enabled') : t('disabled')}
              </Tag>
            </dd>
          </div>
          {(inbound.settings.ip as string) && (
            <div className="info-row">
              <dt>IP</dt>
              <dd><Tag className="value-tag">{inbound.settings.ip as string}</Tag></dd>
            </div>
          )}
          {inbound.settings.auth === 'password' && Array.isArray(inbound.settings.accounts) && (
            <>
              {(inbound.settings.accounts as { user: string; pass: string }[]).map((account, idx) => (
                <div key={idx} className="info-row">
                  <dt>{t('username')} #{idx + 1}</dt>
                  <dd className="account-row">
                    <Tag color="green" className="value-tag">{account.user}</Tag>
                    <span className="account-sep">:</span>
                    <Tag className="value-tag">{account.pass}</Tag>
                    <Tooltip title={t('copy')}>
                      <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyText(`${account.user}:${account.pass}`, t)} />
                    </Tooltip>
                    <Space size={4} wrap className="share-buttons">
                      <Tooltip title={`socks5://${account.user}:${account.pass}@${dbInbound.address}:${dbInbound.port}`}>
                        <Button size="small" onClick={() => copyText(`socks5://${account.user}:${account.pass}@${dbInbound.address}:${dbInbound.port}`, t)}>SOCKS5</Button>
                      </Tooltip>
                      <Tooltip title={`http://${account.user}:${account.pass}@${dbInbound.address}:${dbInbound.port}`}>
                        <Button size="small" onClick={() => copyText(`http://${account.user}:${account.pass}@${dbInbound.address}:${dbInbound.port}`, t)}>HTTP</Button>
                      </Tooltip>
                      <Tooltip title="https://t.me/socks?server=...&port=...&user=...&pass=...">
                        <Button size="small" onClick={() => copyText(`https://t.me/socks?server=${encodeURIComponent(dbInbound.address)}&port=${dbInbound.port}&user=${encodeURIComponent(account.user)}&pass=${encodeURIComponent(account.pass)}`, t)}>Telegram</Button>
                      </Tooltip>
                    </Space>
                  </dd>
                </div>
              ))}
            </>
          )}
          {inbound.settings.auth === 'noauth' && (
            <div className="info-row">
              <dt>{t('copy')}</dt>
              <dd>
                <Space size={4} wrap className="share-buttons">
                  <Tooltip title={`socks5://${dbInbound.address}:${dbInbound.port}`}>
                    <Button size="small" onClick={() => copyText(`socks5://${dbInbound.address}:${dbInbound.port}`, t)}>SOCKS5</Button>
                  </Tooltip>
                  <Tooltip title={`http://${dbInbound.address}:${dbInbound.port}`}>
                    <Button size="small" onClick={() => copyText(`http://${dbInbound.address}:${dbInbound.port}`, t)}>HTTP</Button>
                  </Tooltip>
                  <Tooltip title="https://t.me/socks?server=...&port=...">
                    <Button size="small" onClick={() => copyText(`https://t.me/socks?server=${encodeURIComponent(dbInbound.address)}&port=${dbInbound.port}`, t)}>Telegram</Button>
                  </Tooltip>
                </Space>
              </dd>
            </div>
          )}
        </dl>
      )}

      {dbInbound.isHTTP && Array.isArray(inbound.settings?.accounts) && (inbound.settings!.accounts as unknown[]).length > 0 && (
        <dl className="info-list info-list-block">
          {(inbound.settings!.accounts as { user: string; pass: string }[]).map((account, idx) => (
            <div key={idx} className="info-row">
              <dt>{t('username')} #{idx + 1}</dt>
              <dd className="account-row">
                <Tag color="green" className="value-tag">{account.user}</Tag>
                <span className="account-sep">:</span>
                <Tag className="value-tag">{account.pass}</Tag>
                <Tooltip title={t('copy')}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(`${account.user}:${account.pass}`, t)} />
                </Tooltip>
              </dd>
            </div>
          ))}
        </dl>
      )}

      {dbInbound.isWireguard && inbound.settings && (
        <>
          <dl className="info-list info-list-block">
            <div className="info-row">
              <dt>{t('pages.xray.wireguard.secretKey')}</dt>
              <dd><Tag className="value-tag">{inbound.settings.secretKey as string}</Tag></dd>
            </div>
            <div className="info-row">
              <dt>{t('pages.xray.wireguard.publicKey')}</dt>
              <dd><Tag className="value-tag">{inbound.settings.pubKey as string}</Tag></dd>
            </div>
            <div className="info-row">
              <dt>{t('pages.inbounds.info.mtu')}</dt>
              <dd><Tag>{inbound.settings.mtu as number}</Tag></dd>
            </div>
            <div className="info-row">
              <dt>{t('pages.inbounds.info.noKernelTun')}</dt>
              <dd>
                <Tag color={inbound.settings.noKernelTun ? 'green' : 'default'}>
                  {String(inbound.settings.noKernelTun)}
                </Tag>
              </dd>
            </div>
          </dl>
          {Array.isArray(inbound.settings.peers) && (inbound.settings.peers as { privateKey: string; publicKey: string; psk: string; allowedIPs?: string[]; keepAlive?: number }[]).map((peer, idx) => (
            <Fragment key={idx}>
              <Divider>{t('pages.inbounds.info.peerNumber', { n: idx + 1 })}</Divider>
              <dl className="info-list info-list-block">
                <div className="info-row">
                  <dt>{t('pages.xray.wireguard.secretKey')}</dt>
                  <dd><Tag className="value-tag">{peer.privateKey}</Tag></dd>
                </div>
                <div className="info-row">
                  <dt>{t('pages.xray.wireguard.publicKey')}</dt>
                  <dd><Tag className="value-tag">{peer.publicKey}</Tag></dd>
                </div>
                <div className="info-row">
                  <dt>PSK</dt>
                  <dd><Tag className="value-tag">{peer.psk}</Tag></dd>
                </div>
                <div className="info-row">
                  <dt>{t('pages.xray.wireguard.allowedIPs')}</dt>
                  <dd>
                    {(peer.allowedIPs || []).map((ip, j) => (
                      <Tag key={`wg-ip-${idx}-${j}`} className="value-tag">{ip}</Tag>
                    ))}
                  </dd>
                </div>
                <div className="info-row">
                  <dt>{t('pages.inbounds.info.keepAlive')}</dt>
                  <dd><Tag>{peer.keepAlive}</Tag></dd>
                </div>
              </dl>
              {wireguardConfigs[idx] && (
                <div className="link-panel">
                  <div className="link-panel-header">
                    <Tag color="green">{t('pages.inbounds.info.peerNumberConfig', { n: idx + 1 })}</Tag>
                    <Tooltip title={t('copy')}>
                      <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(wireguardConfigs[idx], t)} />
                    </Tooltip>
                    <Tooltip title={t('download')}>
                      <Button size="small" icon={<DownloadOutlined />} onClick={() => downloadText(wireguardConfigs[idx], `peer-${idx + 1}.conf`)} />
                    </Tooltip>
                  </div>
                  <code className="link-panel-text">{wireguardConfigs[idx]}</code>
                </div>
              )}
              {wireguardLinks[idx] && (
                <div className="link-panel">
                  <div className="link-panel-header">
                    <Tag color="green">Peer {idx + 1} link</Tag>
                    <Tooltip title={t('copy')}>
                      <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(wireguardLinks[idx], t)} />
                    </Tooltip>
                  </div>
                  <code className="link-panel-text">{wireguardLinks[idx]}</code>
                </div>
              )}
            </Fragment>
          ))}
        </>
      )}

      {dbInbound.isSS && !inbound.isSSMultiUser && links.length > 0 && (
        <>
          <Divider>{t('pages.inbounds.copyLink')}</Divider>
          {links.map((link, idx) => (
            <div key={idx} className="link-panel">
              <div className="link-panel-header">
                <Tag color="green">{link.remark || `Link ${idx + 1}`}</Tag>
                <Tooltip title={t('copy')}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => copyText(link.link, t)} />
                </Tooltip>
              </div>
              <code className="link-panel-text">{link.link}</code>
            </div>
          ))}
        </>
      )}
    </>
  );

  const tabItems = [];
  if (showClientTab) {
    tabItems.push({ key: 'client', label: t('pages.inbounds.client'), children: clientTab });
  }
  tabItems.push({ key: 'inbound', label: t('pages.xray.rules.inbound'), children: inboundTab });

  return (
    <Modal open={open} onCancel={onClose} title={t('pages.inbounds.inboundData')} footer={null} width={640} destroyOnHidden>
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />
    </Modal>
  );
}
