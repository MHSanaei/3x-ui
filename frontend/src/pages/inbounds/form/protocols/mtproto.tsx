import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch } from 'antd';

import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function MtprotoFields() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  const routeThroughXray = Form.useWatch(['settings', 'routeThroughXray'], form) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Form.Item
        name={['settings', 'fakeTlsDomain']}
        label={t('pages.inbounds.form.fakeTlsDomain')}
        tooltip={t('pages.inbounds.form.mtprotoFakeTlsDomainHint')}
      >
        <Input placeholder="www.cloudflare.com" />
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
        name={['settings', 'throttleMaxConnections']}
        label={t('pages.inbounds.form.mtgThrottleMaxConnections')}
        tooltip={t('pages.inbounds.form.mtgThrottleMaxConnectionsHint')}
      >
        <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
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
      <Form.Item
        name={['settings', 'publicIpv4']}
        label={t('pages.inbounds.form.mtgPublicIpv4')}
        tooltip={t('pages.inbounds.form.mtgPublicIpHint')}
      >
        <Input allowClear placeholder="1.2.3.4" />
      </Form.Item>
      <Form.Item
        name={['settings', 'publicIpv6']}
        label={t('pages.inbounds.form.mtgPublicIpv6')}
        tooltip={t('pages.inbounds.form.mtgPublicIpHint')}
      >
        <Input allowClear placeholder="2001:db8::1" />
      </Form.Item>
    </>
  );
}
