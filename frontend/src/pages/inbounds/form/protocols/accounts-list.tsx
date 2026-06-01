import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Space } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

import { RandomUtil } from '@/utils';
import { InputAddon } from '@/components/ui';

export default function AccountsList() {
  const { t } = useTranslation();
  return (
    <Form.List name={['settings', 'accounts']}>
      {(fields, { add, remove }) => (
        <>
          <Form.Item label={t('pages.inbounds.form.accounts')}>
            <Button
              size="small"
              onClick={() => add({
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
                <Space.Compact key={field.key} className="mb-8" block>
                  <InputAddon>{String(idx + 1)}</InputAddon>
                  <Form.Item name={[field.name, 'user']} noStyle>
                    <Input placeholder={t('username')} />
                  </Form.Item>
                  <Form.Item name={[field.name, 'pass']} noStyle>
                    <Input placeholder={t('password')} />
                  </Form.Item>
                  <Button onClick={() => remove(field.name)}>
                    <MinusOutlined />
                  </Button>
                </Space.Compact>
              ))}
            </Form.Item>
          )}
        </>
      )}
    </Form.List>
  );
}
