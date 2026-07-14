import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Space, Tag } from 'antd';
import './DnsPresetsModal.css';

interface DnsPresetsModalProps {
  open: boolean;
  onClose: () => void;
  onInstall: (servers: string[]) => void;
}

export const PRESETS: { name: string; tags: string[]; data: string[] }[] = [
  {
    name: 'Google DNS',
    tags: ['UDP'],
    data: ['8.8.8.8', '8.8.4.4', '2001:4860:4860::8888', '2001:4860:4860::8844'],
  },
  {
    name: 'Cloudflare DNS',
    tags: ['UDP'],
    data: ['1.1.1.1', '1.0.0.1', '2606:4700:4700::1111', '2606:4700:4700::1001'],
  },
  {
    name: 'AdGuard DNS',
    tags: ['UDP'],
    data: ['94.140.14.14', '94.140.15.15', '2a10:50c0::ad1:ff', '2a10:50c0::ad2:ff'],
  },
  {
    name: 'AdGuard Family DNS',
    tags: ['UDP', 'Family'],
    data: ['94.140.14.15', '94.140.15.16', '2a10:50c0::bad1:ff', '2a10:50c0::bad2:ff'],
  },
  {
    name: 'Cloudflare Family DNS',
    tags: ['UDP', 'Family'],
    data: ['1.1.1.3', '1.0.0.3', '2606:4700:4700::1113', '2606:4700:4700::1003'],
  },
  {
    name: 'Cloudflare DoH',
    tags: ['DoH'],
    data: ['https://cloudflare-dns.com/dns-query'],
  },
  {
    name: 'Google DoH',
    tags: ['DoH'],
    data: ['https://dns.google/dns-query'],
  },
  {
    name: 'Quad9 Secure DoH',
    tags: ['DoH', 'Malware'],
    data: ['https://dns.quad9.net/dns-query'],
  },
  {
    name: 'AdGuard DoH + DoQ',
    tags: ['DoH', 'DoQ', 'Ads'],
    data: ['https://dns.adguard-dns.com/dns-query', 'quic+local://dns.adguard-dns.com'],
  },
  {
    name: 'Control D Ads DoH + DoQ',
    tags: ['DoH', 'DoQ', 'Ads'],
    data: ['https://freedns.controld.com/p2', 'quic+local://p2.freedns.controld.com'],
  },
  {
    name: 'Control D Family DoH + DoQ',
    tags: ['DoH', 'DoQ', 'Family'],
    data: ['https://freedns.controld.com/p4', 'quic+local://p4.freedns.controld.com'],
  },
];

function tagLabel(tag: string, t: (key: string) => string): string {
  if (tag === 'Family') return t('pages.xray.dns.dnsPresetFamily');
  return tag;
}

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
      <Alert
        type="warning"
        showIcon
        className="preset-warning"
        title={t('pages.xray.dns.dnsLeakWarning')}
      />
      <div className="preset-list">
        {PRESETS.map((preset) => (
          <div key={preset.name} className="preset-row">
            <Space size="small" align="center">
              {preset.tags.map((tag) => (
                <Tag key={tag} color={tag === 'Family' ? 'purple' : tag === 'UDP' ? 'orange' : 'green'}>
                  {tagLabel(tag, t)}
                </Tag>
              ))}
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
