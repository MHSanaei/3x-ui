import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

interface WireguardFieldsProps {
  wgPubKey: string;
  regenInboundWg: () => void;
}

export default function WireguardFields({ wgPubKey, regenInboundWg }: WireguardFieldsProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('pages.xray.wireguard.secretKey')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'secretKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button aria-label={t('regenerate')} icon={<ReloadOutlined />} onClick={regenInboundWg} />
        </Space.Compact>
      </Form.Item>
      <Form.Item label={t('pages.xray.wireguard.publicKey')}>
        <Input value={wgPubKey} disabled />
      </Form.Item>
      <Form.Item name={['settings', 'mtu']} label="MTU">
        <InputNumber />
      </Form.Item>
      <Form.Item name={['settings', 'dns']} label={t('pages.inbounds.info.dns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </Form.Item>
      <Form.Item
        name={['settings', 'noKernelTun']}
        label={t('pages.inbounds.info.noKernelTun')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item name={['settings', 'domainStrategy']} label={t('pages.xray.wireguard.domainStrategy')}>
        <Select
          allowClear
          options={[
            { value: 'ForceIP', label: 'ForceIP' },
            { value: 'ForceIPv4', label: 'ForceIPv4' },
            { value: 'ForceIPv4v6', label: 'ForceIPv4v6' },
            { value: 'ForceIPv6', label: 'ForceIPv6' },
            { value: 'ForceIPv6v4', label: 'ForceIPv6v4' },
          ]}
        />
      </Form.Item>
    </>
  );
}
