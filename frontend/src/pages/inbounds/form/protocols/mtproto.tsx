import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { generateMtprotoSecret, mtprotoSecretForDomain } from '@/lib/xray/inbound-defaults';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function MtprotoFields() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  const routeThroughXray = Form.useWatch(['settings', 'routeThroughXray'], form) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Form.Item name={['settings', 'fakeTlsDomain']} label={t('pages.inbounds.form.fakeTlsDomain')}>
        <Input
          placeholder="www.cloudflare.com"
          onChange={(e) => {
            const current = (form.getFieldValue(['settings', 'secret']) as string) ?? '';
            form.setFieldValue(['settings', 'secret'], mtprotoSecretForDomain(current, e.target.value));
          }}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.form.mtprotoSecret')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'secret']} noStyle>
            <Input readOnly style={{ width: 'calc(100% - 32px)' }} />
          </Form.Item>
          <Button
            icon={<ReloadOutlined />}
            onClick={() => {
              const domain = form.getFieldValue(['settings', 'fakeTlsDomain']);
              form.setFieldValue(['settings', 'secret'], generateMtprotoSecret(domain as string));
            }}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        name={['settings', 'domainFronting', 'ip']}
        label={t('pages.inbounds.form.mtgDomainFrontingIp')}
        tooltip={t('pages.inbounds.form.mtgDomainFrontingHint')}
      >
        <Input placeholder="127.0.0.1" />
      </Form.Item>
      <Form.Item name={['settings', 'domainFronting', 'port']} label={t('pages.inbounds.form.mtgDomainFrontingPort')}>
        <InputNumber min={0} max={65535} placeholder="443" style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item
        name={['settings', 'domainFronting', 'proxyProtocol']}
        label={t('pages.inbounds.form.mtgDomainFrontingProxyProtocol')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name={['settings', 'proxyProtocolListener']}
        label={t('pages.inbounds.form.mtgProxyProtocolListener')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item name={['settings', 'preferIp']} label={t('pages.inbounds.form.mtgPreferIp')}>
        <Select
          allowClear
          placeholder="prefer-ipv6"
          options={[
            { value: 'prefer-ipv6', label: 'prefer-ipv6' },
            { value: 'prefer-ipv4', label: 'prefer-ipv4' },
            { value: 'only-ipv6', label: 'only-ipv6' },
            { value: 'only-ipv4', label: 'only-ipv4' },
          ]}
        />
      </Form.Item>
      <Form.Item name={['settings', 'debug']} label={t('pages.inbounds.form.mtgDebug')} valuePropName="checked">
        <Switch />
      </Form.Item>
      <Form.Item
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.mtgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.mtgRouteThroughXrayHint')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {routeThroughXray && (
        <Form.Item
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.mtgRouteOutbound')}
          tooltip={t('pages.inbounds.form.mtgRouteOutboundHint')}
        >
          <Select
            allowClear
            showSearch
            placeholder={t('pages.inbounds.form.mtgRouteOutboundPlaceholder')}
            options={(outboundTags ?? []).map((tag) => ({ value: tag, label: tag }))}
          />
        </Form.Item>
      )}
    </>
  );
}
