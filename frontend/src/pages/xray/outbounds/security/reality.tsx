import { useTranslation } from 'react-i18next';
import { Input, Select } from 'antd';

import { FormField } from '@/components/form/rhf';

import { UTLS_OPTIONS } from '../outbound-form-constants';

export default function RealityForm() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        label="SNI"
        name={['streamSettings', 'realitySettings', 'serverName']}
      >
        <Input />
      </FormField>
      <FormField
        label="uTLS"
        name={['streamSettings', 'realitySettings', 'fingerprint']}
      >
        <Select options={UTLS_OPTIONS} />
      </FormField>
      <FormField
        label={t('pages.xray.outboundForm.shortId')}
        name={['streamSettings', 'realitySettings', 'shortId']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.spiderX')}
        name={['streamSettings', 'realitySettings', 'spiderX']}
      >
        <Input />
      </FormField>
      <FormField
        label={t('pages.inbounds.publicKey')}
        name={['streamSettings', 'realitySettings', 'publicKey']}
      >
        <Input.TextArea autoSize={{ minRows: 2 }} />
      </FormField>
      <FormField
        label={t('pages.inbounds.form.mldsa65Verify')}
        name={['streamSettings', 'realitySettings', 'mldsa65Verify']}
      >
        <Input.TextArea autoSize={{ minRows: 2 }} />
      </FormField>
    </>
  );
}
