import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Space } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext } from 'react-hook-form';

import { RandomUtil } from '@/utils';
import { InputAddon } from '@/components/ui';
import { FormField } from '@/components/form/rhf';

export default function AccountsList() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: 'settings.accounts' });
  return (
    <>
      <Form.Item label={t('pages.inbounds.form.accounts')}>
        <Button
          size="small"
          onClick={() => append({
            user: RandomUtil.randomLowerAndNum(8),
            pass: RandomUtil.randomLowerAndNum(12),
          })}
        >
          <PlusOutlined /> {t('add')}
        </Button>
      </Form.Item>
      {fields.length > 0 && (
        <Form.Item wrapperCol={{ span: 24 }}>
          {fields.map((field, idx) => (
            <Space.Compact key={field.id} className="mb-8" block>
              <InputAddon>{String(idx + 1)}</InputAddon>
              <FormField name={['settings', 'accounts', idx, 'user']} noStyle>
                <Input placeholder={t('username')} />
              </FormField>
              <FormField name={['settings', 'accounts', idx, 'pass']} noStyle>
                <Input placeholder={t('password')} />
              </FormField>
              <Button aria-label={t('remove')} onClick={() => remove(idx)}>
                <MinusOutlined />
              </Button>
            </Space.Compact>
          ))}
        </Form.Item>
      )}
    </>
  );
}
