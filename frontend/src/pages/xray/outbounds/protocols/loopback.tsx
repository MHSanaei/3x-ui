import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

import SniffingFields from '@/lib/xray/forms/SniffingFields';

export default function LoopbackFields() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();

  return (
    <>
      <Form.Item label={t('pages.xray.outboundForm.inboundTag')} name={['settings', 'inboundTag']}>
        <Input placeholder={t('pages.xray.outboundForm.inboundTagPlaceholder')} />
      </Form.Item>

      <SniffingFields
        name={['settings', 'sniffing']}
        form={form}
        enableLabel={t('pages.inbounds.sniffingTab')}
      />
    </>
  );
}
