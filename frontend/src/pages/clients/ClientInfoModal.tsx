import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Tag, Tooltip, message } from 'antd';
import { CopyOutlined } from '@ant-design/icons';

import { ClipboardManager, HttpUtil, IntlUtil, SizeFormatter } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import './ClientInfoModal.css';

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
}

interface ClientInfoModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById: Record<number, InboundOption>;
  isOnline: boolean;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = { enable: false, subURI: '', subJsonURI: '', subJsonEnable: false };

export default function ClientInfoModal({
  open,
  client,
  inboundsById,
  isOnline,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientInfoModalProps) {
  const { datepicker } = useDatepicker();
  const expiryLabel = (ts?: number) => (!ts || ts <= 0 ? '∞' : IntlUtil.formatDate(ts, datepicker));
  const dateLabel = (ts?: number) => (!ts || ts <= 0 ? '-' : IntlUtil.formatDate(ts, datepicker));
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [links, setLinks] = useState<string[]>([]);

  useEffect(() => {
    if (!open) {
      setLinks([]);
      return;
    }
    if (!client?.subId) return;
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.get(
        `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
      ) as ApiMsg<string[]>;
      if (cancelled) return;
      setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
    })();
    return () => { cancelled = true; };
  }, [open, client?.subId]);

  const traffic = client?.traffic || null;
  const totalBytes = client?.totalGB || 0;
  const used = (traffic?.up || 0) + (traffic?.down || 0);
  const remaining = useMemo(() => {
    if (totalBytes <= 0) return -1;
    const r = totalBytes - used;
    return r > 0 ? r : 0;
  }, [totalBytes, used]);

  const subLink = useMemo(() => {
    if (!client?.subId || !subSettings?.subURI) return '';
    return subSettings.subURI + client.subId;
  }, [client?.subId, subSettings?.subURI]);

  const subJsonLink = useMemo(() => {
    if (!client?.subId) return '';
    if (!subSettings?.subJsonEnable || !subSettings?.subJsonURI) return '';
    return subSettings.subJsonURI + client.subId;
  }, [client?.subId, subSettings?.subJsonEnable, subSettings?.subJsonURI]);

  const showSubscription = !!(subSettings?.enable && client?.subId);

  async function copyValue(text: string) {
    if (!text) return;
    const ok = await ClipboardManager.copyText(String(text));
    if (ok) messageApi.success(t('copied'));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={client ? client.email : t('info')}
        footer={null}
        width={640}
        onCancel={() => onOpenChange(false)}
    >
      {client && (
        <>
          <table className="info-table block">
            <tbody>
              <tr>
                <td>{t('pages.clients.online')}</td>
                <td>
                  {client.enable && isOnline
                    ? <Tag color="green">{t('pages.clients.online')}</Tag>
                    : <Tag>{t('pages.clients.offline')}</Tag>}
                  <span className="hint">{t('lastOnline')}: {dateLabel(traffic?.lastOnline)}</span>
                </td>
              </tr>
              <tr>
                <td>{t('status')}</td>
                <td>
                  <Tag color={client.enable ? 'green' : 'default'}>
                    {client.enable ? t('enabled') : t('disabled')}
                  </Tag>
                </td>
              </tr>
              <tr>
                <td>{t('pages.clients.email')}</td>
                <td>
                  {client.email
                    ? <Tag color="green">{client.email}</Tag>
                    : <Tag color="red">{t('none')}</Tag>}
                </td>
              </tr>
              <tr>
                <td>{t('pages.clients.subId')}</td>
                <td>
                  <Tag className="info-large-tag">{client.subId || '-'}</Tag>
                  {client.subId && (
                    <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyValue(client.subId!)} />
                  )}
                </td>
              </tr>
              {client.uuid && (
                <tr>
                  <td>{t('pages.clients.uuid')}</td>
                  <td>
                    <Tag className="info-large-tag">{client.uuid}</Tag>
                    <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyValue(client.uuid!)} />
                  </td>
                </tr>
              )}
              {client.password && (
                <tr>
                  <td>{t('password')}</td>
                  <td>
                    <Tag className="info-large-tag">{client.password}</Tag>
                    <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyValue(client.password!)} />
                  </td>
                </tr>
              )}
              {client.auth && (
                <tr>
                  <td>{t('pages.clients.auth')}</td>
                  <td>
                    <Tag className="info-large-tag">{client.auth}</Tag>
                    <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => copyValue(client.auth!)} />
                  </td>
                </tr>
              )}
              <tr>
                <td>{t('pages.clients.flow')}</td>
                <td>
                  {client.flow ? <Tag>{client.flow}</Tag> : <Tag color="orange">{t('none')}</Tag>}
                </td>
              </tr>
              <tr>
                <td>{t('pages.inbounds.traffic')}</td>
                <td>
                  <Tag>
                    ↑ {SizeFormatter.sizeFormat(traffic?.up || 0)}
                    {' '}/ ↓ {SizeFormatter.sizeFormat(traffic?.down || 0)}
                  </Tag>
                  <span className="hint">
                    {SizeFormatter.sizeFormat(used)} / {totalBytes > 0 ? SizeFormatter.sizeFormat(totalBytes) : '∞'}
                  </span>
                </td>
              </tr>
              <tr>
                <td>{t('remained')}</td>
                <td>
                  {remaining < 0
                    ? <Tag color="purple">∞</Tag>
                    : <Tag color={remaining > 0 ? '' : 'red'}>{SizeFormatter.sizeFormat(remaining)}</Tag>}
                </td>
              </tr>
              <tr>
                <td>{t('pages.inbounds.expireDate')}</td>
                <td>
                  {!client.expiryTime || client.expiryTime <= 0
                    ? <Tag color="purple">∞</Tag>
                    : <Tag>{expiryLabel(client.expiryTime)}</Tag>}
                  {(client.expiryTime ?? 0) > 0 && (
                    <span className="hint">{IntlUtil.formatRelativeTime(client.expiryTime)}</span>
                  )}
                </td>
              </tr>
              <tr>
                <td>{t('pages.clients.ipLimit')}</td>
                <td>{!client.limitIp ? <Tag>∞</Tag> : <Tag>{client.limitIp}</Tag>}</td>
              </tr>
              <tr>
                <td>{t('pages.inbounds.createdAt')}</td>
                <td><Tag>{dateLabel(client.createdAt)}</Tag></td>
              </tr>
              <tr>
                <td>{t('pages.inbounds.updatedAt')}</td>
                <td><Tag>{dateLabel(client.updatedAt)}</Tag></td>
              </tr>
              {client.comment && (
                <tr>
                  <td>{t('pages.clients.comment')}</td>
                  <td><Tag className="info-large-tag">{client.comment}</Tag></td>
                </tr>
              )}
              <tr>
                <td>{t('pages.clients.attachedInbounds')}</td>
                <td>
                  <div className="chips">
                    {(client.inboundIds || []).map((id) => {
                      const ib = inboundsById[id];
                      return (
                        <Tag key={id} color="blue">
                          {ib ? `${ib.remark || `#${id}`} (${ib.protocol}:${ib.port})` : `#${id}`}
                        </Tag>
                      );
                    })}
                    {(!client.inboundIds || client.inboundIds.length === 0) && (
                      <span className="hint">—</span>
                    )}
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          {links.length > 0 && (
            <>
              <Divider>{t('pages.inbounds.copyLink')}</Divider>
              {links.map((link, idx) => (
                <div key={idx} className="link-panel">
                  <div className="link-panel-header">
                    <Tag color="green">{`${t('pages.clients.link')} ${idx + 1}`}</Tag>
                    <Tooltip title={t('copy')}>
                      <Button size="small" icon={<CopyOutlined />} onClick={() => copyValue(link)} />
                    </Tooltip>
                  </div>
                  <code className="link-panel-text">{link}</code>
                </div>
              ))}
            </>
          )}

          {showSubscription && subLink && (
            <>
              <Divider>{t('subscription.title')}</Divider>
              <div className="link-panel">
                <div className="link-panel-header">
                  <Tag color="green">{t('subscription.title')}</Tag>
                  <Tooltip title={t('copy')}>
                    <Button size="small" icon={<CopyOutlined />} onClick={() => copyValue(subLink)} />
                  </Tooltip>
                </div>
                <a href={subLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">{subLink}</a>
              </div>
              {subJsonLink && (
                <div className="link-panel">
                  <div className="link-panel-header">
                    <Tag color="green">JSON</Tag>
                    <Tooltip title={t('copy')}>
                      <Button size="small" icon={<CopyOutlined />} onClick={() => copyValue(subJsonLink)} />
                    </Tooltip>
                  </div>
                  <a href={subJsonLink} target="_blank" rel="noopener noreferrer" className="link-panel-anchor">{subJsonLink}</a>
                </div>
              )}
            </>
          )}
        </>
      )}
      </Modal>
    </>
  );
}
