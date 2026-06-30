import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Radio, Select, Space, Switch } from 'antd';
import { CloudDownloadOutlined, FileProtectOutlined, MinusOutlined, PlusOutlined } from '@ant-design/icons';

import {
  ALPN_OPTION,
  DOMAIN_STRATEGY_OPTION,
  TLS_CIPHER_OPTION,
  TLS_VERSION_OPTION,
  USAGE_OPTION,
  UTLS_FINGERPRINT,
} from '@/schemas/primitives';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';

const { TextArea } = Input;

interface TlsFormProps {
  saving: boolean;
  setCertFromPanel: (certName: number) => void;
  clearCertFiles: (certName: number) => void;
  pinFromCert: () => void;
  pinFromRemote: () => void;
  getNewEchCert: () => void;
  clearEchCert: () => void;
}

export default function TlsForm({
  saving,
  setCertFromPanel,
  clearCertFiles,
  pinFromCert,
  pinFromRemote,
  getNewEchCert,
  clearEchCert,
}: TlsFormProps) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item name={['streamSettings', 'tlsSettings', 'serverName']} label="SNI">
        <Input placeholder={t('pages.inbounds.form.serverNameIndication')} />
      </Form.Item>
      <Form.Item name={['streamSettings', 'tlsSettings', 'cipherSuites']} label={t('pages.inbounds.form.cipherSuites')}>
        <Select
          options={[
            { value: '', label: t('pages.inbounds.form.autoOption') },
            ...Object.entries(TLS_CIPHER_OPTION).map(([k, v]) => ({ value: v, label: k })),
          ]}
        />
      </Form.Item>
      <Form.Item label={t('pages.inbounds.form.minMaxVersion')}>
        <Space.Compact block>
          <Form.Item name={['streamSettings', 'tlsSettings', 'minVersion']} noStyle>
            <Select
              style={{ width: '50%' }}
              options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
          <Form.Item name={['streamSettings', 'tlsSettings', 'maxVersion']} noStyle>
            <Select
              style={{ width: '50%' }}
              options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
        </Space.Compact>
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'settings', 'fingerprint']}
        label="uTLS"
      >
        <Select
          options={[
            { value: '', label: 'None' },
            ...Object.values(UTLS_FINGERPRINT).map((fp) => ({ value: fp, label: fp })),
          ]}
        />
      </Form.Item>
      <Form.Item name={['streamSettings', 'tlsSettings', 'alpn']} label="ALPN">
        <Select
          mode="multiple"
          tokenSeparators={[',']}
          style={{ width: '100%' }}
          options={Object.values(ALPN_OPTION).map((a) => ({ value: a, label: a }))}
        />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'curvePreferences']}
        label={t('pages.inbounds.form.curvePreferences')}
        tooltip={t('pages.inbounds.form.curvePreferencesTip')}
      >
        <Select
          mode="tags"
          tokenSeparators={[',', ' ']}
          style={{ width: '100%' }}
          options={['X25519MLKEM768', 'X25519', 'P-256', 'P-384', 'P-521'].map((c) => ({
            value: c,
            label: c,
          }))}
        />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'rejectUnknownSni']}
        label={t('pages.inbounds.form.rejectUnknownSni')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'disableSystemRoot']}
        label={t('pages.inbounds.form.disableSystemRoot')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'enableSessionResumption']}
        label={t('pages.inbounds.form.sessionResumption')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>

      <Form.List name={['streamSettings', 'tlsSettings', 'certificates']}>
        {(certFields, { add, remove }) => (
          <>
            <Form.Item label={t('certificate')}>
              <Button
                aria-label={t('add')}
                type="primary"
                size="small"
                onClick={() => add({
                  useFile: true,
                  certificateFile: '',
                  keyFile: '',
                  certificate: [],
                  key: [],
                  ocspStapling: 0,
                  oneTimeLoading: false,
                  usage: 'encipherment',
                  buildChain: false,
                })}
              >
                <PlusOutlined />
              </Button>
            </Form.Item>
            {certFields.map((certField, idx) => (
              <div key={certField.key}>
                <Form.Item
                  name={[certField.name, 'useFile']}
                  label={`${t('certificate')} ${idx + 1}`}
                >
                  <Radio.Group buttonStyle="solid">
                    <Radio.Button value={true}>
                      {t('pages.inbounds.certificatePath')}
                    </Radio.Button>
                    <Radio.Button value={false}>
                      {t('pages.inbounds.certificateContent')}
                    </Radio.Button>
                  </Radio.Group>
                </Form.Item>
                {certFields.length > 1 && (
                  <Form.Item label=" ">
                    <Button
                      size="small"
                      danger
                      onClick={() => remove(certField.name)}
                    >
                      <MinusOutlined /> {t('remove')}
                    </Button>
                  </Form.Item>
                )}
                <Form.Item
                  noStyle
                  shouldUpdate={(prev, curr) =>
                    prev.streamSettings?.tlsSettings?.certificates?.[certField.name]?.useFile
                    !== curr.streamSettings?.tlsSettings?.certificates?.[certField.name]?.useFile
                  }
                >
                  {({ getFieldValue }) => {
                    const useFile = getFieldValue([
                      'streamSettings', 'tlsSettings', 'certificates',
                      certField.name, 'useFile',
                    ]);
                    return useFile ? (
                      <>
                        <Form.Item
                          name={[certField.name, 'certificateFile']}
                          label={t('pages.inbounds.publicKey')}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item
                          name={[certField.name, 'keyFile']}
                          label={t('pages.inbounds.privatekey')}
                        >
                          <Input />
                        </Form.Item>
                        <Form.Item label=" ">
                          <Space>
                            <Button
                              type="primary"
                              loading={saving}
                              onClick={() => setCertFromPanel(certField.name)}
                            >
                              {t('pages.inbounds.setDefaultCert')}
                            </Button>
                            <Button danger onClick={() => clearCertFiles(certField.name)}>
                              {t('clear')}
                            </Button>
                          </Space>
                        </Form.Item>
                      </>
                    ) : (
                      <>
                        <Form.Item
                          name={[certField.name, 'certificate']}
                          label={t('pages.inbounds.publicKey')}
                          normalize={(v) => typeof v === 'string'
                            ? v.split('\n')
                            : v}
                          getValueProps={(v) => ({
                            value: Array.isArray(v) ? v.join('\n') : v,
                          })}
                        >
                          <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
                        </Form.Item>
                        <Form.Item
                          name={[certField.name, 'key']}
                          label={t('pages.inbounds.privatekey')}
                          normalize={(v) => typeof v === 'string'
                            ? v.split('\n')
                            : v}
                          getValueProps={(v) => ({
                            value: Array.isArray(v) ? v.join('\n') : v,
                          })}
                        >
                          <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
                        </Form.Item>
                      </>
                    );
                  }}
                </Form.Item>
                <Form.Item
                  name={[certField.name, 'ocspStapling']}
                  label="OCSP Stapling"
                >
                  <InputNumber min={0} suffix="s" style={{ width: '50%' }} />
                </Form.Item>
                <Form.Item
                  name={[certField.name, 'oneTimeLoading']}
                  label={t('pages.inbounds.form.oneTimeLoading')}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Form.Item
                  name={[certField.name, 'usage']}
                  label={t('pages.inbounds.form.usageOption')}
                >
                  <Select
                    style={{ width: '50%' }}
                    options={Object.values(USAGE_OPTION).map((u) => ({ value: u, label: u }))}
                  />
                </Form.Item>
                <Form.Item
                  noStyle
                  shouldUpdate={(prev, curr) =>
                    prev.streamSettings?.tlsSettings?.certificates?.[certField.name]?.usage
                    !== curr.streamSettings?.tlsSettings?.certificates?.[certField.name]?.usage
                  }
                >
                  {({ getFieldValue }) => {
                    const usage = getFieldValue([
                      'streamSettings', 'tlsSettings', 'certificates',
                      certField.name, 'usage',
                    ]);
                    if (usage !== 'issue') return null;
                    return (
                      <Form.Item
                        name={[certField.name, 'buildChain']}
                        label={t('pages.inbounds.form.buildChain')}
                        valuePropName="checked"
                      >
                        <Switch />
                      </Form.Item>
                    );
                  }}
                </Form.Item>
              </div>
            ))}
          </>
        )}
      </Form.List>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'masterKeyLog']}
        label={t('pages.inbounds.form.masterKeyLog')}
        tooltip={t('pages.inbounds.form.masterKeyLogTip')}
      >
        <Input placeholder="/path/to/sslkeylog.txt" />
      </Form.Item>
      <Form.Item
        noStyle
        shouldUpdate={(prev, curr) =>
          !!(prev.streamSettings as { tlsSettings?: { echSockopt?: unknown } } | undefined)?.tlsSettings?.echSockopt
          !== !!(curr.streamSettings as { tlsSettings?: { echSockopt?: unknown } } | undefined)?.tlsSettings?.echSockopt
        }
      >
        {({ getFieldValue, setFieldValue }) => {
          const on = !!getFieldValue(['streamSettings', 'tlsSettings', 'echSockopt']);
          return (
            <>
              <Form.Item label={t('pages.inbounds.form.echSockopt')} tooltip={t('pages.inbounds.form.echSockoptTip')}>
                <Switch
                  checked={on}
                  onChange={(v) =>
                    setFieldValue(
                      ['streamSettings', 'tlsSettings', 'echSockopt'],
                      v ? SockoptStreamSettingsSchema.parse({}) : undefined,
                    )
                  }
                />
              </Form.Item>
              {on && (
                <>
                  <Form.Item
                    name={['streamSettings', 'tlsSettings', 'echSockopt', 'dialerProxy']}
                    label={t('pages.inbounds.form.dialerProxy')}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item
                    name={['streamSettings', 'tlsSettings', 'echSockopt', 'domainStrategy']}
                    label={t('pages.xray.wireguard.domainStrategy')}
                  >
                    <Select
                      options={Object.values(DOMAIN_STRATEGY_OPTION).map((v) => ({ value: v, label: v }))}
                    />
                  </Form.Item>
                  <Form.Item
                    name={['streamSettings', 'tlsSettings', 'echSockopt', 'tcpFastOpen']}
                    label={t('pages.inbounds.form.tcpFastOpen')}
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                  <Form.Item
                    name={['streamSettings', 'tlsSettings', 'echSockopt', 'tcpMptcp']}
                    label={t('pages.inbounds.form.multipathTcp')}
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                </>
              )}
            </>
          );
        }}
      </Form.Item>
      <Form.Item name={['streamSettings', 'tlsSettings', 'echServerKeys']} label={t('pages.inbounds.form.echKey')}>
        <Input />
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'settings', 'echConfigList']}
        label={t('pages.inbounds.form.echConfig')}
      >
        <Input />
      </Form.Item>
      <Form.Item label=" ">
        <Space>
          <Button type="primary" loading={saving} onClick={getNewEchCert}>
            {t('pages.inbounds.form.getNewEchCert')}
          </Button>
          <Button danger onClick={clearEchCert}>{t('clear')}</Button>
        </Space>
      </Form.Item>
      <Form.Item
        label={t('pages.inbounds.form.pinnedPeerCertSha256')}
        tooltip={t('pages.inbounds.form.pinnedPeerCertSha256Tip')}
      >
        <Space.Compact block>
          <Form.Item
            name={['streamSettings', 'tlsSettings', 'settings', 'pinnedPeerCertSha256']}
            noStyle
          >
            <Select
              mode="tags"
              tokenSeparators={[',', ' ']}
              placeholder={t('pages.inbounds.form.pinnedPeerCertSha256Placeholder')}
              style={{ width: 'calc(100% - 64px)' }}
            />
          </Form.Item>
          <Button
            icon={<FileProtectOutlined />}
            onClick={pinFromCert}
            loading={saving}
            title={t('pages.inbounds.form.pinFromCert')}
          />
          <Button
            icon={<CloudDownloadOutlined />}
            onClick={pinFromRemote}
            loading={saving}
            title={t('pages.inbounds.form.pinFromRemote')}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item
        name={['streamSettings', 'tlsSettings', 'settings', 'verifyPeerCertByName']}
        label={t('pages.inbounds.form.verifyPeerCertByName')}
        tooltip={t('pages.inbounds.form.verifyPeerCertByNameTip')}
      >
        <Input placeholder="example.com" />
      </Form.Item>
    </>
  );
}
