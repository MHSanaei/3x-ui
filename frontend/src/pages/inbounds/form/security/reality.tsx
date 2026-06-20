import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { UTLS_FINGERPRINT } from '@/schemas/primitives';
import { validateRealityTarget } from '@/lib/xray/stream-wire-normalize';

interface RealityFormProps {
  saving: boolean;
  randomizeRealityTarget: () => void;
  randomizeShortIds: () => void;
  genRealityKeypair: () => void;
  clearRealityKeypair: () => void;
  genMldsa65: () => void;
  clearMldsa65: () => void;
}

export default function RealityForm({
  saving,
  randomizeRealityTarget,
  randomizeShortIds,
  genRealityKeypair,
  clearRealityKeypair,
  genMldsa65,
  clearMldsa65,
}: RealityFormProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'show']}
        label={t('pages.inbounds.form.show')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item name={['streamSettings', 'realitySettings', 'xver']} label={t('pages.inbounds.form.xver')}>
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'settings', 'fingerprint']}
        label="uTLS"
      >
        <Select
          options={Object.values(UTLS_FINGERPRINT).map((fp) => ({ value: fp, label: fp }))}
        />
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.target')}
        tooltip={t('pages.inbounds.form.realityTargetHint')}
      >
        <Space.Compact block>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'target']}
            noStyle
            rules={[
              {
                validator: async (_, value) => {
                  const errKey = validateRealityTarget(typeof value === 'string' ? value : '');
                  if (errKey) throw new Error(t(errKey));
                },
              },
            ]}
          >
            <Input style={{ width: 'calc(100% - 32px)' }} placeholder="example.com:443" />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={randomizeRealityTarget} />
        </Space.Compact>
      </Form.Item>
      <Form.Item label="SNI">
        <Space.Compact block style={{ display: 'flex' }}>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'serverNames']}
            noStyle
          >
            <Select mode="tags" tokenSeparators={[',']} style={{ flex: 1 }} />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={randomizeRealityTarget} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'maxTimediff']}
        label={t('pages.inbounds.form.maxTimeDiff')}
      >
        <InputNumber min={0} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'minClientVer']}
        label={t('pages.inbounds.form.minClientVer')}
      >
        <Input placeholder="25.9.11" />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'maxClientVer']}
        label={t('pages.inbounds.form.maxClientVer')}
      >
        <Input placeholder="25.9.11" />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.form.shortIds')}>
        <Space.Compact block style={{ display: 'flex' }}>
          <Form.Item
            name={['streamSettings', 'realitySettings', 'shortIds']}
            noStyle
          >
            <Select mode="tags" tokenSeparators={[',']} style={{ flex: 1 }} />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={randomizeShortIds} />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'settings', 'spiderX']}
        label={t('pages.inbounds.form.spiderX')}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'settings', 'publicKey']}
        label={t('pages.inbounds.publicKey')}
      >
        <Input.TextArea autoSize={{ minRows: 1, maxRows: 4 }} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'privateKey']}
        label={t('pages.inbounds.privatekey')}
      >
        <Input.TextArea autoSize={{ minRows: 1, maxRows: 4 }} />
      </Form.Item>
      <Form.Item label=" ">
        <Space>
          <Button type="primary" loading={saving} onClick={genRealityKeypair}>
            {t('pages.inbounds.form.getNewCert')}
          </Button>
          <Button danger onClick={clearRealityKeypair}>{t('clear')}</Button>
        </Space>
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'mldsa65Seed']}
        label={t('pages.inbounds.form.mldsa65Seed')}
      >
        <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'realitySettings', 'settings', 'mldsa65Verify']}
        label={t('pages.inbounds.form.mldsa65Verify')}
      >
        <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} />
      </Form.Item>
      <Form.Item label=" ">
        <Space>
          <Button type="primary" loading={saving} onClick={genMldsa65}>
            {t('pages.inbounds.form.getNewSeed')}
          </Button>
          <Button danger onClick={clearMldsa65}>{t('clear')}</Button>
        </Space>
      </Form.Item>
    </>
  );
}
