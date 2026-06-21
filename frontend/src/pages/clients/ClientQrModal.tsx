import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Collapse, Modal, Spin, Tooltip, message } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';
import { HttpUtil, ClipboardManager, FileManager } from '@/utils';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

interface ClientQrModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById?: Record<number, InboundOption>;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = { enable: false, subURI: '', subJsonURI: '', subJsonEnable: false };

function buildWgConfig(client: ClientRecord, inbound: InboundOption | undefined): string {
  const wg = client.wgPeer;
  if (!wg) return '';
  const serverPubKey = inbound?.wgPublicKey || '';
  const endpoint = `${window.location.hostname}:${inbound?.port || ''}`;
  const address = (wg.allowedIPs || []).join(', ') || '10.0.0.2/32';
  const lines: string[] = [
    '[Interface]',
    `PrivateKey = ${client.password || ''}`,
    `Address = ${address}`,
    'DNS = 8.8.8.8',
    '',
    '[Peer]',
    `PublicKey = ${serverPubKey}`,
    'AllowedIPs = 0.0.0.0/0, ::/0',
    `Endpoint = ${endpoint}`,
  ];
  if (wg.preSharedKey) lines.push(`PresharedKey = ${wg.preSharedKey}`);
  if (wg.keepAlive && wg.keepAlive > 0) lines.push(`PersistentKeepalive = ${wg.keepAlive}`);
  return lines.join('\n');
}

function WgConfigPanel({ config, email }: { config: string; email: string }) {
  const { t } = useTranslation();
  const [messageApi, ctx] = message.useMessage();

  async function copy() {
    const ok = await ClipboardManager.copyText(config);
    if (ok) messageApi.success(t('copied'));
  }

  function download() {
    FileManager.downloadTextFile(config, `${email}.conf`);
  }

  return (
    <div>
      {ctx}
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginBottom: 8 }}>
        <Tooltip title={t('copy')}>
          <Button size="small" icon={<CopyOutlined />} onClick={copy} />
        </Tooltip>
        <Tooltip title={t('download')}>
          <Button size="small" icon={<DownloadOutlined />} onClick={download} />
        </Tooltip>
      </div>
      <pre style={{
        background: 'var(--ant-color-fill-quaternary, #f5f5f5)',
        borderRadius: 6,
        padding: '10px 14px',
        margin: 0,
        fontSize: 13,
        lineHeight: 1.6,
        overflowX: 'auto',
        whiteSpace: 'pre',
        userSelect: 'all',
      }}>
        {config}
      </pre>
      <div style={{ marginTop: 12 }}>
        <QrPanel
          value={config}
          remark={email}
          downloadName={`${email}.conf`}
          showQr={true}
        />
      </div>
    </div>
  );
}

export default function ClientQrModal({
  open,
  client,
  inboundsById,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientQrModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const isWg = !!(client?.wgPeer);

  const subLink = useMemo(() => {
    if (isWg) return '';
    if (!client?.subId || !subSettings?.enable || !subSettings?.subURI) return '';
    return subSettings.subURI + client.subId;
  }, [isWg, client?.subId, subSettings?.enable, subSettings?.subURI]);

  const subJsonLink = useMemo(() => {
    if (isWg) return '';
    if (!client?.subId || !subSettings?.enable) return '';
    if (!subSettings?.subJsonEnable || !subSettings?.subJsonURI) return '';
    return subSettings.subJsonURI + client.subId;
  }, [isWg, client?.subId, subSettings?.enable, subSettings?.subJsonEnable, subSettings?.subJsonURI]);

  const wgInbound = useMemo(() => {
    if (!isWg || !client?.inboundIds || !inboundsById) return undefined;
    for (const id of client.inboundIds) {
      const ib = inboundsById[id];
      if (ib?.protocol === 'wireguard') return ib;
    }
    return undefined;
  }, [isWg, client?.inboundIds, inboundsById]);

  const wgConfig = useMemo(() => {
    if (!isWg || !client) return '';
    return buildWgConfig(client, wgInbound);
  }, [isWg, client, wgInbound]);

  const hasAnything = !!subLink || !!subJsonLink || links.length > 0 || !!wgConfig;

  useEffect(() => {
    if (!open || !client?.subId || isWg) {
      setLinks([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const msg = await HttpUtil.get(
          `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
        ) as ApiMsg<string[]>;
        if (!cancelled) {
          setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [open, client?.subId, isWg]);

  const [activeKey, setActiveKey] = useState<string[]>([]);

  const items = useMemo(() => {
    const out: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [];

    if (wgConfig) {
      out.push({
        key: 'wg',
        label: 'WireGuard Config',
        children: <WgConfigPanel config={wgConfig} email={client?.email || 'peer'} />,
      });
    }

    if (subLink) {
      out.push({
        key: 'sub',
        label: t('subscription.title'),
        children: <QrPanel value={subLink} remark={`${client?.email || ''} — ${t('subscription.title')}`} />,
      });
    }
    if (subJsonLink) {
      out.push({
        key: 'subJson',
        label: `${t('subscription.title')} (JSON)`,
        children: <QrPanel value={subJsonLink} remark={`${client?.email || ''} — JSON`} />,
      });
    }
    links.forEach((link, idx) => {
      const parts = parseLinkParts(link);
      const meta = parts ? linkMetaText(parts) : '';
      const label: React.ReactNode = parts ? (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
          <LinkTags parts={parts} />
          {meta && <span style={{ opacity: 0.6, fontSize: 12 }}>({meta})</span>}
        </span>
      ) : `${t('pages.clients.link')} ${idx + 1}`;
      out.push({
        key: `l${idx}`,
        label,
        children: (
          <QrPanel
            value={link}
            remark={`${client?.email || ''} #${idx + 1}`}
            showQr={!isPostQuantumLink(link)}
          />
        ),
      });
    });
    return out;
  }, [wgConfig, subLink, subJsonLink, links, client?.email, t]);

  useEffect(() => {
    if (!open) {
      setActiveKey([]);
      return;
    }
    setActiveKey(items.length > 0 ? [items[0].key] : []);
  }, [open, items]);

  return (
    <Modal
      open={open}
      title={client ? `${t('qrCode')} — ${client.email}` : t('qrCode')}
      footer={null}
      width={520}
      centered
      onCancel={() => onOpenChange(false)}
    >
      <Spin spinning={loading}>
        {!hasAnything && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>{t('pages.clients.noLinks')}</div>
        )}
        {hasAnything && (
          <Collapse
            activeKey={activeKey}
            onChange={(keys) => setActiveKey(typeof keys === 'string' ? [keys] : (keys as string[]))}
            items={items}
          />
        )}
      </Spin>
    </Modal>
  );
}
