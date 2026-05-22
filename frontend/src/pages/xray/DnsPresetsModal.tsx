import { useTranslation } from 'react-i18next';
import { Button, Modal, Space, Tag } from 'antd';
import './DnsPresetsModal.css';

interface DnsPresetsModalProps {
  open: boolean;
  onClose: () => void;
  onInstall: (servers: string[]) => void;
}

const PRESETS: { name: string; family: boolean; data: string[] }[] = [
  {
    name: 'Google DNS',
    family: false,
    data: ['8.8.8.8', '8.8.4.4', '2001:4860:4860::8888', '2001:4860:4860::8844'],
  },
  {
    name: 'Cloudflare DNS',
    family: false,
    data: ['1.1.1.1', '1.0.0.1', '2606:4700:4700::1111', '2606:4700:4700::1001'],
  },
  {
    name: 'AdGuard DNS',
    family: false,
    data: ['94.140.14.14', '94.140.15.15', '2a10:50c0::ad1:ff', '2a10:50c0::ad2:ff'],
  },
  {
    name: 'AdGuard Family DNS',
    family: true,
    data: ['94.140.14.15', '94.140.15.16', '2a10:50c0::bad1:ff', '2a10:50c0::bad2:ff'],
  },
  {
    name: 'Cloudflare Family DNS',
    family: true,
    data: ['1.1.1.3', '1.0.0.3', '2606:4700:4700::1113', '2606:4700:4700::1003'],
  },
];

export default function DnsPresetsModal({ open, onClose, onInstall }: DnsPresetsModalProps) {
  const { t } = useTranslation();

  return (
    <Modal
      open={open}
      title={t('pages.xray.dns.dnsPresetTitle')}
      footer={null}
      mask={{ closable: false }}
      onCancel={onClose}
    >
      <div className="preset-list">
        {PRESETS.map((preset) => (
          <div key={preset.name} className="preset-row">
            <Space size="small" align="center">
              <Tag color={preset.family ? 'purple' : 'green'}>
                {preset.family ? t('pages.xray.dns.dnsPresetFamily') : 'DNS'}
              </Tag>
              <span className="preset-name">{preset.name}</span>
            </Space>
            <Button type="primary" size="small" onClick={() => onInstall([...preset.data])}>
              {t('install')}
            </Button>
          </div>
        ))}
      </div>
    </Modal>
  );
}
