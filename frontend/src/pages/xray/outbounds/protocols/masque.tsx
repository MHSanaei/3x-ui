import { useTranslation } from 'react-i18next';
import { Select } from 'antd';

import { selectTagRender } from '@/components/form';
import { FormField } from '@/components/form/rhf';

export default function MasqueFields() {
  const { t } = useTranslation();
  return (
    <FormField label={t('pages.xray.outboundForm.remoteDNS')} name={['settings', 'remoteDNS']}>
      <Select
        mode="tags"
        tagRender={selectTagRender}
        tokenSeparators={[',', ' ']}
        open={false}
        placeholder="1.1.1.1, 2606:4700:4700::1111"
      />
    </FormField>
  );
}
