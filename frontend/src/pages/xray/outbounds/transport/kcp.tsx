import { useTranslation } from 'react-i18next';
import { Form, InputNumber } from 'antd';

export default function KcpForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label="MTU" name={['streamSettings', 'kcpSettings', 'mtu']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.form.ttiMs')} name={['streamSettings', 'kcpSettings', 'tti']}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.uplinkMbps')}
        name={['streamSettings', 'kcpSettings', 'uplinkCapacity']}
      >
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.downlinkMbps')}
        name={['streamSettings', 'kcpSettings', 'downlinkCapacity']}
      >
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.cwndMultiplier')}
        name={['streamSettings', 'kcpSettings', 'cwndMultiplier']}
      >
        <InputNumber min={1} />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.maxSendingWindow')}
        name={['streamSettings', 'kcpSettings', 'maxSendingWindow']}
      >
        <InputNumber min={0} />
      </Form.Item>
    </>
  );
}
