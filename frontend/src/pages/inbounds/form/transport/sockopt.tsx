import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';

import {
  Address_Port_Strategy,
  DOMAIN_STRATEGY_OPTION,
  TCP_CONGESTION_OPTION,
} from '@/schemas/primitives';
import { HappyEyeballsSchema } from '@/schemas/protocols/stream/sockopt';

export default function SockoptForm({
  toggleSockopt,
}: {
  toggleSockopt: (on: boolean) => void;
}) {
  const { t } = useTranslation();
  return (
    <Form.Item
      noStyle
      shouldUpdate={(prev, curr) => {
        const a = (prev.streamSettings as { sockopt?: object } | undefined)?.sockopt;
        const b = (curr.streamSettings as { sockopt?: object } | undefined)?.sockopt;
        return !!a !== !!b;
      }}
    >
      {({ getFieldValue }) => {
        const sock = getFieldValue(['streamSettings', 'sockopt']);
        const on = !!sock && typeof sock === 'object' && Object.keys(sock).length > 0;
        return (
          <>
            <Form.Item label="Sockopt">
              <Switch checked={on} onChange={toggleSockopt} />
            </Form.Item>
            {on && (
              <>
                <Form.Item name={['streamSettings', 'sockopt', 'mark']} label={t('pages.inbounds.form.routeMark')}>
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
                  label={t('pages.inbounds.form.tcpKeepAliveInterval')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpKeepAliveIdle']}
                  label={t('pages.inbounds.form.tcpKeepAliveIdle')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item name={['streamSettings', 'sockopt', 'tcpMaxSeg']} label={t('pages.inbounds.form.tcpMaxSeg')}>
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpUserTimeout']}
                  label={t('pages.inbounds.form.tcpUserTimeout')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpWindowClamp']}
                  label={t('pages.inbounds.form.tcpWindowClamp')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'acceptProxyProtocol']}
                  label={t('pages.inbounds.form.proxyProtocol')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpFastOpen']}
                  label={t('pages.inbounds.form.tcpFastOpen')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpMptcp']}
                  label={t('pages.inbounds.form.multipathTcp')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'penetrate']}
                  label={t('pages.inbounds.form.penetrate')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'V6Only']}
                  label={t('pages.inbounds.form.v6Only')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'domainStrategy']}
                  label={t('pages.xray.wireguard.domainStrategy')}
                >
                  <Select
                    style={{ width: '50%' }}
                    options={Object.values(DOMAIN_STRATEGY_OPTION).map((d) => ({ value: d, label: d }))}
                  />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'tcpcongestion']}
                  label={t('pages.inbounds.form.tcpCongestion')}
                >
                  <Select
                    style={{ width: '50%' }}
                    options={Object.values(TCP_CONGESTION_OPTION).map((c) => ({ value: c, label: c }))}
                  />
                </Form.Item>
                <Form.Item name={['streamSettings', 'sockopt', 'tproxy']} label="TProxy">
                  <Select
                    style={{ width: '50%' }}
                    options={[
                      { value: 'off', label: 'Off' },
                      { value: 'redirect', label: 'Redirect' },
                      { value: 'tproxy', label: 'TProxy' },
                    ]}
                  />
                </Form.Item>
                <Form.Item name={['streamSettings', 'sockopt', 'dialerProxy']} label={t('pages.inbounds.form.dialerProxy')}>
                  <Input />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'interface']}
                  label={t('pages.inbounds.info.interfaceName')}
                >
                  <Input />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'trustedXForwardedFor']}
                  label={t('pages.inbounds.form.trustedXForwardedFor')}
                >
                  <Select
                    mode="tags"
                    style={{ width: '100%' }}
                    tokenSeparators={[',']}
                    options={[
                      { value: 'CF-Connecting-IP', label: 'CF-Connecting-IP' },
                      { value: 'X-Real-IP', label: 'X-Real-IP' },
                      { value: 'True-Client-IP', label: 'True-Client-IP' },
                      { value: 'X-Client-IP', label: 'X-Client-IP' },
                    ]}
                  />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'addressPortStrategy']}
                  label={t('pages.inbounds.form.addressPortStrategy')}
                >
                  <Select
                    style={{ width: '50%' }}
                    options={Object.values(Address_Port_Strategy).map((v) => ({ value: v, label: v }))}
                  />
                </Form.Item>
                <Form.Item shouldUpdate noStyle>
                  {({ getFieldValue, setFieldValue }) => {
                    const he = getFieldValue(['streamSettings', 'sockopt', 'happyEyeballs']);
                    const hasHe = he != null;
                    return (
                      <>
                        <Form.Item label="Happy Eyeballs">
                          <Switch
                            checked={hasHe}
                            onChange={(v) => {
                              setFieldValue(
                                ['streamSettings', 'sockopt', 'happyEyeballs'],
                                v ? HappyEyeballsSchema.parse({}) : undefined,
                              );
                            }}
                          />
                        </Form.Item>
                        {hasHe && (
                          <>
                            <Form.Item
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'tryDelayMs']}
                              label={t('pages.inbounds.form.tryDelayMs')}
                            >
                              <InputNumber min={0} placeholder="0 disabled — 250 recommended" />
                            </Form.Item>
                            <Form.Item
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'prioritizeIPv6']}
                              label={t('pages.inbounds.form.prioritizeIPv6')}
                              valuePropName="checked"
                            >
                              <Switch />
                            </Form.Item>
                            <Form.Item
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'interleave']}
                              label={t('pages.inbounds.form.interleave')}
                            >
                              <InputNumber min={1} />
                            </Form.Item>
                            <Form.Item
                              name={['streamSettings', 'sockopt', 'happyEyeballs', 'maxConcurrentTry']}
                              label={t('pages.inbounds.form.maxConcurrentTry')}
                            >
                              <InputNumber min={0} />
                            </Form.Item>
                          </>
                        )}
                      </>
                    );
                  }}
                </Form.Item>
                <Form.List name={['streamSettings', 'sockopt', 'customSockopt']}>
                  {(fields, { add, remove }) => (
                    <>
                      <Form.Item label={t('pages.inbounds.form.customSockopt')}>
                        <Button
                          type="dashed"
                          size="small"
                          onClick={() => add({ type: 'int', level: '6', opt: '', value: '' })}
                        >
                          + {t('pages.inbounds.form.addCustomOption')}
                        </Button>
                      </Form.Item>
                      {fields.map((field) => (
                        <Space.Compact key={field.key} style={{ display: 'flex', marginBottom: 8 }}>
                          <Form.Item name={[field.name, 'system']} noStyle>
                            <Select
                              placeholder="all"
                              allowClear
                              style={{ width: 100 }}
                              options={[
                                { value: 'linux', label: 'linux' },
                                { value: 'windows', label: 'windows' },
                                { value: 'darwin', label: 'darwin' },
                              ]}
                            />
                          </Form.Item>
                          <Form.Item name={[field.name, 'type']} noStyle>
                            <Select
                              style={{ width: 80 }}
                              options={[
                                { value: 'int', label: 'int' },
                                { value: 'str', label: 'str' },
                              ]}
                            />
                          </Form.Item>
                          <Form.Item name={[field.name, 'level']} noStyle>
                            <Input placeholder="level (6=TCP)" style={{ width: 100 }} />
                          </Form.Item>
                          <Form.Item name={[field.name, 'opt']} noStyle>
                            <Input placeholder="opt" style={{ width: 120 }} />
                          </Form.Item>
                          <Form.Item name={[field.name, 'value']} noStyle>
                            <Input placeholder="value" style={{ flex: 1 }} />
                          </Form.Item>
                          <Button danger onClick={() => remove(field.name)}>−</Button>
                        </Space.Compact>
                      ))}
                    </>
                  )}
                </Form.List>
              </>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
