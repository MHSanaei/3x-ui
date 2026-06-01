import { useTranslation } from 'react-i18next';
import { Form, Input, Select } from 'antd';

import { ALPN_OPTIONS, UTLS_OPTIONS } from '../outbound-form-constants';

export default function TlsForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label="SNI"
        name={['streamSettings', 'tlsSettings', 'serverName']}
      >
        <Input placeholder={t('pages.xray.outboundForm.serverNamePlaceholder')} />
      </Form.Item>
      <Form.Item
        label="uTLS"
        name={['streamSettings', 'tlsSettings', 'fingerprint']}
      >
        <Select
          allowClear
          placeholder={t('none')}
          options={UTLS_OPTIONS}
        />
      </Form.Item>
      <Form.Item
        label="ALPN"
        name={['streamSettings', 'tlsSettings', 'alpn']}
      >
        <Select mode="multiple" options={ALPN_OPTIONS} />
      </Form.Item>
      <Form.Item
        label="ECH"
        name={['streamSettings', 'tlsSettings', 'echConfigList']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.xray.outboundForm.verifyPeerName')}
        name={['streamSettings', 'tlsSettings', 'verifyPeerCertByName']}
      >
        <Input placeholder="cloudflare-dns.com" />
      </Form.Item>
      <Form.Item
        label={t('pages.xray.outboundForm.pinnedSha256')}
        name={['streamSettings', 'tlsSettings', 'pinnedPeerCertSha256']}
      >
        <Input placeholder="base64 SHA256" />
      </Form.Item>
    </>
  );
}
