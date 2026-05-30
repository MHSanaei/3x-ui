import { useTranslation } from 'react-i18next';
import { Form, Input, Select } from 'antd';

import { UTLS_OPTIONS } from '../outbound-form-constants';

export default function RealityForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label="SNI"
        name={['streamSettings', 'realitySettings', 'serverName']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label="uTLS"
        name={['streamSettings', 'realitySettings', 'fingerprint']}
      >
        <Select options={UTLS_OPTIONS} />
      </Form.Item>
      <Form.Item
        label={t('pages.xray.outboundForm.shortId')}
        name={['streamSettings', 'realitySettings', 'shortId']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.spiderX')}
        name={['streamSettings', 'realitySettings', 'spiderX']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.publicKey')}
        name={['streamSettings', 'realitySettings', 'publicKey']}
      >
        <Input.TextArea autoSize={{ minRows: 2 }} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.mldsa65Verify')}
        name={['streamSettings', 'realitySettings', 'mldsa65Verify']}
      >
        <Input.TextArea autoSize={{ minRows: 2 }} />
      </Form.Item>
    </>
  );
}
