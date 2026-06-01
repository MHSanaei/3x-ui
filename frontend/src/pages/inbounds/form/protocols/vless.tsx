import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Space, Typography } from 'antd';

interface VlessFieldsProps {
  saving: boolean;
  selectedVlessAuth: string;
  network: string;
  security: string;
  getNewVlessEnc: (kind: 'x25519' | 'mlkem768') => void;
  clearVlessEnc: () => void;
}

export default function VlessFields({
  saving,
  selectedVlessAuth,
  network,
  security,
  getNewVlessEnc,
  clearVlessEnc,
}: VlessFieldsProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['settings', 'decryption']} label={t('pages.inbounds.decryption')}>
        <Input />
      </Form.Item>
      <Form.Item name={['settings', 'encryption']} label={t('pages.inbounds.encryption')}>
        <Input />
      </Form.Item>
      <Form.Item label=" ">
        <Space size={8} wrap>
          <Button type="primary" loading={saving} onClick={() => getNewVlessEnc('x25519')}>
            {t('pages.inbounds.vlessAuthX25519')}
          </Button>
          <Button type="primary" loading={saving} onClick={() => getNewVlessEnc('mlkem768')}>
            {t('pages.inbounds.vlessAuthMlkem768')}
          </Button>
          <Button danger onClick={clearVlessEnc}>{t('clear')}</Button>
        </Space>
        <Typography.Text type="secondary" className="vless-auth-state">
          {t('pages.inbounds.vlessAuthSelected', { auth: selectedVlessAuth })}
        </Typography.Text>
      </Form.Item>
      {network === 'tcp' && (security === 'tls' || security === 'reality') && (
        <Form.Item
          label={t('pages.inbounds.form.visionTestseed')}
          extra="Applies only to clients using the xtls-rprx-vision flow; ignored otherwise."
        >
          <Space.Compact block>
            {[900, 500, 900, 256].map((def, i) => (
              <Form.Item key={i} name={['settings', 'testseed', i]} noStyle initialValue={def}>
                <InputNumber min={1} style={{ width: '25%' }} />
              </Form.Item>
            ))}
          </Space.Compact>
        </Form.Item>
      )}
    </>
  );
}
