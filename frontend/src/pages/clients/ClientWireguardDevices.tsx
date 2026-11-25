import { useState } from 'react';
import { Alert, Button, Divider, Dropdown, Modal, Popover, Tag, Tooltip, message } from 'antd';
import {
  CheckOutlined,
  CopyOutlined,
  DownloadOutlined,
  MoreOutlined,
  PlusOutlined,
  QrcodeOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { ClipboardManager, FileManager } from '@/utils';
import { formatTunnelConfigMeta } from '@/lib/inbounds/label';
import { QrPanel } from '@/pages/inbounds/qr';
import PromptModal from '@/components/feedback/PromptModal';
import type { useClientWireguardDevices } from '@/hooks/useClientWireguardDevices';
import type { buildWireguardDeviceConfigs } from './wireguardConfig';

type DevicesState = ReturnType<typeof useClientWireguardDevices>;

export default function ClientWireguardDevices({
  state,
  subLink,
  clientEmail,
  configs,
}: {
  state: DevicesState;
  subLink: string;
  clientEmail: string;
  configs: ReturnType<typeof buildWireguardDeviceConfigs>;
}) {
  const { t } = useTranslation();
  const [modal, modalContext] = Modal.useModal();
  const [messageApi, messageContext] = message.useMessage();
  const [prompt, setPrompt] = useState<{ id: string; name: string } | null>(null);
  const deviceCount = state.data?.inbounds[0]?.devices.length ?? 0;

  async function copy(text: string) {
    if (await ClipboardManager.copyText(text)) messageApi.success(t('copied'));
  }

  function copySubscription(id: string) {
    const url = new URL(subLink, window.location.href);
    if (id !== 'primary') url.searchParams.set('wgDevice', id);
    void copy(url.toString());
  }

  return (
    <>
      {modalContext}
      {messageContext}
      <Divider>{t('pages.clients.wgDevices.title')}</Divider>
      {configs.map((config) => {
        const { inbound, deviceId, deviceName, text, enabled } = config;
        const name = deviceName || t('pages.clients.wgDevices.primary');
        const meta = formatTunnelConfigMeta(inbound, clientEmail, configs.length, config.endpoint);
        const label =
          meta.label || [inbound.remark || inbound.tag, name].filter(Boolean).join(' · ');
        const device = state.data?.inbounds
          .find((entry) => entry.inboundId === inbound.id)
          ?.devices.find((entry) => entry.id === deviceId);
        const items = [
          ...(deviceId !== 'primary'
            ? [
                { key: 'rename', label: t('edit') },
                {
                  key: 'enable',
                  label: t('pages.clients.wgDevices.enabled'),
                  icon: device?.enable ? <CheckOutlined aria-hidden /> : null,
                },
              ]
            : []),
          ...(subLink
            ? [{ key: 'subscription', label: t('pages.clients.wgDevices.subscription') }]
            : []),
          ...(deviceId !== 'primary'
            ? [{ key: 'delete', label: t('pages.clients.wgDevices.remove'), danger: true }]
            : []),
        ];
        return (
          <div className="link-row" key={`${inbound.id}-${deviceId}-${config.endpoint}`}>
            <Tooltip title={enabled ? 'WireGuard' : t('disabled')}>
              <Tag color={enabled ? 'cyan' : 'default'} className="link-row-tag">
                WG
              </Tag>
            </Tooltip>
            <span
              className="link-row-title"
              title={`${label} — ${device?.allowedIPs.join(', ') ?? ''}`}
            >
              {label}
            </span>
            <div className="link-row-actions">
              <Tooltip title={t('copy')}>
                <Button
                  size="small"
                  aria-label={`${name} ${t('copy')}`}
                  icon={<CopyOutlined aria-hidden />}
                  disabled={!enabled}
                  onClick={() => void copy(text)}
                />
              </Tooltip>
              <Tooltip title={t('download')}>
                <Button
                  size="small"
                  aria-label={`${name} ${t('download')}`}
                  icon={<DownloadOutlined aria-hidden />}
                  disabled={!enabled}
                  onClick={() =>
                    FileManager.downloadTextFile(
                      text,
                      deviceId === 'primary'
                        ? meta.fileName
                        : meta.fileName.replace(/\.conf$/, `-${deviceId}.conf`),
                    )
                  }
                />
              </Tooltip>
              <Popover
                trigger="click"
                placement="left"
                destroyOnHidden
                content={<QrPanel value={text} remark={meta.qrRemark || label} size={220} />}
              >
                <Tooltip title={t('pages.clients.qrCode')}>
                  <Button
                    size="small"
                    aria-label={`${name} ${t('pages.clients.qrCode')}`}
                    icon={<QrcodeOutlined aria-hidden />}
                    disabled={!enabled}
                  />
                </Tooltip>
              </Popover>
              {items.length > 0 && (
                <Dropdown
                  trigger={['click']}
                  menu={{
                    items,
                    onClick: ({ key }) => {
                      if (key === 'rename') setPrompt({ id: deviceId, name: deviceName });
                      if (key === 'enable') state.update(deviceId, { enable: !device?.enable });
                      if (key === 'subscription') copySubscription(deviceId);
                      if (key === 'delete')
                        modal.confirm({
                          title: t('pages.clients.wgDevices.deleteConfirm'),
                          onOk: () => state.remove(deviceId),
                        });
                    },
                  }}
                >
                  <Button
                    size="small"
                    aria-label={`${name} ${t('more')}`}
                    icon={<MoreOutlined aria-hidden />}
                    disabled={state.pending}
                  />
                </Dropdown>
              )}
            </div>
          </div>
        );
      })}
      <Button
        size="small"
        icon={<PlusOutlined aria-hidden />}
        loading={state.pending}
        disabled={!state.data || deviceCount >= state.data.limit}
        onClick={() => setPrompt({ id: '', name: '' })}
      >
        {t('pages.clients.wgDevices.add')}
      </Button>
      {state.error && <Alert type="error" title={state.error.message} />}
      {state.mutationError && <Alert type="error" title={state.mutationError.message} />}
      <PromptModal
        open={prompt !== null}
        title={t('pages.clients.wgDevices.name')}
        initialValue={prompt?.name ?? ''}
        loading={state.pending}
        onClose={() => setPrompt(null)}
        onConfirm={(value) => {
          const name = value.trim();
          if (!name || !prompt) return;
          if (prompt.id) state.update(prompt.id, { name });
          else state.create(name);
          setPrompt(null);
        }}
      />
    </>
  );
}
