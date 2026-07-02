import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Typography } from 'antd';

import { VLESS_AUTH_LABEL_KEYS, type VlessAuthKind } from '@/lib/xray/vless-encryption';

interface VlessFieldsProps {
  saving: boolean;
  selectedVlessAuth: string;
  vlessAuthKind: VlessAuthKind | null;
  network: string;
  security: string;
  getNewVlessEnc: (kind: VlessAuthKind) => void;
  clearVlessEnc: () => void;
}

export default function VlessFields({
  saving,
  selectedVlessAuth,
  vlessAuthKind,
  network,
  security,
  getNewVlessEnc,
  clearVlessEnc,
}: VlessFieldsProps) {
  const { t } = useTranslation();
  const [authKind, setAuthKind] = useState<VlessAuthKind>(vlessAuthKind ?? 'x25519');

  useEffect(() => {
    setAuthKind(vlessAuthKind ?? 'x25519');
  }, [vlessAuthKind]);

  const authOptions = (Object.entries(VLESS_AUTH_LABEL_KEYS) as [VlessAuthKind, string][]).map(
    ([value, labelKey]) => ({ value, label: t(labelKey) }),
  );

  return (
    <>
      <Form.Item name={['settings', 'decryption']} label={t('pages.inbounds.decryption')}>
        <Input />
      </Form.Item>
      <Form.Item name={['settings', 'encryption']} label={t('pages.inbounds.encryption')}>
        <Input />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.vlessAuthGenerate')}>
        <Space size={8} wrap>
          <Select
            value={authKind}
            onChange={(v) => setAuthKind(v)}
            options={authOptions}
            style={{ width: 240 }}
          />
          <Button type="primary" loading={saving} onClick={() => getNewVlessEnc(authKind)}>
            {t('pages.inbounds.vlessAuthGenerateButton')}
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
