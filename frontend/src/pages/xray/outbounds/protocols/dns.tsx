import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { activateOnKey } from '@/utils/a11y';
import { DNSRuleActions } from '@/schemas/primitives';

export default function DnsFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: 'settings.rules' });
  return (
    <>
      <FormField label={t('pages.xray.outboundForm.rewriteNetwork')} name={['settings', 'rewriteNetwork']}>
        <Select
          allowClear
          placeholder={t('pages.xray.outboundForm.unchanged')}
          options={[
            { value: 'udp', label: 'udp' },
            { value: 'tcp', label: 'tcp' },
          ]}
        />
      </FormField>
      <FormField label={t('pages.inbounds.form.rewriteAddress')} name={['settings', 'rewriteAddress']}>
        <Input placeholder={t('pages.xray.outboundForm.unchangedAddress')} />
      </FormField>
      <FormField label={t('pages.inbounds.form.rewritePort')} name={['settings', 'rewritePort']}>
        <InputNumber min={0} max={65535} style={{ width: '100%' }} />
      </FormField>
      <FormField label={t('pages.xray.tun.userLevel')} name={['settings', 'userLevel']}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </FormField>
      <Form.Item label={t('pages.xray.outboundForm.rules')}>
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          aria-label={t('add')}
          onClick={() => append({ action: 'direct', qType: '', domain: '', rCode: 0 })}
        />
      </Form.Item>
      {fields.map((field, index) => (
        <div key={field.id}>
          <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
            <div className="item-heading">
              <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
              <DeleteOutlined
                className="danger-icon"
                role="button"
                tabIndex={0}
                aria-label={t('remove')}
                onClick={() => remove(index)}
                onKeyDown={activateOnKey(() => remove(index))}
              />
            </div>
          </Form.Item>
          <FormField label={t('pages.xray.outboundForm.action')} name={['settings', 'rules', index, 'action']}>
            <Select
              options={DNSRuleActions.map((a) => ({ value: a, label: a }))}
            />
          </FormField>
          <FormField label="QType" name={['settings', 'rules', index, 'qType']}>
            <Input placeholder="1,3,23-24" />
          </FormField>
          <FormField label={t('domainName')} name={['settings', 'rules', index, 'domain']}>
            <Input placeholder="domain:example.com" />
          </FormField>
          <FormField label="RCode" name={['settings', 'rules', index, 'rCode']}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} />
          </FormField>
        </div>
      ))}
    </>
  );
}
