import { useTranslation } from 'react-i18next';
import { Alert, Form, InputNumber, Segmented, Select, Switch } from 'antd';
import { Controller, useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { SockoptCustomField } from '@/lib/xray/forms/fields';
import { TCP_CONGESTION_OPTION } from '@/schemas/primitives';

/* Transport key that carries its own acceptProxyProtocol field (mirrored
   alongside the sockopt-level one so the PROXY preset never silently no-ops). */
const TRANSPORT_PROXY_FIELD: Record<string, string> = {
  tcp: 'tcpSettings',
  ws: 'wsSettings',
  httpupgrade: 'httpupgradeSettings',
};
/* Transports on which xray-core honors sockopt.trustedXForwardedFor. gRPC joined
   in v26.6.22 (xray-core 711aea4): it now reads X-Forwarded-For via this option
   instead of the old x-real-ip gRPC metadata. */
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
  const { control, getValues, setValue } = useFormContext();
  const sock = useWatch({ control, name: 'streamSettings.sockopt' });
  const on = !!sock && typeof sock === 'object' && Object.keys(sock).length > 0;

  const transportField = TRANSPORT_PROXY_FIELD[network];
  const sockAcceptPP = useWatch({ control, name: 'streamSettings.sockopt.acceptProxyProtocol' });
  const sockTrusted = useWatch({ control, name: 'streamSettings.sockopt.trustedXForwardedFor' });
  const transportAcceptPP = useWatch({
    control,
    name: transportField ? `streamSettings.${transportField}.acceptProxyProtocol` : 'streamSettings.__noTransportProxyField',
  });

  /* Presets write the same sockopt fields the user could set by hand below,
     picking the mechanism xray-core actually honors for the chosen transport:
     CF-Connecting-IP via trustedXForwardedFor (ws/httpupgrade/xhttp/grpc) or the
     PROXY-protocol header via acceptProxyProtocol (every transport but mKCP). */
  const applyRealClientIpPreset = (preset: RealClientIpPreset) => {
    const sockopt = getValues('streamSettings.sockopt');
    const sockoptOn =
      !!sockopt && typeof sockopt === 'object' && Object.keys(sockopt as object).length > 0;
    if (preset !== 'off' && !sockoptOn) {
      toggleSockopt(true);
    }

    if (preset === 'off') {
      setValue('streamSettings.sockopt.trustedXForwardedFor', []);
      setValue('streamSettings.sockopt.acceptProxyProtocol', false);
      if (transportField) setValue(`streamSettings.${transportField}.acceptProxyProtocol`, false);
      return;
    }

    if (preset === 'cloudflare') {
      const current = getValues('streamSettings.sockopt.trustedXForwardedFor');
      const list = Array.isArray(current) ? [...(current as string[])] : [];
      if (!list.includes('CF-Connecting-IP')) list.push('CF-Connecting-IP');
      setValue('streamSettings.sockopt.trustedXForwardedFor', list);
      setValue('streamSettings.sockopt.acceptProxyProtocol', false);
      if (transportField) setValue(`streamSettings.${transportField}.acceptProxyProtocol`, false);
      return;
    }

    /* proxy — clear trustedXForwardedFor so a lingering header can't override the
       PROXY-recovered IP (xray reads the header last on ws/httpupgrade/xhttp/grpc). */
    setValue('streamSettings.sockopt.trustedXForwardedFor', []);
    setValue('streamSettings.sockopt.acceptProxyProtocol', true);
    if (transportField) setValue(`streamSettings.${transportField}.acceptProxyProtocol`, true);
  };

  const transportPP = transportField ? transportAcceptPP === true : false;
  const proxyOn = sockAcceptPP === true || transportPP;
  const trusted = Array.isArray(sockTrusted) ? (sockTrusted as string[]) : [];
  const presetValue: RealClientIpPreset = proxyOn
    ? 'proxy'
    : trusted.length > 0
      ? 'cloudflare'
      : 'off';
  const trustedMismatch = trusted.length > 0 && !TRUSTED_HEADER_NETWORKS.includes(network);
  const proxyMismatch = proxyOn && network === 'kcp';

  return (
    <>
      <Form.Item label="Sockopt">
        <Switch checked={on} onChange={toggleSockopt} aria-label="Sockopt" />
      </Form.Item>
      {on && (
        <>
          <Form.Item
            label={t('pages.inbounds.form.realClientIp')}
            tooltip={t('pages.inbounds.form.realClientIpHint')}
          >
            <Segmented
              value={presetValue}
              onChange={(v) => applyRealClientIpPreset(v as RealClientIpPreset)}
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
          <FormField name={['streamSettings', 'sockopt', 'mark']} label={t('pages.inbounds.form.routeMark')}>
            <InputNumber min={0} />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpKeepAliveInterval']}
            label={t('pages.inbounds.form.tcpKeepAliveInterval')}
          >
            <InputNumber min={0} />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpKeepAliveIdle']}
            label={t('pages.inbounds.form.tcpKeepAliveIdle')}
          >
            <InputNumber min={0} />
          </FormField>
          <FormField name={['streamSettings', 'sockopt', 'tcpMaxSeg']} label={t('pages.inbounds.form.tcpMaxSeg')}>
            <InputNumber min={0} />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpUserTimeout']}
            label={t('pages.inbounds.form.tcpUserTimeout')}
          >
            <InputNumber min={0} />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpWindowClamp']}
            label={t('pages.inbounds.form.tcpWindowClamp')}
            tooltip={t('pages.inbounds.form.tcpWindowClampHint')}
          >
            <InputNumber min={0} />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'acceptProxyProtocol']}
            label={t('pages.inbounds.form.proxyProtocol')}
            tooltip={t('pages.inbounds.form.proxyProtocolHint')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpFastOpen']}
            label={t('pages.inbounds.form.tcpFastOpen')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'penetrate']}
            label={t('pages.inbounds.form.penetrate')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'V6Only']}
            label={t('pages.inbounds.form.v6Only')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
          <FormField
            name={['streamSettings', 'sockopt', 'tcpcongestion']}
            label={t('pages.inbounds.form.tcpCongestion')}
          >
            <Select
              style={{ width: '50%' }}
              options={Object.values(TCP_CONGESTION_OPTION).map((c) => ({ value: c, label: c }))}
            />
          </FormField>
          <FormField name={['streamSettings', 'sockopt', 'tproxy']} label="TProxy">
            <Select
              style={{ width: '50%' }}
              options={[
                { value: 'off', label: 'Off' },
                { value: 'redirect', label: 'Redirect' },
                { value: 'tproxy', label: 'TProxy' },
              ]}
            />
          </FormField>
          <FormField
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
          </FormField>
          <Controller
            control={control}
            name="streamSettings.sockopt.customSockopt"
            render={({ field }) => (
              <SockoptCustomField value={field.value} onChange={field.onChange} />
            )}
          />
        </>
      )}
    </>
  );
}
