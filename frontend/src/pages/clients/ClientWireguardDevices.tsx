import { useState } from 'react';
import {
  Alert,
  Button,
  Divider,
  Input,
  InputNumber,
  Popconfirm,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { CopyOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { ClipboardManager } from '@/utils';
import type { useClientWireguardDevices } from '@/hooks/useClientWireguardDevices';

type DevicesState = ReturnType<typeof useClientWireguardDevices>;

export default function ClientWireguardDevices({
  state,
  subLink,
  onConfig,
}: {
  state: DevicesState;
  subLink: string;
  onConfig: (id: string) => void;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  if (!state.data) return state.error ? <Alert type="error" title={state.error.message} /> : null;
  const devices = state.data.inbounds[0]?.devices ?? [];
  return (
    <>
      <Divider>{t('pages.clients.wgDevices.title')}</Divider>
      <Space vertical style={{ width: '100%' }} size="middle">
        <Space wrap>
          <Tag>
            {devices.length} / {state.data.limit}
          </Tag>
          <Typography.Text>{t('pages.clients.wgDevices.limit')}</Typography.Text>
          <InputNumber
            aria-label={t('pages.clients.wgDevices.limit')}
            min={Math.max(1, devices.length)}
            max={32}
            value={state.data.limit}
            disabled={state.pending}
            onChange={(value) => {
              if (value != null) state.setLimit(value);
            }}
          />
        </Space>
        {devices.map((device) => (
          <div
            key={device.id}
            style={{
              border: '1px solid var(--ant-color-border-secondary)',
              borderRadius: 8,
              padding: 12,
            }}
          >
            <Space vertical style={{ width: '100%' }}>
              <Space wrap>
                {device.id === 'primary' ? (
                  <Typography.Text strong>{t('pages.clients.wgDevices.primary')}</Typography.Text>
                ) : (
                  <Typography.Text
                    editable={{
                      onChange: (value) => {
                        if (value.trim()) state.update(device.id, { name: value.trim() });
                      },
                    }}
                  >
                    {device.name}
                  </Typography.Text>
                )}
                <Switch
                  aria-label={`${device.name} ${t('pages.clients.wgDevices.enabled')}`}
                  checked={device.enable}
                  disabled={state.pending || device.id === 'primary'}
                  onChange={(enable) => state.update(device.id, { enable })}
                />
              </Space>
              <Space wrap>
                {state.data?.inbounds.map((inbound) => {
                  const current = inbound.devices.find((entry) => entry.id === device.id);
                  return (
                    current && <Tag key={inbound.inboundId}>{current.allowedIPs.join(', ')}</Tag>
                  );
                })}
              </Space>
              <Space wrap>
                <Button size="small" disabled={!device.enable} onClick={() => onConfig(device.id)}>
                  {t('pages.clients.wireguardConfig')}
                </Button>
                {subLink && (
                  <Button
                    size="small"
                    icon={<CopyOutlined aria-hidden />}
                    onClick={() => {
                      const url = new URL(subLink, window.location.href);
                      if (device.id !== 'primary') url.searchParams.set('wgDevice', device.id);
                      void ClipboardManager.copyText(url.toString());
                    }}
                  >
                    {t('pages.clients.wgDevices.subscription')}
                  </Button>
                )}
                {device.id !== 'primary' && (
                  <Popconfirm
                    title={t('pages.clients.wgDevices.deleteConfirm')}
                    onConfirm={() => state.remove(device.id)}
                  >
                    <Button
                      size="small"
                      danger
                      icon={<DeleteOutlined aria-hidden />}
                      disabled={state.pending}
                    >
                      {t('pages.clients.wgDevices.remove')}
                    </Button>
                  </Popconfirm>
                )}
              </Space>
            </Space>
          </div>
        ))}
        <Space.Compact style={{ width: '100%' }}>
          <Input
            aria-label={t('pages.clients.wgDevices.name')}
            placeholder={t('pages.clients.wgDevices.name')}
            value={name}
            maxLength={64}
            disabled={state.pending || devices.length >= state.data.limit}
            onChange={(event) => setName(event.target.value)}
          />
          <Button
            icon={<PlusOutlined aria-hidden />}
            loading={state.pending}
            disabled={!name.trim() || devices.length >= state.data.limit}
            onClick={() => {
              state.create(name.trim());
              setName('');
            }}
          >
            {t('pages.clients.wgDevices.add')}
          </Button>
        </Space.Compact>
        {state.mutationError && <Alert type="error" title={state.mutationError.message} />}
      </Space>
    </>
  );
}
