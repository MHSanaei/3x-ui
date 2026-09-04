import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { Alert, Button, Collapse, Empty, Modal, Segmented, Spin, Tag, Typography } from 'antd';
import { LockOutlined } from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import type { HappLinkResult } from '@/generated/types';
import { HappLinkResultSchema } from '@/generated/zod';
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

interface SubSettings {
  enable: boolean;
  happLinkEnable?: boolean;
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
const HAPP_SETTINGS_PATH = '/settings?subscriptionTab=happ#subscription';
// antd's level-M QR encoder tops out at 2331 UTF-8 bytes in byte mode.
const HAPP_QR_MAX_BYTES = 2331;
const UTF8_ENCODER = new TextEncoder();

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

function canRenderHappQr(link: string) {
  return UTF8_ENCODER.encode(link).byteLength <= HAPP_QR_MAX_BYTES;
}

interface SubscriptionQrPresentationProps {
  variant: QrVariant;
  standardLink: string;
  remark: string;
  happLink: string;
  happLoading: boolean;
  happError: boolean;
  happLinkEnabled: boolean;
  onVariantChange: (variant: QrVariant) => void;
  onRegenerate: () => void;
  onOpenHappSettings: () => void;
}

function SubscriptionQrPresentation({
  variant,
  standardLink,
  remark,
  happLink,
  happLoading,
  happError,
  happLinkEnabled,
  onVariantChange,
  onRegenerate,
  onOpenHappSettings,
}: SubscriptionQrPresentationProps) {
  const { t } = useTranslation();
  const showHappQr = canRenderHappQr(happLink);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <Segmented<QrVariant>
        block
        value={variant}
        options={[
          { label: t('pages.clients.qrStandard'), value: 'standard' },
          {
            label: (
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                {!happLinkEnabled ? (
                  <LockOutlined aria-label={t('pages.clients.happLinkDisabledHint')} />
                ) : null}
                <span>{t('pages.clients.happLinkOptionLabel')}</span>
              </span>
            ),
            value: 'happ',
          },
        ]}
        onChange={onVariantChange}
      />
      {variant === 'standard' ? (
        <QrPanel value={standardLink} remark={remark} />
      ) : !happLinkEnabled ? (
        <Empty
          image={<LockOutlined aria-hidden style={{ fontSize: 40, opacity: 0.45 }} />}
          styles={{ image: { height: 44, marginBottom: 12 } }}
          style={{
            minHeight: 190,
            margin: 0,
            padding: '20px 12px',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
          }}
          description={
            <div style={{ maxWidth: 400, margin: '0 auto' }}>
              <Typography.Text strong>{t('pages.clients.happLinkDisabledTitle')}</Typography.Text>
              <Typography.Paragraph type="secondary" style={{ margin: '6px 0 0' }}>
                {t('pages.clients.happLinkDisabledDescription')}
              </Typography.Paragraph>
            </div>
          }
        >
          <Button type="primary" onClick={onOpenHappSettings}>
            {t('pages.clients.happLinkSettingsAction')}
          </Button>
        </Empty>
      ) : (
        <div>
          <Alert
            style={{ marginBottom: 16 }}
            type="warning"
            showIcon
            title={t('pages.clients.happLinkDisclosure')}
          />
          <Spin spinning={happLoading}>
            <div style={{ minHeight: happLoading ? 48 : undefined }}>
              {happLink ? (
                <>
                  {!showHappQr ? (
                    <Alert
                      style={{ marginBottom: 12 }}
                      type="info"
                      showIcon
                      title={t('pages.clients.happLinkQrTooLong')}
                    />
                  ) : null}
                  <QrPanel value={happLink} remark={remark} showQr={showHappQr} />
                </>
              ) : null}
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
  happLinkEnable: false,
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
  const happLinkEnabled = subSettings.happLinkEnable === true;
  // A gate change remounts this scope to clear Happ state and retire any in-flight response.
  const scopeKey = `${props.open ? 1 : 0}\0${props.client?.id ?? ''}\0${subId}\0${subLink}\0${happLinkEnabled ? 1 : 0}`;

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
  const navigate = useNavigate();
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
  const happLinkEnabled = subSettings.happLinkEnable === true;
  const [variant, setVariant] = useState<QrVariant>('standard');
  const [happAttempt, setHappAttempt] = useState(0);
  const [happLink, setHappLink] = useState('');
  const [happLoading, setHappLoading] = useState(false);
  const [happError, setHappError] = useState(false);
  const canGenerateHapp =
    happLinkEnabled &&
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
      const generateHapp = nextVariant === 'happ' && happLinkEnabled;
      setVariant(nextVariant);
      setHappLink('');
      setHappLoading(generateHapp && canGenerateHapp);
      setHappError(generateHapp && !canGenerateHapp);
    },
    [canGenerateHapp, happLinkEnabled],
  );

  const regenerateHappLink = useCallback(() => {
    setHappLink('');
    setHappLoading(canGenerateHapp);
    setHappError(!canGenerateHapp);
    if (!canGenerateHapp) return;
    setHappAttempt((attempt) => attempt + 1);
  }, [canGenerateHapp]);

  const openHappSettings = useCallback(() => {
    // This path only exposes the operator gate; authorization and saving remain explicit in Settings.
    onOpenChange(false);
    navigate(HAPP_SETTINGS_PATH);
  }, [navigate, onOpenChange]);

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
          subSettings.publicHost ?? '',
          address,
        );
        return { inbound: ib, text };
      })
      .filter((c) => !!c.text);
  }, [client, wgInbounds, tunnelAllowedIPs, subSettings.publicHost]);

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
          subSettings.publicHost ?? '',
          address,
        );
        return { inbound: ib, text };
      })
      .filter((c) => !!c.text);
  }, [client, awgInbounds, tunnelAllowedIPs, subSettings.publicHost]);

  const hasAnything =
    !!subLink || !!subJsonLink || wgConfigs.length > 0 || awgConfigs.length > 0 || links.length > 0;

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
            happLinkEnabled={happLinkEnabled}
            onVariantChange={selectVariant}
            onRegenerate={regenerateHappLink}
            onOpenHappSettings={openHappSettings}
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
    return out;
  }, [
    subLink,
    subJsonLink,
    variant,
    happLink,
    happLoading,
    happError,
    happLinkEnabled,
    wgConfigs,
    awgConfigs,
    links,
    client?.email,
    selectVariant,
    regenerateHappLink,
    openHappSettings,
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
