import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, type FormInstance } from 'antd';

import { HysteriaMasqueradeForm } from '@/lib/xray/forms/transport';
import type { OutboundFormValues } from '@/schemas/forms/outbound-form';

export default function HysteriaForm({ form }: { form: FormInstance<OutboundFormValues> }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('pages.inbounds.form.version')}
        name={['streamSettings', 'hysteriaSettings', 'version']}
      >
        <InputNumber min={2} max={2} disabled style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item
        label={t('pages.xray.outboundForm.authPassword')}
        name={['streamSettings', 'hysteriaSettings', 'auth']}
      >
        <Input />
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
