import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function AmneziawgFields() {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('pages.xray.amneziawg.serverPrivateKey')}>
        <FormField name={['settings', 'server', 'privateKey']} noStyle>
          <Input />
        </FormField>
      </Form.Item>
      <FormField name={['settings', 'server', 'publicKey']} label={t('pages.xray.amneziawg.serverPublicKey')}>
        <Input disabled />
      </FormField>
      <FormField name={['settings', 'server', 'psk']} label={t('pages.xray.amneziawg.serverPsk')}>
        <Input />
      </FormField>

      <Form.Item label={t('pages.xray.amneziawg.obfuscationParams')}>
        <Input.Group compact>
          <FormField name={['settings', 'server', 'jc']} noStyle>
            <InputNumber addonBefore="Jc" style={{ width: 120 }} />
          </FormField>
          <FormField name={['settings', 'server', 'jmin']} noStyle>
            <InputNumber addonBefore="Jmin" style={{ width: 120 }} />
          </FormField>
          <FormField name={['settings', 'server', 'jmax']} noStyle>
            <InputNumber addonBefore="Jmax" style={{ width: 120 }} />
          </FormField>
        </Input.Group>
      </Form.Item>

      <Form.Item label={t('pages.xray.amneziawg.junkPacketSizes')}>
        <Input.Group compact>
          <FormField name={['settings', 'server', 's1']} noStyle>
            <InputNumber addonBefore="S1" style={{ width: 120 }} />
          </FormField>
          <FormField name={['settings', 'server', 's2']} noStyle>
            <InputNumber addonBefore="S2" style={{ width: 120 }} />
          </FormField>
          <FormField name={['settings', 'server', 's3']} noStyle>
            <InputNumber addonBefore="S3" style={{ width: 120 }} />
          </FormField>
          <FormField name={['settings', 'server', 's4']} noStyle>
            <InputNumber addonBefore="S4" style={{ width: 120 }} />
          </FormField>
        </Input.Group>
      </Form.Item>

      <Form.Item label={t('pages.xray.amneziawg.magicHeaders')}>
        <FormField name={['settings', 'server', 'h1']} noStyle>
          <Input addonBefore="H1" style={{ width: '100%', marginBottom: 4 }} />
        </FormField>
        <FormField name={['settings', 'server', 'h2']} noStyle>
          <Input addonBefore="H2" style={{ width: '100%', marginBottom: 4 }} />
        </FormField>
        <FormField name={['settings', 'server', 'h3']} noStyle>
          <Input addonBefore="H3" style={{ width: '100%', marginBottom: 4 }} />
        </FormField>
        <FormField name={['settings', 'server', 'h4']} noStyle>
          <Input addonBefore="H4" style={{ width: '100%' }} />
        </FormField>
      </Form.Item>

      <Form.Item label={t('pages.xray.amneziawg.subnet')}>
        <Input.Group compact>
          <FormField name={['settings', 'server', 'subnetIp']} noStyle>
            <Input addonBefore={t('pages.xray.amneziawg.ip')} style={{ width: 180 }} />
          </FormField>
          <FormField name={['settings', 'server', 'subnetCidr']} noStyle>
            <InputNumber addonBefore="/" style={{ width: 100 }} min={16} max={30} />
          </FormField>
        </Input.Group>
      </Form.Item>

      <Form.Item label={t('pages.inbounds.info.dns')}>
        <Input.Group compact>
          <FormField name={['settings', 'server', 'primaryDns']} noStyle>
            <Input addonBefore={t('pages.xray.amneziawg.primary')} style={{ width: 200 }} />
          </FormField>
          <FormField name={['settings', 'server', 'secondaryDns']} noStyle>
            <Input addonBefore={t('pages.xray.amneziawg.secondary')} style={{ width: 200 }} />
          </FormField>
        </Input.Group>
      </Form.Item>
    </>
  );
}
