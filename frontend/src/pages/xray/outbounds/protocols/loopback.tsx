import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

export default function LoopbackFields() {
  const { t } = useTranslation();
  return (
    <Form.Item label={t('pages.xray.outboundForm.inboundTag')} name={['settings', 'inboundTag']}>
      <Input placeholder={t('pages.xray.outboundForm.inboundTagPlaceholder')} />
    </Form.Item>
  );
}
