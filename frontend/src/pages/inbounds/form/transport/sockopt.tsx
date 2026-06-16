import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, InputNumber, Segmented, Select, Space, Switch } from 'antd';

import {
  Address_Port_Strategy,
  DOMAIN_STRATEGY_OPTION,
  TCP_CONGESTION_OPTION,
} from '@/schemas/primitives';
import { HappyEyeballsSchema } from '@/schemas/protocols/stream/sockopt';

// Transport key that carries its own acceptProxyProtocol field (mirrored
// alongside the sockopt-level one so the PROXY preset never silently no-ops).
const TRANSPORT_PROXY_FIELD: Record<string, string> = {
  tcp: 'tcpSettings',
  ws: 'wsSettings',
  httpupgrade: 'httpupgradeSettings',
};
// Transports on which xray-core honors sockopt.trustedXForwardedFor.
const TRUSTED_HEADER_NETWORKS = ['ws', 'httpupgrade', 'xhttp'];

type RealClientIpPreset = 'off' | 'cloudflare' | 'proxy';

export default function SockoptForm({
  toggleSockopt,
  network,
}: {
  toggleSockopt: (on: boolean) => void;
  network: string;
}) {
  const { t } = useTranslation();

  // Presets write the same sockopt fields the user could set by hand below,
  // picking the mechanism xray-core actually honors for the chosen transport:
  // CF-Connecting-IP via trustedXForwardedFor (ws/httpupgrade/xhttp) or the
  // PROXY-protocol header via acceptProxyProtocol (every transport but mKCP).
  const applyRealClientIpPreset = (
    preset: RealClientIpPreset,
    getFieldValue: (name: (string | number)[]) => unknown,
    setFieldValue: (name: (string | number)[], value: unknown) => void,
  ) => {
    const sockopt = getFieldValue(['streamSettings', 'sockopt']);
    const sockoptOn =
      !!sockopt && typeof sockopt === 'object' && Object.keys(sockopt as object).length > 0;
    if (preset !== 'off' && !sockoptOn) {
      toggleSockopt(true);
    }
    const transportField = TRANSPORT_PROXY_FIELD[network];

    if (preset === 'off') {
      setFieldValue(['streamSettings', 'sockopt', 'trustedXForwardedFor'], []);
      setFieldValue(['streamSettings', 'sockopt', 'acceptProxyProtocol'], false);
      if (transportField) setFieldValue(['streamSettings', transportField, 'acceptProxyProtocol'], false);
      return;
    }

    if (preset === 'cloudflare') {
      const current = getFieldValue(['streamSettings', 'sockopt', 'trustedXForwardedFor']);
      const list = Array.isArray(current) ? [...(current as string[])] : [];
      if (!list.includes('CF-Connecting-IP')) list.push('CF-Connecting-IP');
      setFieldValue(['streamSettings', 'sockopt', 'trustedXForwardedFor'], list);
      setFieldValue(['streamSettings', 'sockopt', 'acceptProxyProtocol'], false);
      if (transportField) setFieldValue(['streamSettings', transportField, 'acceptProxyProtocol'], false);
      return;
    }

    // proxy — clear trustedXForwardedFor so a lingering header can't override the
    // PROXY-recovered IP (xray reads the header last on ws/httpupgrade/xhttp).
    setFieldValue(['streamSettings', 'sockopt', 'trustedXForwardedFor'], []);
    setFieldValue(['streamSettings', 'sockopt', 'acceptProxyProtocol'], true);
    if (transportField) setFieldValue(['streamSettings', transportField, 'acceptProxyProtocol'], true);
  };

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
                <Form.Item
                  noStyle
                  shouldUpdate={(prev, curr) => {
                    type ProxyWatch = {
                      streamSettings?: {
                        sockopt?: { trustedXForwardedFor?: unknown; acceptProxyProtocol?: unknown };
                        tcpSettings?: { acceptProxyProtocol?: unknown };
                        wsSettings?: { acceptProxyProtocol?: unknown };
                        httpupgradeSettings?: { acceptProxyProtocol?: unknown };
                      };
                    };
                    const pick = (v: ProxyWatch) => {
                      const s = v.streamSettings;
                      return JSON.stringify([
                        s?.sockopt?.trustedXForwardedFor,
                        s?.sockopt?.acceptProxyProtocol,
                        s?.tcpSettings?.acceptProxyProtocol,
                        s?.wsSettings?.acceptProxyProtocol,
                        s?.httpupgradeSettings?.acceptProxyProtocol,
                      ]);
                    };
                    return pick(prev as ProxyWatch) !== pick(curr as ProxyWatch);
                  }}
                >
                  {({ getFieldValue, setFieldValue }) => {
                    const sockopt = (getFieldValue(['streamSettings', 'sockopt']) ?? {}) as Record<
                      string,
                      unknown
                    >;
                    const transportField = TRANSPORT_PROXY_FIELD[network];
                    const transportPP = transportField
                      ? getFieldValue(['streamSettings', transportField, 'acceptProxyProtocol']) === true
                      : false;
                    const proxyOn = sockopt.acceptProxyProtocol === true || transportPP;
                    const trusted = Array.isArray(sockopt.trustedXForwardedFor)
                      ? (sockopt.trustedXForwardedFor as string[])
                      : [];
                    const value: RealClientIpPreset = proxyOn
                      ? 'proxy'
                      : trusted.length > 0
                        ? 'cloudflare'
                        : 'off';
                    const trustedMismatch =
                      trusted.length > 0 && !TRUSTED_HEADER_NETWORKS.includes(network);
                    const proxyMismatch = proxyOn && network === 'kcp';
                    return (
                      <>
                        <Form.Item
                          label={t('pages.inbounds.form.realClientIp')}
                          tooltip={t('pages.inbounds.form.realClientIpHint')}
                        >
                          <Segmented
                            value={value}
                            onChange={(v) =>
                              applyRealClientIpPreset(v as RealClientIpPreset, getFieldValue, setFieldValue)
                            }
                            options={[
                              { value: 'off', label: t('pages.inbounds.form.realClientIpPresetOff') },
                              { value: 'cloudflare', label: t('pages.inbounds.form.realClientIpPresetCloudflare') },
                              { value: 'proxy', label: t('pages.inbounds.form.realClientIpPresetProxyProtocol') },
                            ]}
                          />
                        </Form.Item>
                        {trustedMismatch && (
                          <Alert
                            type="warning"
                            showIcon
                            style={{ marginBottom: 16 }}
                            message={t('pages.inbounds.form.realClientIpTrustedHeaderTransportWarn')}
                          />
                        )}
                        {proxyMismatch && (
                          <Alert
                            type="warning"
                            showIcon
                            style={{ marginBottom: 16 }}
                            message={t('pages.inbounds.form.realClientIpProxyProtocolTransportWarn')}
                          />
                        )}
                      </>
                    );
                  }}
                </Form.Item>
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
                  tooltip={t('pages.inbounds.form.tcpWindowClampHint')}
                >
                  <InputNumber min={0} />
                </Form.Item>
                <Form.Item
                  name={['streamSettings', 'sockopt', 'acceptProxyProtocol']}
                  label={t('pages.inbounds.form.proxyProtocol')}
                  tooltip={t('pages.inbounds.form.proxyProtocolHint')}
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
                  tooltip={t('pages.inbounds.form.trustedXForwardedForHint')}
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
