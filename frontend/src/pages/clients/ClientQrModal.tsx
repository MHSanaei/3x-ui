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
import { buildTuicClientConfig, findTuicInbound, isTuicClient } from './tuicConfig';
import { tunnelConfigEndpoints, tunnelEndpointLabel } from './tunnelEndpoints';
import type { HostRecord } from '@/schemas/api/host';

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
  hosts?: HostRecord[];
  onOpenChange: (open: boolean) => void;
}

const NO_HOSTS: HostRecord[] = [];

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

type QrVariant = 'standard' | 'happ';
type HappError = 'too_long' | 'unavailable' | null;

const HAPP_CRYPT5_PREFIX = 'happ://crypt5/';
const HAPP_SETTINGS_PATH = '/settings?subscriptionTab=happ#subscription';
// QrPanel encodes at error level L; QR version 40 holds 2953 UTF-8 bytes at that level.
const HAPP_QR_MAX_BYTES = 2953;
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
  happError: HappError;
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
                  title={
                    happError === 'too_long'
                      ? t('pages.clients.happLinkSourceTooLong')
                      : t('pages.clients.happLinkErrorHint', {
                          dashboard: t('menu.dashboard'),
                          logs: t('pages.index.logs'),
                        })
                  }
                />
              ) : null}
            </div>
          </Spin>
          {happLink || happError === 'unavailable' ? (
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
  // A gate or source change remounts this scope to clear Happ state and retire any in-flight response.
  const scopeKey = `${props.client?.id ?? ''}\0${subId}\0${subLink}\0${happLinkEnabled ? 1 : 0}`;

  return <ClientQrModalContent key={scopeKey} {...props} />;
}

function ClientQrModalContent({
  open,
  client,
  inboundsById,
  tunnelAllowedIPs,
  subSettings = DEFAULT_SUB,
  hosts = NO_HOSTS,
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
  const [happError, setHappError] = useState<HappError>(null);
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
          // Only this fixed API code is safe to localize; arbitrary error messages stay hidden.
          setHappError(
            msg?.success === false && msg.msg === 'happ_source_too_long'
              ? 'too_long'
              : 'unavailable',
          );
        }
      } catch {
        if (!cancelled) setHappError('unavailable');
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
      setHappError(generateHapp && !canGenerateHapp ? 'unavailable' : null);
    },
    [canGenerateHapp, happLinkEnabled],
  );

  const regenerateHappLink = useCallback(() => {
    setHappLink('');
    setHappLoading(canGenerateHapp);
    setHappError(canGenerateHapp ? null : 'unavailable');
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
    const host = window.location.hostname;
    const publicHost = subSettings.publicHost ?? '';
    return wgInbounds
      .flatMap((ib) => {
        const address = tunnelAllowedIPs?.[ib.id] ?? '';
        return tunnelConfigEndpoints(ib, hosts, host, publicHost).map((ep) => ({
          inbound: ib,
          endpoint: tunnelEndpointLabel(ep),
          text: buildWireguardClientConfig(client, ib, host, publicHost, address, ep),
        }));
      })
      .filter((c) => !!c.text);
  }, [client, wgInbounds, tunnelAllowedIPs, subSettings.publicHost, hosts]);

  const awgInbounds = useMemo(
    () => findAmneziaWGInbounds(client, inboundsById),
    [client, inboundsById],
  );
  const awgConfigs = useMemo(() => {
    if (!client || !isAmneziaWGClient(client)) return [];
    const host = window.location.hostname;
    const publicHost = subSettings.publicHost ?? '';
    return awgInbounds
      .flatMap((ib) => {
        const address = tunnelAllowedIPs?.[ib.id] ?? '';
        return tunnelConfigEndpoints(ib, hosts, host, publicHost).map((ep) => ({
          inbound: ib,
          endpoint: tunnelEndpointLabel(ep),
          text: buildAmneziaWGClientConfig(client, ib, host, publicHost, address, ep),
        }));
      })
      .filter((c) => !!c.text);
  }, [client, awgInbounds, tunnelAllowedIPs, subSettings.publicHost, hosts]);

  const tuicInbound = useMemo(() => findTuicInbound(client, inboundsById), [client, inboundsById]);
  const tuicConfigs = useMemo(() => {
    if (!client || !tuicInbound || !isTuicClient(client)) return [];
    const host = window.location.hostname;
    const publicHost = subSettings.publicHost ?? '';
    return tunnelConfigEndpoints(tuicInbound, hosts, host, publicHost, 'clash').map((ep) => ({
      endpoint: tunnelEndpointLabel(ep),
      text: buildTuicClientConfig(client, tuicInbound, host, publicHost, ep),
    }));
  }, [client, tuicInbound, subSettings.publicHost, hosts]);

  const hasAnything =
    !!subLink ||
    !!subJsonLink ||
    wgConfigs.length > 0 ||
    awgConfigs.length > 0 ||
    tuicConfigs.length > 0 ||
    links.length > 0;

  // The reset runs during render so the effect only carries the request.
  const openSubId = open ? (client?.subId ?? '') : '';
  const [syncedSubId, setSyncedSubId] = useState(openSubId);
  if (openSubId !== syncedSubId) {
    setSyncedSubId(openSubId);
    setLinks([]);
    setLoading(!!openSubId);
    setVariant('standard');
    setHappLink('');
    setHappLoading(false);
    setHappError(null);
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
    wgConfigs.forEach(({ inbound, endpoint, text }) => {
      const meta = formatTunnelConfigMeta(inbound, client?.email, wgConfigs.length, endpoint);
      const label = (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <Tag color="cyan" style={{ margin: 0 }}>
            {t('pages.clients.wireguardConfig')}
          </Tag>
          {meta.label && <span style={{ opacity: 0.85, fontSize: 12 }}>{meta.label}</span>}
        </span>
      );
      out.push({
        key: `wg-config-${inbound.id}-${endpoint}`,
        label,
        children: <QrPanel value={text} remark={meta.qrRemark} downloadName={meta.fileName} />,
      });
    });
    awgConfigs.forEach(({ inbound, endpoint, text }) => {
      const meta = formatTunnelConfigMeta(inbound, client?.email, awgConfigs.length, endpoint);
      const label = (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <Tag color="purple" style={{ margin: 0 }}>
            {t('pages.clients.amneziaWgConfig')}
          </Tag>
          {meta.label && <span style={{ opacity: 0.85, fontSize: 12 }}>{meta.label}</span>}
        </span>
      );
      out.push({
        key: `awg-config-${inbound.id}-${endpoint}`,
        label,
        children: <QrPanel value={text} remark={meta.qrRemark} downloadName={meta.fileName} />,
      });
    });
    tuicConfigs.forEach(({ endpoint, text }) => {
      const name = client?.email || 'tuic';
      const multi = tuicConfigs.length > 1 ? endpoint : '';
      out.push({
        key: `tuic-config-${endpoint}`,
        label: (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <Tag color="orange" style={{ margin: 0 }}>
              {t('pages.clients.tuicConfig')}
            </Tag>
            {multi && <span style={{ opacity: 0.85, fontSize: 12 }}>{multi}</span>}
          </span>
        ),
        children: (
          <QrPanel
            value={text}
            remark={[name, multi].filter(Boolean).join(' - ')}
            downloadName={`${[name, multi.replace(/[^\w.-]+/g, '_')].filter(Boolean).join('-')}.yaml`}
          />
        ),
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
    tuicConfigs,
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
