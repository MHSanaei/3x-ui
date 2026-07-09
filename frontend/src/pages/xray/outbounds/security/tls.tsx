import { useTranslation } from 'react-i18next';
import { Input, Select } from 'antd';

import { FormField } from '@/components/form/rhf';

import { ALPN_OPTIONS, UTLS_OPTIONS } from '../outbound-form-constants';

export default function TlsForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label="SNI"
        name={['streamSettings', 'tlsSettings', 'serverName']}
      >
        <Input placeholder={t('pages.xray.outboundForm.serverNamePlaceholder')} />
      </FormField>
      <FormField
        label="uTLS"
        name={['streamSettings', 'tlsSettings', 'fingerprint']}
      >
        <Select
          allowClear
          placeholder={t('none')}
          options={[{ value: '', label: t('none') }, ...UTLS_OPTIONS]}
        />
      </FormField>
      <FormField
        label="ALPN"
        name={['streamSettings', 'tlsSettings', 'alpn']}
      >
        <Select mode="multiple" options={ALPN_OPTIONS} />
      </FormField>
      <FormField
        label="ECH"
        name={['streamSettings', 'tlsSettings', 'echConfigList']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.xray.outboundForm.verifyPeerName')}
        name={['streamSettings', 'tlsSettings', 'verifyPeerCertByName']}
      >
        <Input placeholder="cloudflare-dns.com" />
      </FormField>
      <FormField
        label={t('pages.xray.outboundForm.pinnedSha256')}
        name={['streamSettings', 'tlsSettings', 'pinnedPeerCertSha256']}
      >
        <Input placeholder="base64 SHA256" />
      </FormField>
    </>
  );
}
