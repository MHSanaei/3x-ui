import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  AutoComplete,
  Button,
  Collapse,
  Divider,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  message,
} from 'antd';
import { CloudDownloadOutlined, SyncOutlined } from '@ant-design/icons';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { HttpUtil } from '@/utils';

export default function TuicFields() {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const { control, setValue } = useFormContext();
  const [loadingPanelCert, setLoadingPanelCert] = useState(false);

  const sni = (useWatch({ control, name: 'settings.server.sni' }) ?? '') as string;
  const certificate = (useWatch({ control, name: 'settings.server.certificate' }) ?? '') as string;
  const privateKey = (useWatch({ control, name: 'settings.server.private_key' }) ?? '') as string;
  const nodeId = useWatch({ control, name: 'nodeId' }) as number | null | undefined;

  const handleSniChange = (newSni: string) => {
    setValue('settings.server.sni', newSni);
  };

  const autofillFromSni = () => {
    const cleanSni = (sni || '').trim();
    if (!cleanSni) {
      messageApi.warning(t('pages.xray.tuic.sniRequired'));
      return;
    }
    setValue('settings.server.certificate', `/root/cert/${cleanSni}/fullchain.pem`);
    setValue('settings.server.private_key', `/root/cert/${cleanSni}/privkey.pem`);
  };

  const setCertFromPanel = async () => {
    setLoadingPanelCert(true);
    try {
      const msg =
        typeof nodeId === 'number'
          ? await HttpUtil.get(`/panel/api/nodes/webCert/${nodeId}`, undefined, { silent: true })
          : await HttpUtil.post('/panel/api/setting/all', undefined, { silent: true });
      if (!msg?.success) {
        messageApi.warning(msg?.msg || t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      const obj = msg.obj as { webCertFile?: string; webKeyFile?: string };
      if (!obj?.webCertFile && !obj?.webKeyFile) {
        messageApi.warning(t('pages.inbounds.setDefaultCertEmpty'));
        return;
      }
      if (obj.webCertFile) {
        setValue('settings.server.certificate', obj.webCertFile);
      }
      if (obj.webKeyFile) {
        setValue('settings.server.private_key', obj.webKeyFile);
      }
      messageApi.success(t('pages.inbounds.setSuccess'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    } finally {
      setLoadingPanelCert(false);
    }
  };

  const certOptions = sni
    ? [
        { value: `/root/cert/${sni}/fullchain.pem` },
        { value: `/etc/letsencrypt/live/${sni}/fullchain.pem` },
        { value: '/root/cert.pem' },
      ]
    : [{ value: '/root/cert.pem' }];

  const keyOptions = sni
    ? [
        { value: `/root/cert/${sni}/privkey.pem` },
        { value: `/etc/letsencrypt/live/${sni}/privkey.pem` },
        { value: '/root/privkey.pem' },
      ]
    : [{ value: '/root/privkey.pem' }];

  const advancedItems = [
    {
      key: 'advanced',
      label: t('pages.inbounds.advancedTitle'),
      children: (
        <>
          <FormField
            name={['settings', 'server', 'zero_rtt_handshake']}
            label={t('pages.xray.tuic.zeroRttHandshake')}
            valueProp="checked"
          >
            <Switch />
          </FormField>

          <FormField
            name={['settings', 'server', 'log_level']}
            label={t('pages.xray.tuic.logLevel')}
            tooltip={t('pages.xray.tuic.logLevelHint')}
          >
            <Select
              options={[
                { label: 'Info', value: 'info' },
                { label: 'Warn', value: 'warn' },
                { label: 'Error', value: 'error' },
                { label: 'Debug', value: 'debug' },
              ]}
            />
          </FormField>

          <FormField
            name={['settings', 'server', 'max_idle_time']}
            label={t('pages.xray.tuic.maxIdleTime')}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>

          <FormField
            name={['settings', 'server', 'authentication_timeout']}
            label={t('pages.xray.tuic.authTimeout')}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>

          <FormField
            name={['settings', 'server', 'max_udp_relay_packet_size']}
            label={t('pages.xray.tuic.maxUdpRelayPacketSize')}
          >
            <InputNumber min={1} max={65245} style={{ width: '100%' }} />
          </FormField>
        </>
      ),
    },
  ];

  return (
    <>
      {messageContextHolder}
      <Form.Item label={t('pages.inbounds.publicKey')}>
        <AutoComplete
          value={certificate}
          options={certOptions}
          onChange={(v) => setValue('settings.server.certificate', v)}
          placeholder="/root/cert.pem"
        />
      </Form.Item>

      <Form.Item label={t('pages.inbounds.privatekey')}>
        <AutoComplete
          value={privateKey}
          options={keyOptions}
          onChange={(v) => setValue('settings.server.private_key', v)}
          placeholder="/root/privkey.pem"
        />
      </Form.Item>

      <Form.Item label=" ">
        <Space wrap style={{ display: 'flex', flexWrap: 'wrap', width: '100%' }}>
          <Button
            type="primary"
            icon={<CloudDownloadOutlined />}
            loading={loadingPanelCert}
            onClick={setCertFromPanel}
          >
            {t('pages.inbounds.setDefaultCert')}
          </Button>
          <Button icon={<SyncOutlined />} onClick={autofillFromSni}>
            {t('pages.inbounds.form.autoFill')}
          </Button>
          <Button
            danger
            onClick={() => {
              setValue('settings.server.certificate', '');
              setValue('settings.server.private_key', '');
            }}
          >
            {t('clear')}
          </Button>
        </Space>
      </Form.Item>

      <FormField
        name={['settings', 'server', 'congestion_control']}
        label={t('pages.xray.tuic.congestionControl')}
        tooltip={t('pages.xray.tuic.congestionControlHint')}
      >
        <Select
          options={[
            { label: 'BBR', value: 'bbr' },
            { label: 'CUBIC', value: 'cubic' },
            { label: 'New Reno', value: 'new_reno' },
          ]}
        />
      </FormField>

      <FormField name={['settings', 'server', 'alpn']} label={t('pages.xray.tuic.alpn')}>
        <Select
          mode="tags"
          tokenSeparators={[',']}
          options={[
            { label: 'h3', value: 'h3' },
            { label: 'spdy/3.1', value: 'spdy/3.1' },
          ]}
        />
      </FormField>

      <Divider titlePlacement="start">{t('pages.xray.tuic.profileOptions')}</Divider>

      <Form.Item label={t('pages.xray.tuic.sni')} tooltip={t('pages.xray.tuic.sniHint')}>
        <Input
          value={sni}
          placeholder="example.com"
          onChange={(e) => handleSniChange(e.target.value)}
        />
      </Form.Item>

      <FormField
        name={['settings', 'server', 'udp_relay_mode']}
        label={t('pages.xray.tuic.udpRelayMode')}
        tooltip={t('pages.xray.tuic.udpRelayModeHint')}
      >
        <Select
          options={[
            { label: 'Native (Recommended)', value: 'native' },
            { label: 'QUIC', value: 'quic' },
          ]}
        />
      </FormField>

      <Collapse style={{ marginTop: 16, marginBottom: 8 }} items={advancedItems} />
    </>
  );
}
