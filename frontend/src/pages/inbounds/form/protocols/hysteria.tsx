import { useTranslation } from 'react-i18next';
import { Form, InputNumber, type FormInstance } from 'antd';

import { HysteriaMasqueradeForm } from '@/lib/xray/forms/transport';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';

export default function HysteriaFields({ form }: { form: FormInstance<InboundFormValues> }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('pages.inbounds.form.version')}
        name={['streamSettings', 'hysteriaSettings', 'version']}
      >
        <InputNumber min={2} max={2} disabled />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.udpIdleTimeout')}
        name={['streamSettings', 'hysteriaSettings', 'udpIdleTimeout']}
      >
        <InputNumber min={1} style={{ width: '100%' }} />
      </Form.Item>

      <HysteriaMasqueradeForm form={form} />
    </>
  );
}
