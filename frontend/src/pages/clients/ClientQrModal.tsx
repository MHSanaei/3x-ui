import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal, Spin } from 'antd';
import { HttpUtil } from '@/utils';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord } from '@/hooks/useClients';

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

interface ClientQrModalProps {
  open: boolean;
  client: ClientRecord | null;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = { enable: false, subURI: '', subJsonURI: '', subJsonEnable: false };

export default function ClientQrModal({
  open,
  client,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientQrModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const subLink = useMemo(() => {
    if (!client?.subId || !subSettings?.enable || !subSettings?.subURI) return '';
    return subSettings.subURI + client.subId;
  }, [client?.subId, subSettings?.enable, subSettings?.subURI]);

  const subJsonLink = useMemo(() => {
    if (!client?.subId || !subSettings?.enable) return '';
    if (!subSettings?.subJsonEnable || !subSettings?.subJsonURI) return '';
    return subSettings.subJsonURI + client.subId;
  }, [client?.subId, subSettings?.enable, subSettings?.subJsonEnable, subSettings?.subJsonURI]);

  const hasAnything = !!subLink || !!subJsonLink || links.length > 0;

  useEffect(() => {
    if (!open || !client?.subId) {
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
  }, [open, client?.subId]);

  const [activeKey, setActiveKey] = useState<string[]>([]);

  const items = useMemo(() => {
    const out: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [];
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
      const parts = parseLinkParts(link, client?.email ?? '');
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
  }, [subLink, subJsonLink, links, client?.email, t]);

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
        {!client?.subId && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>{t('pages.clients.noSubId')}</div>
        )}
        {client?.subId && !hasAnything && !loading && (
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
