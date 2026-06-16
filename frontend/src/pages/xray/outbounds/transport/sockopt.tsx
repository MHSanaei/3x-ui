import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Select, Switch, type FormInstance } from 'antd';

import { CustomSockoptList } from '@/components/form';
import { DOMAIN_STRATEGY_OPTION, TCP_CONGESTION_OPTION } from '@/schemas/primitives';
import { HappyEyeballsSchema, SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

import { ADDRESS_PORT_STRATEGY_OPTIONS } from '../outbound-form-constants';

export default function SockoptForm({
  form,
  outboundTags = [],
}: {
  form: FormInstance<OutboundFormValues>;
  outboundTags?: string[];
}) {
  const { t } = useTranslation();
  return (
    <Form.Item shouldUpdate noStyle>
      {() => {
        const hasSockopt = !!form.getFieldValue([
          'streamSettings',
          'sockopt',
        ]);
        const dialerProxy = (form.getFieldValue([
          'streamSettings',
          'sockopt',
          'dialerProxy',
        ]) ?? '') as string;
        const dialerProxyOptions = Array.from(
          new Set([...outboundTags, dialerProxy].filter(Boolean)),
        ).map((tg) => ({ value: tg, label: tg }));
        return (
          <>
            <Form.Item label={t('pages.xray.outboundForm.sockopts')}>
              <Switch
                checked={hasSockopt}
                onChange={(checked) => {
                  form.setFieldValue(
                    ['streamSettings', 'sockopt'],
                    checked ? SockoptStreamSettingsSchema.parse({}) : undefined,
                  );
                }}
              />
            </Form.Item>
            {hasSockopt && (
              <>
                <Form.Item
                  label={t('pages.inbounds.form.dialerProxy')}
                  name={['streamSettings', 'sockopt', 'dialerProxy']}
                  tooltip={t('pages.xray.outboundForm.dialerProxyHint')}
                >
                  <Select
                    allowClear
                    showSearch
                    placeholder={t('pages.xray.outboundForm.dialerProxyPlaceholder')}
                    options={dialerProxyOptions}
                  />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.wireguard.domainStrategy')}
                  name={['streamSettings', 'sockopt', 'domainStrategy']}
                >
                  <Select
                    options={Object.values(DOMAIN_STRATEGY_OPTION).map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.addressPortStrategy')}
                  name={['streamSettings', 'sockopt', 'addressPortStrategy']}
                >
                  <Select options={ADDRESS_PORT_STRATEGY_OPTIONS} />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.outboundForm.keepAliveInterval')}
                  name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.tcpFastOpen')}
                  name={['streamSettings', 'sockopt', 'tcpFastOpen']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.multipathTcp')}
                  name={['streamSettings', 'sockopt', 'tcpMptcp']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.penetrate')}
                  name={['streamSettings', 'sockopt', 'penetrate']}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.outboundForm.markFwmark')}
                  name={['streamSettings', 'sockopt', 'mark']}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.outboundForm.interface')}
                  name={['streamSettings', 'sockopt', 'interface']}
                >
                  <Input />
                </Form.Item>
                <Form.Item
                  label="TProxy"
                  name={['streamSettings', 'sockopt', 'tproxy']}
                >
                  <Select
                    options={[
                      { value: 'off', label: 'off' },
                      { value: 'redirect', label: 'redirect' },
                      { value: 'tproxy', label: 'tproxy' },
                    ]}
                  />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.tcpCongestion')}
                  name={['streamSettings', 'sockopt', 'tcpcongestion']}
                >
                  <Select
                    options={Object.values(TCP_CONGESTION_OPTION).map((v) => ({
                      value: v,
                      label: v,
                    }))}
                  />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.outboundForm.tcpUserTimeoutMs')}
                  name={['streamSettings', 'sockopt', 'tcpUserTimeout']}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  label={t('pages.xray.outboundForm.tcpKeepAliveIdleS')}
                  name={['streamSettings', 'sockopt', 'tcpKeepAliveIdle']}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.tcpMaxSeg')}
                  name={['streamSettings', 'sockopt', 'tcpMaxSeg']}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  label={t('pages.inbounds.form.tcpWindowClamp')}
                  name={['streamSettings', 'sockopt', 'tcpWindowClamp']}
                  tooltip={t('pages.inbounds.form.tcpWindowClampHint')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item shouldUpdate noStyle>
                  {() => {
                    const he = form.getFieldValue([
                      'streamSettings', 'sockopt', 'happyEyeballs',
                    ]);
                    const hasHe = he != null;
                    return (
                      <>
                        <Form.Item label="Happy Eyeballs">
                          <Switch
                            checked={hasHe}
                            onChange={(v) => {
                              form.setFieldValue(
                                ['streamSettings', 'sockopt', 'happyEyeballs'],
                                v ? HappyEyeballsSchema.parse({}) : undefined,
                              );
                            }}
                          />
                        </Form.Item>
                        {hasHe && (
                          <>
                            <Form.Item
                              label={t('pages.inbounds.form.tryDelayMs')}
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'tryDelayMs']}
                            >
                              <InputNumber min={0} placeholder="0 (disabled) — 250 recommended" />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.prioritizeIPv6')}
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'prioritizeIPv6']}
                              valuePropName="checked"
                            >
                              <Switch />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.interleave')}
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'interleave']}
                            >
                              <InputNumber min={1} />
                            </Form.Item>
                            <Form.Item
                              label={t('pages.inbounds.form.maxConcurrentTry')}
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'maxConcurrentTry']}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                          </>
                        )}
                      </>
                    );
                  }}
                </Form.Item>
                <CustomSockoptList />
              </>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
