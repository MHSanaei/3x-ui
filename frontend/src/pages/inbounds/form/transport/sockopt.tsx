import { useTranslation } from 'react-i18next';
import { Alert, Form, InputNumber, Segmented, Select, Switch } from 'antd';

import { CustomSockoptList } from '@/components/form';
import { TCP_CONGESTION_OPTION } from '@/schemas/primitives';

// Transport key that carries its own acceptProxyProtocol field (mirrored
// alongside the sockopt-level one so the PROXY preset never silently no-ops).
const TRANSPORT_PROXY_FIELD: Record<string, string> = {
  tcp: 'tcpSettings',
  ws: 'wsSettings',
  httpupgrade: 'httpupgradeSettings',
};
// Transports on which xray-core honors sockopt.trustedXForwardedFor. gRPC joined
// in v26.6.22 (xray-core 711aea4): it now reads X-Forwarded-For via this option
// instead of the old x-real-ip gRPC metadata.
const TRUSTED_HEADER_NETWORKS = ['ws', 'httpupgrade', 'xhttp', 'grpc'];

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
  // CF-Connecting-IP via trustedXForwardedFor (ws/httpupgrade/xhttp/grpc) or the
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
    // PROXY-recovered IP (xray reads the header last on ws/httpupgrade/xhttp/grpc).
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
                            title={t('pages.inbounds.form.realClientIpTrustedHeaderTransportWarn')}
                          />
                        )}
                        {proxyMismatch && (
                          <Alert
                            type="warning"
                            showIcon
                            style={{ marginBottom: 16 }}
                            title={t('pages.inbounds.form.realClientIpProxyProtocolTransportWarn')}
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
                <CustomSockoptList />
              </>
            )}
          </>
        );
      }}
    </Form.Item>
  );
}
