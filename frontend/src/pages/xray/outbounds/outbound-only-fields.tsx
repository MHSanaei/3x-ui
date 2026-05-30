import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { DNSRuleActions } from '@/schemas/primitives';

export function OutboundOnlyProtocolFields({ protocol }: { protocol: string }) {
  const { t } = useTranslation();
  return (
    <>
      {protocol === 'loopback' && (
        <Form.Item label={t('pages.xray.outboundForm.inboundTag')} name={['settings', 'inboundTag']}>
          <Input placeholder={t('pages.xray.outboundForm.inboundTagPlaceholder')} />
        </Form.Item>
      )}

      {protocol === 'blackhole' && (
        <Form.Item label={t('pages.xray.outboundForm.responseType')} name={['settings', 'type']}>
          <Select
            options={[
              { value: '', label: '(empty)' },
              { value: 'none', label: 'none' },
              { value: 'http', label: 'http' },
            ]}
          />
        </Form.Item>
      )}

      {protocol === 'dns' && (
        <>
          <Form.Item label={t('pages.xray.outboundForm.rewriteNetwork')} name={['settings', 'rewriteNetwork']}>
            <Select
              allowClear
              placeholder={t('pages.xray.outboundForm.unchanged')}
              options={[
                { value: 'udp', label: 'udp' },
                { value: 'tcp', label: 'tcp' },
              ]}
            />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.form.rewriteAddress')} name={['settings', 'rewriteAddress']}>
            <Input placeholder={t('pages.xray.outboundForm.unchangedAddress')} />
          </Form.Item>
          <Form.Item label={t('pages.inbounds.form.rewritePort')} name={['settings', 'rewritePort']}>
            <InputNumber min={0} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('pages.xray.tun.userLevel')} name={['settings', 'userLevel']}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.List name={['settings', 'rules']}>
            {(fields, { add, remove }) => (
              <>
                <Form.Item label={t('pages.xray.outboundForm.rules')}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<PlusOutlined />}
                    onClick={() => add({ action: 'direct', qtype: '', domain: '' })}
                  />
                </Form.Item>
                {fields.map((field, index) => (
                  <div key={field.key}>
                    <Form.Item wrapperCol={{ md: { span: 14, offset: 8 } }}>
                      <div className="item-heading">
                        <span>{t('pages.xray.outboundForm.ruleN', { n: index + 1 })}</span>
                        <DeleteOutlined
                          className="danger-icon"
                          onClick={() => remove(field.name)}
                        />
                      </div>
                    </Form.Item>
                    <Form.Item label={t('pages.xray.outboundForm.action')} name={[field.name, 'action']}>
                      <Select
                        options={DNSRuleActions.map((a) => ({ value: a, label: a }))}
                      />
                    </Form.Item>
                    <Form.Item label="QType" name={[field.name, 'qtype']}>
                      <Input placeholder="1,3,23-24" />
                    </Form.Item>
                    <Form.Item label={t('domainName')} name={[field.name, 'domain']}>
                      <Input placeholder="domain:example.com" />
                    </Form.Item>
                  </div>
                ))}
              </>
            )}
          </Form.List>
        </>
      )}
    </>
  );
}
