import { useTranslation } from 'react-i18next';
import { Form, Select } from 'antd';

export default function BlackholeFields() {
  const { t } = useTranslation();
  return (
    <Form.Item label={t('pages.xray.outboundForm.responseType')} name={['settings', 'type']}>
      <Select
        options={[
          { value: '', label: '(empty)' },
          { value: 'none', label: 'none' },
          { value: 'http', label: 'http' },
        ]}
      />
    </Form.Item>
  );
}
