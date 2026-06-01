import { useTranslation } from 'react-i18next';
import { Form, InputNumber } from 'antd';

export default function KcpForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['streamSettings', 'kcpSettings', 'mtu']} label="MTU">
        <InputNumber min={576} max={1460} />
      </Form.Item>
      <Form.Item name={['streamSettings', 'kcpSettings', 'tti']} label={t('pages.inbounds.form.ttiMs')}>
        <InputNumber min={10} max={100} />
      </Form.Item>
      <Form.Item name={['streamSettings', 'kcpSettings', 'uplinkCapacity']} label={t('pages.inbounds.form.uplinkMbps')}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item name={['streamSettings', 'kcpSettings', 'downlinkCapacity']} label={t('pages.inbounds.form.downlinkMbps')}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
        label={t('pages.inbounds.form.cwndMultiplier')}
      >
        <InputNumber min={1} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
        label={t('pages.inbounds.form.maxSendingWindow')}
      >
        <InputNumber min={0} />
      </Form.Item>
    </>
  );
}
