import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal, Spin, Tag } from 'antd';
import { HttpUtil } from '@/utils';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { formatTunnelConfigMeta } from '@/lib/inbounds/label';
import {
  buildWireguardClientConfig,
  findWireguardInbounds,
  isWireguardClient,
} from './wireguardConfig';
import {
  buildAmneziaWGClientConfig,
  findAmneziaWGInbounds,
  isAmneziaWGClient,
} from './amneziawgConfig';
import { buildTuicClientConfig, findTuicInbound, isTuicClient } from './tuicConfig';

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
  publicHost?: string;
}

interface ClientQrModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById: Record<number, InboundOption>;
  tunnelAllowedIPs?: Record<number, string>;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = {
  enable: false,
  subURI: '',
  subJsonURI: '',
  subJsonEnable: false,
  publicHost: '',
};

export default function ClientQrModal({
  open,
  client,
  inboundsById,
  tunnelAllowedIPs,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientQrModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const subId = client?.subId;
  const subEnabled = !!subSettings?.enable;
  const subLink = subId && subEnabled && subSettings?.subURI ? subSettings.subURI + subId : '';
  const subJsonLink =
    subId && subEnabled && subSettings?.subJsonEnable && subSettings?.subJsonURI
      ? subSettings.subJsonURI + subId
      : '';

  const wgInbounds = useMemo(
    () => findWireguardInbounds(client, inboundsById),
    [client, inboundsById],
  );
  const wgConfigs = useMemo(() => {
    if (!client || !isWireguardClient(client)) return [];
    return wgInbounds
      .map((ib) => {
        const address = tunnelAllowedIPs?.[ib.id] ?? '';
        const text = buildWireguardClientConfig(
          client,
          ib,
          window.location.hostname,
          subSettings?.publicHost ?? '',
          address,
        );
        return { inbound: ib, text };
      })
      .filter((c) => !!c.text);
  }, [client, wgInbounds, tunnelAllowedIPs, subSettings?.publicHost]);

  const awgInbounds = useMemo(
    () => findAmneziaWGInbounds(client, inboundsById),
    [client, inboundsById],
  );
  const awgConfigs = useMemo(() => {
    if (!client || !isAmneziaWGClient(client)) return [];
    return awgInbounds
      .map((ib) => {
        const address = tunnelAllowedIPs?.[ib.id] ?? '';
        const text = buildAmneziaWGClientConfig(
          client,
          ib,
          window.location.hostname,
          subSettings?.publicHost ?? '',
          address,
        );
        return { inbound: ib, text };
      })
      .filter((c) => !!c.text);
  }, [client, awgInbounds, tunnelAllowedIPs, subSettings?.publicHost]);

  const tuicInbound = useMemo(() => findTuicInbound(client, inboundsById), [client, inboundsById]);
  const tuicConfigText = useMemo(() => {
    if (!client || !tuicInbound || !isTuicClient(client)) return '';
    return buildTuicClientConfig(
      client,
      tuicInbound,
      window.location.hostname,
      subSettings?.publicHost ?? '',
    );
  }, [client, tuicInbound, subSettings?.publicHost]);

  const hasAnything =
    !!subLink ||
    !!subJsonLink ||
    wgConfigs.length > 0 ||
    awgConfigs.length > 0 ||
    !!tuicConfigText ||
    links.length > 0;

  // The reset runs during render so the effect only carries the request.
  const openSubId = open ? (client?.subId ?? '') : '';
  const [syncedSubId, setSyncedSubId] = useState(openSubId);
  if (openSubId !== syncedSubId) {
    setSyncedSubId(openSubId);
    setLinks([]);
    setLoading(!!openSubId);
  }

  useEffect(() => {
    if (!open || !client?.subId) return;
    let cancelled = false;
    (async () => {
      try {
        const msg = (await HttpUtil.get(
          `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
        )) as ApiMsg<string[]>;
        if (!cancelled) {
          setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, client?.subId]);

  const [activeKey, setActiveKey] = useState<string[]>([]);

  const items = useMemo(() => {
    const out: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [];
    if (subLink) {
      out.push({
        key: 'sub',
        label: t('subscription.title'),
        children: (
          <QrPanel value={subLink} remark={`${client?.email || ''} — ${t('subscription.title')}`} />
        ),
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
      ) : (
        `${t('pages.clients.link')} ${idx + 1}`
      );
      out.push({
        key: `l${idx}`,
        label,
        children: (
          <QrPanel
            value={link}
            remark={parts?.remark || `${client?.email || ''} #${idx + 1}`}
            showQr={!isPostQuantumLink(link)}
          />
        ),
      });
    });
    wgConfigs.forEach(({ inbound, text }) => {
      const meta = formatTunnelConfigMeta(inbound, client?.email, wgConfigs.length);
      const label = (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <Tag color="cyan" style={{ margin: 0 }}>
            {t('pages.clients.wireguardConfig')}
          </Tag>
          {meta.label && <span style={{ opacity: 0.85, fontSize: 12 }}>{meta.label}</span>}
        </span>
      );
      out.push({
        key: `wg-config-${inbound.id}`,
        label,
        children: <QrPanel value={text} remark={meta.qrRemark} downloadName={meta.fileName} />,
      });
    });
    awgConfigs.forEach(({ inbound, text }) => {
      const meta = formatTunnelConfigMeta(inbound, client?.email, awgConfigs.length);
      const label = (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <Tag color="purple" style={{ margin: 0 }}>
            {t('pages.clients.amneziaWgConfig')}
          </Tag>
          {meta.label && <span style={{ opacity: 0.85, fontSize: 12 }}>{meta.label}</span>}
        </span>
      );
      out.push({
        key: `awg-config-${inbound.id}`,
        label,
        children: <QrPanel value={text} remark={meta.qrRemark} downloadName={meta.fileName} />,
      });
    });
    if (tuicConfigText) {
      out.push({
        key: 'tuic-config',
        label: (
          <Tag color="orange" style={{ margin: 0 }}>
            {t('pages.clients.tuicConfig')}
          </Tag>
        ),
        children: (
          <QrPanel
            value={tuicConfigText}
            remark={client?.email || 'tuic'}
            downloadName={`${client?.email || 'tuic'}.yaml`}
          />
        ),
      });
    }
    return out;
  }, [subLink, subJsonLink, wgConfigs, awgConfigs, tuicConfigText, links, client?.email, t]);

  // Expanding the first panel is a render-time adjustment, not a side effect.
  const firstKey = open && items.length > 0 ? items[0].key : null;
  const [syncedFirstKey, setSyncedFirstKey] = useState<string | null>(null);
  if (firstKey !== syncedFirstKey) {
    setSyncedFirstKey(firstKey);
    setActiveKey(firstKey ? [firstKey] : []);
  }

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
        {!client?.subId && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>
            {t('pages.clients.noSubId')}
          </div>
        )}
        {client?.subId && !hasAnything && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>
            {t('pages.clients.noLinks')}
          </div>
        )}
        {hasAnything && (
          <Collapse
            activeKey={activeKey}
            onChange={(keys) =>
              setActiveKey(typeof keys === 'string' ? [keys] : (keys as string[]))
            }
            items={items}
          />
        )}
      </Spin>
    </Modal>
  );
}
