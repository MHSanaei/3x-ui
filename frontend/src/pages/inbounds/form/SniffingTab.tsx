import { useTranslation } from 'react-i18next';
import { Form } from 'antd';

import SniffingFields from '@/lib/xray/forms/SniffingFields';

export default function SniffingTab() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  return (
    <SniffingFields
      name={['sniffing']}
      form={form}
      enableLabel={t('enable')}
    />
  );
}
