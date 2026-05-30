import { useTranslation } from 'react-i18next';
import { Form, Input } from 'antd';

import { HeaderMapEditor } from '@/components/form';

export default function HttpUpgradeForm() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        label={t('host')}
        name={['streamSettings', 'httpupgradeSettings', 'host']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('path')}
        name={['streamSettings', 'httpupgradeSettings', 'path']}
      >
        <Input />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.headers')}
        name={['streamSettings', 'httpupgradeSettings', 'headers']}
      >
        <HeaderMapEditor mode="v1" />
      </Form.Item>
    </>
  );
}
