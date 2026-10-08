import type { ReactElement } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Collapse, Input, InputNumber, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function MtprotoFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const routeThroughXray = useWatch({ control, name: 'settings.routeThroughXray' }) as
    | boolean
    | undefined;
  const dcPoolEnabled = useWatch({ control, name: 'settings.dcPool.enabled' }) as
    | boolean
    | undefined;
  const { data: outboundTags } = useOutboundTags({ excludeBlackhole: true });
  const f = (key: string) => t(`pages.inbounds.form.${key}`);
  // Free-form lists (URLs, proxies, CIDRs, DC ids) are entered as tags; the
  // dropdown stays closed because there is nothing to pick from.
  const tags = (placeholder: string) => (
    <Select
      mode="tags"
      open={false}
      allowClear
      tokenSeparators={[',', ' ']}
      placeholder={placeholder}
      style={{ width: '100%' }}
    />
  );
  const text = (placeholder: string) => <Input allowClear placeholder={placeholder} />;
  const num = (min: number, max: number, placeholder: string) => (
    <InputNumber min={min} max={max} placeholder={placeholder} style={{ width: '100%' }} />
  );
  const field = (
    name: string[],
    label: string,
    input: ReactElement,
    hint?: string,
    checked = false,
  ) => (
    <FormField
      key={name.join('.')}
      name={['settings', ...name]}
      label={f(label)}
      tooltip={hint ? f(hint) : undefined}
      valueProp={checked ? 'checked' : undefined}
    >
      {input}
    </FormField>
  );
  const ipList = (name: 'blocklist' | 'allowlist', label: string, hint: string) => [
    field(['defense', name, 'enabled'], label, <Switch />, hint, true),
    field(
      ['defense', name, 'urls'],
      'mtgIpListUrls',
      tags('https://iplists.firehol.org/files/firehol_level1.netset'),
      'mtgIpListUrlsHint',
    ),
    field(
      ['defense', name, 'updateEach'],
      'mtgIpListUpdateEach',
      text('24h'),
      'mtgIpListUpdateEachHint',
    ),
    field(
      ['defense', name, 'downloadConcurrency'],
      'mtgIpListDownloadConcurrency',
      num(1, 65535, '1'),
      'mtgIpListDownloadConcurrencyHint',
    ),
  ];

  const advanced = [
    {
      key: 'relay',
      label: f('mtgAdvancedRelay'),
      children: [
        <Alert
          key="hint"
          type="info"
          showIcon
          title={f('mtgRelayHint')}
          style={{ marginBottom: 16 }}
        />,
        field(['network', 'clientMss'], 'mtgClientMss', num(0, 1460, '92'), 'mtgClientMssHint'),
        field(
          ['network', 'clientMssBulk'],
          'mtgClientMssBulk',
          num(0, 65495, '1400'),
          'mtgClientMssBulkHint',
        ),
      ],
    },
    {
      key: 'general',
      label: f('mtgAdvancedGeneral'),
      children: [
        field(['concurrency'], 'mtgConcurrency', num(1, 65535, '4096'), 'mtgConcurrencyHint'),
        field(
          ['tolerateTimeSkewness'],
          'mtgTolerateTimeSkewness',
          text('3s'),
          'mtgTolerateTimeSkewnessHint',
        ),
        field(
          ['allowFallbackOnUnknownDc'],
          'mtgAllowFallbackOnUnknownDc',
          <Switch />,
          'mtgAllowFallbackOnUnknownDcHint',
          true,
        ),
        field(['autoUpdate'], 'mtgAutoUpdate', <Switch />, 'mtgAutoUpdateHint', true),
        field(
          ['throttleCheckInterval'],
          'mtgThrottleCheckInterval',
          text('5s'),
          'mtgThrottleCheckIntervalHint',
        ),
      ],
    },
    {
      key: 'network',
      label: f('mtgAdvancedNetwork'),
      children: [
        field(['network', 'dns'], 'mtgDns', text('https://1.1.1.1'), 'mtgDnsHint'),
        ...(routeThroughXray
          ? []
          : [
              field(
                ['network', 'proxies'],
                'mtgProxies',
                tags('socks5://user:pass@host:1080'),
                'mtgProxiesHint',
              ),
            ]),
        field(
          ['network', 'tcpNotSentLowat'],
          'mtgTcpNotSentLowat',
          text('128kib'),
          'mtgTcpNotSentLowatHint',
        ),
        field(['network', 'timeout', 'tcp'], 'mtgTimeoutTcp', text('10s'), 'mtgTimeoutTcpHint'),
        field(['network', 'timeout', 'http'], 'mtgTimeoutHttp', text('10s'), 'mtgTimeoutHttpHint'),
        field(['network', 'timeout', 'idle'], 'mtgTimeoutIdle', text('5m'), 'mtgTimeoutIdleHint'),
        field(
          ['network', 'timeout', 'handshake'],
          'mtgTimeoutHandshake',
          text('10s'),
          'mtgTimeoutHandshakeHint',
        ),
        field(
          ['network', 'keepAlive', 'disabled'],
          'mtgKeepAliveDisabled',
          <Switch />,
          undefined,
          true,
        ),
        field(
          ['network', 'keepAlive', 'idle'],
          'mtgKeepAliveIdle',
          text('15s'),
          'mtgKeepAliveIdleHint',
        ),
        field(
          ['network', 'keepAlive', 'interval'],
          'mtgKeepAliveInterval',
          text('15s'),
          'mtgKeepAliveIntervalHint',
        ),
        field(
          ['network', 'keepAlive', 'count'],
          'mtgKeepAliveCount',
          num(1, 65535, '9'),
          'mtgKeepAliveCountHint',
        ),
      ],
    },
    {
      key: 'defense',
      label: f('mtgAdvancedDefense'),
      children: [
        field(
          ['defense', 'antiReplay', 'enabled'],
          'mtgAntiReplay',
          <Switch />,
          'mtgAntiReplayHint',
          true,
        ),
        field(
          ['defense', 'antiReplay', 'maxSize'],
          'mtgAntiReplayMaxSize',
          text('1mib'),
          'mtgAntiReplayMaxSizeHint',
        ),
        field(
          ['defense', 'antiReplay', 'errorRate'],
          'mtgAntiReplayErrorRate',
          num(0, 99.999, '0.001'),
          'mtgAntiReplayErrorRateHint',
        ),
        ...ipList('blocklist', 'mtgBlocklist', 'mtgBlocklistHint'),
        ...ipList('allowlist', 'mtgAllowlist', 'mtgAllowlistHint'),
        field(
          ['defense', 'doppelganger', 'urls'],
          'mtgDoppelgangerUrls',
          tags('https://cdn.example.com/app.js'),
          'mtgDoppelgangerUrlsHint',
        ),
        field(
          ['defense', 'doppelganger', 'repeatsPerRaid'],
          'mtgDoppelgangerRepeats',
          num(1, 65535, '10'),
          'mtgDoppelgangerRepeatsHint',
        ),
        field(
          ['defense', 'doppelganger', 'raidEach'],
          'mtgDoppelgangerRaidEach',
          text('6h'),
          'mtgDoppelgangerRaidEachHint',
        ),
        field(
          ['defense', 'doppelganger', 'drs'],
          'mtgDoppelgangerDrs',
          <Switch />,
          'mtgDoppelgangerDrsHint',
          true,
        ),
        field(
          ['defense', 'pendingHandshakes', 'maxPerIp'],
          'mtgPendingMaxPerIp',
          num(0, 65535, '0'),
          'mtgPendingMaxPerIpHint',
        ),
        field(
          ['defense', 'pendingHandshakes', 'dryRun'],
          'mtgPendingDryRun',
          <Switch />,
          'mtgPendingDryRunHint',
          true,
        ),
      ],
    },
    {
      key: 'stats',
      label: f('mtgAdvancedStats'),
      children: [
        field(
          ['stats', 'prometheus', 'enabled'],
          'mtgPrometheus',
          <Switch />,
          'mtgPrometheusHint',
          true,
        ),
        field(
          ['stats', 'prometheus', 'bindTo'],
          'mtgPrometheusBindTo',
          text('127.0.0.1:3129'),
          'mtgPrometheusBindToHint',
        ),
        field(
          ['stats', 'prometheus', 'httpPath'],
          'mtgPrometheusPath',
          text('/'),
          'mtgPrometheusPathHint',
        ),
        field(
          ['stats', 'prometheus', 'metricPrefix'],
          'mtgMetricPrefix',
          text('mtg'),
          'mtgMetricPrefixHint',
        ),
        field(['stats', 'statsd', 'enabled'], 'mtgStatsd', <Switch />, 'mtgStatsdHint', true),
        field(
          ['stats', 'statsd', 'address'],
          'mtgStatsdAddress',
          text('127.0.0.1:8125'),
          'mtgStatsdAddressHint',
        ),
        field(
          ['stats', 'statsd', 'metricPrefix'],
          'mtgMetricPrefix',
          text('mtg'),
          'mtgMetricPrefixHint',
        ),
        field(
          ['stats', 'statsd', 'tagFormat'],
          'mtgStatsdTagFormat',
          <Select
            allowClear
            placeholder="datadog"
            options={['datadog', 'influxdb', 'graphite'].map((v) => ({ value: v, label: v }))}
          />,
          'mtgStatsdTagFormatHint',
        ),
      ],
    },
    {
      key: 'web',
      label: f('mtgAdvancedWeb'),
      children: [
        field(['web', 'bindTo'], 'mtgWebBindTo', text('127.0.0.1:18080'), 'mtgWebBindToHint'),
        field(['web', 'host'], 'mtgWebHost', text('proxy.example.com'), 'mtgWebHostHint'),
        field(
          ['web', 'secretMode'],
          'mtgWebSecretMode',
          <Select
            allowClear
            placeholder="dd"
            options={['dd', 'plain'].map((v) => ({ value: v, label: v }))}
          />,
          'mtgWebSecretModeHint',
        ),
        field(['web', 'decoyDir'], 'mtgWebDecoyDir', text('/var/www/decoy'), 'mtgWebDecoyDirHint'),
        field(
          ['web', 'trustedProxies'],
          'mtgWebTrustedProxies',
          tags('127.0.0.1/32'),
          'mtgWebTrustedProxiesHint',
        ),
        field(
          ['web', 'maxSessions'],
          'mtgWebMaxSessions',
          num(1, 65535, '1024'),
          'mtgWebMaxSessionsHint',
        ),
        field(
          ['web', 'maxPending'],
          'mtgWebMaxPending',
          num(1, 65535, '4096'),
          'mtgWebMaxPendingHint',
        ),
        field(['web', 'diag'], 'mtgWebDiag', <Switch />, 'mtgWebDiagHint', true),
      ],
    },
    {
      key: 'extra',
      label: f('mtgAdvancedExtra'),
      children: [
        field(
          ['extraToml'],
          'mtgExtraToml',
          <Input.TextArea
            autoSize={{ minRows: 3, maxRows: 12 }}
            placeholder={'usage-state-file = "/var/lib/mtg/usage.json"'}
            spellCheck={false}
            style={{ fontFamily: 'monospace' }}
          />,
          'mtgExtraTomlHint',
        ),
      ],
    },
  ];

  return (
    <>
      <FormField
        name={['settings', 'fakeTlsDomain']}
        label={t('pages.inbounds.form.fakeTlsDomain')}
        tooltip={t('pages.inbounds.form.mtprotoFakeTlsDomainHint')}
      >
        <Input placeholder="www.cloudflare.com" />
      </FormField>
      <FormField
        name={['settings', 'domainFronting', 'ip']}
        label={t('pages.inbounds.form.mtgDomainFrontingIp')}
        tooltip={t('pages.inbounds.form.mtgDomainFrontingHint')}
      >
        <Input placeholder="127.0.0.1" />
      </FormField>
      <FormField
        name={['settings', 'domainFronting', 'port']}
        label={t('pages.inbounds.form.mtgDomainFrontingPort')}
      >
        <InputNumber min={0} max={65535} placeholder="443" style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'domainFronting', 'proxyProtocol']}
        label={t('pages.inbounds.form.mtgDomainFrontingProxyProtocol')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'proxyProtocolListener']}
        label={t('pages.inbounds.form.mtgProxyProtocolListener')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField name={['settings', 'preferIp']} label={t('pages.inbounds.form.mtgPreferIp')}>
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
      </FormField>
      <FormField
        name={['settings', 'debug']}
        label={t('pages.inbounds.form.mtgDebug')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'throttleMaxConnections']}
        label={t('pages.inbounds.form.mtgThrottleMaxConnections')}
        tooltip={t('pages.inbounds.form.mtgThrottleMaxConnectionsHint')}
      >
        <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.mtgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.mtgRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.mtgRouteOutbound')}
          tooltip={t('pages.inbounds.form.mtgRouteOutboundHint')}
        >
          <Select
            id="mtprotoOutboundTag"
            allowClear
            showSearch
            placeholder={t('pages.inbounds.form.mtgRouteOutboundPlaceholder')}
            options={(outboundTags ?? []).map((tag) => ({ value: tag, label: tag }))}
          />
        </FormField>
      )}
      <FormField
        name={['settings', 'publicIpv4']}
        label={t('pages.inbounds.form.mtgPublicIpv4')}
        tooltip={t('pages.inbounds.form.mtgPublicIpHint')}
      >
        <Input allowClear placeholder="1.2.3.4" />
      </FormField>
      <FormField
        name={['settings', 'publicIpv6']}
        label={t('pages.inbounds.form.mtgPublicIpv6')}
        tooltip={t('pages.inbounds.form.mtgPublicIpHint')}
      >
        <Input allowClear placeholder="2001:db8::1" />
      </FormField>
      <FormField
        name={['settings', 'secured']}
        label={t('pages.inbounds.form.mtgSecured')}
        tooltip={t('pages.inbounds.form.mtgSecuredHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'dcPool', 'enabled']}
        label={t('pages.inbounds.form.mtgDcPool')}
        tooltip={t('pages.inbounds.form.mtgDcPoolHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {dcPoolEnabled && (
        <FormField
          name={['settings', 'dcPool', 'size']}
          label={t('pages.inbounds.form.mtgDcPoolSize')}
          tooltip={t('pages.inbounds.form.mtgDcPoolSizeHint')}
        >
          <InputNumber min={1} max={64} placeholder="2" style={{ width: '100%' }} />
        </FormField>
      )}
      {dcPoolEnabled &&
        field(
          ['dcPool', 'dcs'],
          'mtgDcPoolDcs',
          tags('1, 2, 3, 4, 5, -2, 203'),
          'mtgDcPoolDcsHint',
        )}
      <Collapse size="small" items={advanced} />
    </>
  );
}
