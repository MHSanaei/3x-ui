import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Space, Tooltip } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

export default function TunFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['settings', 'name']} label={t('pages.inbounds.info.interfaceName')}>
        <Input placeholder="xray0" />
      </Form.Item>
      <Form.Item name={['settings', 'mtu']} label="MTU">
        <InputNumber min={0} />
      </Form.Item>
      <Form.List name={['settings', 'gateway']}>
        {(fields, { add, remove }) => (
          <Form.Item label={t('pages.inbounds.info.gateway')}>
            <Button aria-label={t('add')} size="small" onClick={() => add('')}>
              <PlusOutlined />
            </Button>
            {fields.map((field, j) => (
              <Space.Compact key={field.key} block className="mt-4">
                <Form.Item name={field.name} noStyle>
                  <Input placeholder={j === 0 ? '10.0.0.1/16' : 'fc00::1/64'} />
                </Form.Item>
                <Button aria-label={t('remove')} size="small" onClick={() => remove(field.name)}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
        )}
      </Form.List>
      <Form.List name={['settings', 'dns']}>
        {(fields, { add, remove }) => (
          <Form.Item label="DNS">
            <Button aria-label={t('add')} size="small" onClick={() => add('')}>
              <PlusOutlined />
            </Button>
            {fields.map((field, j) => (
              <Space.Compact key={field.key} block className="mt-4">
                <Form.Item name={field.name} noStyle>
                  <Input placeholder={j === 0 ? '1.1.1.1' : '8.8.8.8'} />
                </Form.Item>
                <Button aria-label={t('remove')} size="small" onClick={() => remove(field.name)}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
        )}
      </Form.List>
      <Form.Item name={['settings', 'userLevel']} label={t('pages.xray.tun.userLevel')}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.List name={['settings', 'autoSystemRoutingTable']}>
        {(fields, { add, remove }) => (
          <Form.Item
            label={
              <Tooltip title={t('pages.inbounds.form.autoSystemRoutesTooltip')}>
                {t('pages.inbounds.info.autoSystemRoutes')}
              </Tooltip>
            }
          >
            <Button aria-label={t('add')} size="small" onClick={() => add('')}>
              <PlusOutlined />
            </Button>
            {fields.map((field, j) => (
              <Space.Compact key={field.key} block className="mt-4">
                <Form.Item name={field.name} noStyle>
                  <Input placeholder={j === 0 ? '0.0.0.0/0' : '::/0'} />
                </Form.Item>
                <Button aria-label={t('remove')} size="small" onClick={() => remove(field.name)}>
                  <MinusOutlined />
                </Button>
              </Space.Compact>
            ))}
          </Form.Item>
        )}
      </Form.List>
      <Form.Item
        name={['settings', 'autoOutboundsInterface']}
        label={
          <Tooltip title={t('pages.inbounds.form.autoOutboundsInterfaceTooltip')}>
            {t('pages.inbounds.form.autoOutboundsInterface')}
          </Tooltip>
        }
      >
        <Input placeholder="auto" />
      </Form.Item>
    </>
  );
}
