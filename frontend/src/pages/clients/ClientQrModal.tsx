import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Collapse, Modal, Segmented, Spin, Tag } from 'antd';
import { HttpUtil } from '@/utils';
import type { HappLinkResult } from '@/generated/types';
import { HappLinkResultSchema } from '@/generated/zod';
import { isPostQuantumLink } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import {
  buildWireguardClientConfig,
  findWireguardInbound,
  isWireguardClient,
} from './wireguardConfig';
import {
  buildAmneziaWGClientConfig,
  findAmneziaWGInbound,
  isAmneziaWGClient,
} from './amneziawgConfig';

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

type QrVariant = 'standard' | 'happ';

const HAPP_CRYPT5_PREFIX = 'happ://crypt5/';

function hasHappForbiddenCharacter(link: string) {
  return Array.from(link).some((character) => {
    const codePoint = character.codePointAt(0) ?? 0;
    return /\s/u.test(character) || codePoint <= 0x1f || (codePoint >= 0x7f && codePoint <= 0x9f);
  });
}

function isValidHappCrypt5Link(link: string) {
  return (
    link.startsWith(HAPP_CRYPT5_PREFIX) &&
    link.length > HAPP_CRYPT5_PREFIX.length &&
    !hasHappForbiddenCharacter(link)
  );
}

interface SubscriptionQrPresentationProps {
  variant: QrVariant;
  standardLink: string;
  remark: string;
  happLink: string;
  happLoading: boolean;
  happError: boolean;
  onVariantChange: (variant: QrVariant) => void;
  onRegenerate: () => void;
}

function SubscriptionQrPresentation({
  variant,
  standardLink,
  remark,
  happLink,
  happLoading,
  happError,
  onVariantChange,
  onRegenerate,
}: SubscriptionQrPresentationProps) {
  const { t } = useTranslation();

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Segmented<QrVariant>
        block
        value={variant}
        options={[
          { label: t('pages.clients.qrStandard'), value: 'standard' },
          { label: 'Happ', value: 'happ' },
        ]}
        onChange={onVariantChange}
      />
      {variant === 'standard' ? (
        <QrPanel value={standardLink} remark={remark} />
      ) : (
        <div>
          <Spin spinning={happLoading}>
            <div style={{ minHeight: happLoading ? 48 : undefined }}>
              {happLink ? <QrPanel value={happLink} remark={remark} /> : null}
              {happError ? (
                <Alert
                  type="error"
                  showIcon
                  title={t('pages.clients.happLinkErrorHint', {
                    dashboard: t('menu.dashboard'),
                    logs: t('pages.index.logs'),
                  })}
                />
              ) : null}
            </div>
          </Spin>
          {happLink || happError ? (
            <Button style={{ marginTop: 12 }} onClick={onRegenerate}>
              {happError ? t('pages.clients.happLinkRetry') : t('regenerate')}
            </Button>
          ) : null}
        </div>
      )}
    </div>
  );
}

const DEFAULT_SUB: SubSettings = {
  enable: false,
  subURI: '',
  subJsonURI: '',
  subJsonEnable: false,
  publicHost: '',
};

export default function ClientQrModal(props: ClientQrModalProps) {
  const subSettings = props.subSettings ?? DEFAULT_SUB;
  const subId = props.client?.subId ?? '';
  const subLink =
    subId && subSettings.enable && subSettings.subURI ? subSettings.subURI + subId : '';
  const scopeKey = `${props.open ? 1 : 0}\0${props.client?.id ?? ''}\0${subId}\0${subLink}`;

  return <ClientQrModalContent key={scopeKey} {...props} />;
}

function ClientQrModalContent({
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
  const clientId = client?.id;
  const clientSubId = subId ?? '';
  const [variant, setVariant] = useState<QrVariant>('standard');
  const [happAttempt, setHappAttempt] = useState(0);
  const [happLink, setHappLink] = useState('');
  const [happLoading, setHappLoading] = useState(false);
  const [happError, setHappError] = useState(false);
  const canGenerateHapp =
    typeof clientId === 'number' &&
    Number.isSafeInteger(clientId) &&
    clientId > 0 &&
    !!clientSubId &&
    !!subLink;

  useEffect(() => {
    if (!open || variant !== 'happ' || !canGenerateHapp) return;

    let cancelled = false;

    (async () => {
      try {
        const msg = await HttpUtil.post<HappLinkResult>(
          `/panel/api/clients/happLink/${clientId}`,
          undefined,
          { silent: true },
        );
        if (cancelled) return;

        const result = HappLinkResultSchema.safeParse(msg?.obj);
        if (msg?.success && result.success && isValidHappCrypt5Link(result.data.encryptedLink)) {
          setHappLink(result.data.encryptedLink);
        } else {
          setHappError(true);
        }
      } catch {
        if (!cancelled) setHappError(true);
      } finally {
        if (!cancelled) setHappLoading(false);
      }
    })();

    return () => {
      // A retired generation must never replace the QR for a newer modal scope.
      cancelled = true;
    };
  }, [open, variant, clientId, clientSubId, subLink, happAttempt, canGenerateHapp]);

  const selectVariant = useCallback(
    (nextVariant: QrVariant) => {
      const generateHapp = nextVariant === 'happ';
      setVariant(nextVariant);
      setHappLink('');
      setHappLoading(generateHapp && canGenerateHapp);
      setHappError(generateHapp && !canGenerateHapp);
    },
    [canGenerateHapp],
  );

  const regenerateHappLink = useCallback(() => {
    setHappLink('');
    setHappLoading(canGenerateHapp);
    setHappError(!canGenerateHapp);
    if (!canGenerateHapp) return;
    setHappAttempt((attempt) => attempt + 1);
  }, [canGenerateHapp]);

  const wgInbound = useMemo(
    () => findWireguardInbound(client, inboundsById),
    [client, inboundsById],
  );
  const wgConfigText = useMemo(() => {
    if (!client || !wgInbound || !isWireguardClient(client)) return '';
    return buildWireguardClientConfig(
      client,
      wgInbound,
      window.location.hostname,
      subSettings?.publicHost ?? '',
    );
  }, [client, wgInbound, subSettings?.publicHost]);

  const awgInbound = useMemo(
    () => findAmneziaWGInbound(client, inboundsById),
    [client, inboundsById],
  );
  const awgConfigText = useMemo(() => {
    if (!client || !awgInbound || !isAmneziaWGClient(client)) return '';
    const address = awgInbound ? (tunnelAllowedIPs?.[awgInbound.id] ?? '') : '';
    return buildAmneziaWGClientConfig(
      client,
      awgInbound,
      window.location.hostname,
      subSettings?.publicHost ?? '',
      address,
    );
  }, [client, awgInbound, tunnelAllowedIPs, subSettings?.publicHost]);

  const hasAnything =
    !!subLink || !!subJsonLink || !!wgConfigText || !!awgConfigText || links.length > 0;

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
          <SubscriptionQrPresentation
            variant={variant}
            standardLink={subLink}
            remark={`${client?.email || ''} — ${t('subscription.title')}`}
            happLink={happLink}
            happLoading={happLoading}
            happError={happError}
            onVariantChange={selectVariant}
            onRegenerate={regenerateHappLink}
          />
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
    if (wgConfigText) {
      out.push({
        key: 'wg-config',
        label: (
          <Tag color="cyan" style={{ margin: 0 }}>
            {t('pages.clients.wireguardConfig')}
          </Tag>
        ),
        children: (
          <QrPanel
            value={wgConfigText}
            remark={client?.email || 'peer'}
            downloadName={`${client?.email || 'peer'}.conf`}
          />
        ),
      });
    }
    if (awgConfigText) {
      out.push({
        key: 'awg-config',
        label: (
          <Tag color="purple" style={{ margin: 0 }}>
            {t('pages.clients.amneziaWgConfig')}
          </Tag>
        ),
        children: (
          <QrPanel
            value={awgConfigText}
            remark={client?.email || 'peer'}
            downloadName={`${client?.email || 'peer'}.conf`}
          />
        ),
      });
    }
    return out;
  }, [
    subLink,
    subJsonLink,
    variant,
    happLink,
    happLoading,
    happError,
    wgConfigText,
    awgConfigText,
    links,
    client?.email,
    selectVariant,
    regenerateHappLink,
    t,
  ]);

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
