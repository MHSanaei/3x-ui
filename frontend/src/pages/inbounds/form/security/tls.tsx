import { useId } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Form,
  Input,
  InputNumber,
  Radio,
  Select,
  Space,
  Switch,
  Typography,
} from 'antd';
import {
  CloudDownloadOutlined,
  FileProtectOutlined,
  MinusOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form';

import { CipherSuitesSelect } from '@/components/form';
import DynamicEchHelper from '@/components/form/DynamicEchHelper';
import { FormField } from '@/components/form/rhf';
import {
  ALPN_OPTION,
  DOMAIN_STRATEGY_OPTION,
  TLS_VERSION_OPTION,
  USAGE_OPTION,
  UTLS_FINGERPRINT,
} from '@/schemas/primitives';
import { SockoptStreamSettingsSchema } from '@/schemas/protocols/stream/sockopt';

const { TextArea } = Input;

const CERT_LINES_TRANSFORM = {
  input: (v: unknown) => (Array.isArray(v) ? v.join('\n') : v),
  output: (raw: unknown) => (typeof raw === 'string' ? raw.split('\n') : raw),
};

interface TlsFormProps {
  saving: boolean;
  setCertFromPanel: (certName: number) => void;
  clearCertFiles: (certName: number) => void;
  pinFromCert: () => void;
  pinFromRemote: () => void;
  getNewEchCert: () => void;
  clearEchCert: () => void;
}

interface CertRowProps {
  index: number;
  total: number;
  saving: boolean;
  onRemove: () => void;
  setCertFromPanel: (certName: number) => void;
  clearCertFiles: (certName: number) => void;
}

function CertRow({
  index,
  total,
  saving,
  onRemove,
  setCertFromPanel,
  clearCertFiles,
}: CertRowProps) {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const useFile = useWatch({
    control,
    name: `streamSettings.tlsSettings.certificates.${index}.useFile`,
  });
  const usage = useWatch({
    control,
    name: `streamSettings.tlsSettings.certificates.${index}.usage`,
  });
  return (
    <div>
      <FormField
        name={['streamSettings', 'tlsSettings', 'certificates', index, 'useFile']}
        label={`${t('certificate')} ${index + 1}`}
      >
        <Radio.Group buttonStyle="solid">
          <Radio.Button value={true}>{t('pages.inbounds.certificatePath')}</Radio.Button>
          <Radio.Button value={false}>{t('pages.inbounds.certificateContent')}</Radio.Button>
        </Radio.Group>
      </FormField>
      {total > 1 && (
        <Form.Item label=" ">
          <Button size="small" danger onClick={onRemove}>
            <MinusOutlined aria-hidden /> {t('remove')}
          </Button>
        </Form.Item>
      )}
      {useFile ? (
        <>
          <FormField
            name={['streamSettings', 'tlsSettings', 'certificates', index, 'certificateFile']}
            label={t('pages.inbounds.publicKey')}
          >
            <Input />
          </FormField>
          <FormField
            name={['streamSettings', 'tlsSettings', 'certificates', index, 'keyFile']}
            label={t('pages.inbounds.privatekey')}
          >
            <Input />
          </FormField>
          <Form.Item label=" ">
            <Space>
              <Button type="primary" loading={saving} onClick={() => setCertFromPanel(index)}>
                {t('pages.inbounds.setDefaultCert')}
              </Button>
              <Button danger onClick={() => clearCertFiles(index)}>
                {t('clear')}
              </Button>
            </Space>
          </Form.Item>
        </>
      ) : (
        <>
          <FormField
            name={['streamSettings', 'tlsSettings', 'certificates', index, 'certificate']}
            label={t('pages.inbounds.publicKey')}
            transform={CERT_LINES_TRANSFORM}
          >
            <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
          </FormField>
          <FormField
            name={['streamSettings', 'tlsSettings', 'certificates', index, 'key']}
            label={t('pages.inbounds.privatekey')}
            transform={CERT_LINES_TRANSFORM}
          >
            <TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
          </FormField>
        </>
      )}
      <FormField
        name={['streamSettings', 'tlsSettings', 'certificates', index, 'ocspStapling']}
        label="OCSP Stapling"
      >
        <InputNumber min={0} suffix="s" style={{ width: '50%' }} />
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'certificates', index, 'oneTimeLoading']}
        label={t('pages.inbounds.form.oneTimeLoading')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'certificates', index, 'usage']}
        label={t('pages.inbounds.form.usageOption')}
      >
        <Select
          style={{ width: '50%' }}
          options={Object.values(USAGE_OPTION).map((u) => ({ value: u, label: u }))}
        />
      </FormField>
      {usage === 'issue' && (
        <FormField
          name={['streamSettings', 'tlsSettings', 'certificates', index, 'buildChain']}
          label={t('pages.inbounds.form.buildChain')}
          valueProp="checked"
        >
          <Switch />
        </FormField>
      )}
    </div>
  );
}

function EchSockoptSection() {
  const { t } = useTranslation();
  const uid = useId();
  const { control, setValue } = useFormContext();
  const echSockopt = useWatch({ control, name: 'streamSettings.tlsSettings.echSockopt' });
  const on = !!echSockopt;
  return (
    <>
      <Form.Item
        htmlFor={`${uid}-echSockopt`}
        label={t('pages.inbounds.form.echSockopt')}
        tooltip={t('pages.inbounds.form.echSockoptTip')}
      >
        <Switch
          id={`${uid}-echSockopt`}
          aria-describedby={`${uid}-echSockopt-hint`}
          checked={on}
          onChange={(v) =>
            setValue(
              'streamSettings.tlsSettings.echSockopt',
              v ? SockoptStreamSettingsSchema.parse({}) : undefined,
            )
          }
        />
        <span id={`${uid}-echSockopt-hint`} hidden>
          {t('pages.inbounds.form.echSockoptTip')}
        </span>
      </Form.Item>
      {on && (
        <>
          <FormField
            name={['streamSettings', 'tlsSettings', 'echSockopt', 'dialerProxy']}
            label={t('pages.inbounds.form.dialerProxy')}
          >
            <Input />
          </FormField>
          <FormField
            name={['streamSettings', 'tlsSettings', 'echSockopt', 'domainStrategy']}
            label={t('pages.xray.wireguard.domainStrategy')}
          >
            <Select
              options={Object.values(DOMAIN_STRATEGY_OPTION).map((v) => ({ value: v, label: v }))}
            />
          </FormField>
          <FormField
            name={['streamSettings', 'tlsSettings', 'echSockopt', 'tcpFastOpen']}
            label={t('pages.inbounds.form.tcpFastOpen')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
          <FormField
            name={['streamSettings', 'tlsSettings', 'echSockopt', 'tcpMptcp']}
            label={t('pages.inbounds.form.multipathTcp')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
        </>
      )}
    </>
  );
}

interface EchKeySectionProps {
  saving: boolean;
  getNewEchCert: () => void;
  clearEchCert: () => void;
}

function EchKeySection({ saving, getNewEchCert, clearEchCert }: EchKeySectionProps) {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const localKeys = useWatch({ control, name: 'streamSettings.tlsSettings.echServerKeys' });
  const echConfig = useWatch({
    control,
    name: 'streamSettings.tlsSettings.settings.echConfigList',
  });
  const serverName: string =
    useWatch({ control, name: 'streamSettings.tlsSettings.serverName' }) ?? '';
  const helper = (
    <DynamicEchHelper
      value={echConfig ?? ''}
      sni={serverName}
      hasLocalKeys={!!localKeys}
      onApply={(next) =>
        setValue('streamSettings.tlsSettings.settings.echConfigList', next, {
          shouldDirty: true,
        })
      }
      onClearLocalKeys={() => setValue('streamSettings.tlsSettings.echServerKeys', '')}
    />
  );
  return (
    <>
      <FormField
        name={['streamSettings', 'tlsSettings', 'settings', 'echConfigList']}
        label={t('pages.inbounds.form.echConfigList')}
        extra={
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('pages.inbounds.form.echConfigListHint')}
          </Typography.Text>
        }
      >
        <Input placeholder={t('pages.inbounds.form.echDynamicPlaceholder')} />
      </FormField>
      <Form.Item label=" " style={{ marginBottom: 8 }}>
        {helper}
      </Form.Item>
      <FormField
        name={['streamSettings', 'tlsSettings', 'echServerKeys']}
        label={t('pages.inbounds.form.echKey')}
      >
        <Input placeholder={t('pages.inbounds.form.echKeyHint')} />
      </FormField>
      <Alert
        type="info"
        showIcon
        data-testid="ech-cdn-warning"
        style={{ marginBottom: 12 }}
        title={t('pages.inbounds.form.echCdnWarningTitle')}
        description={t('pages.inbounds.form.echCdnWarningBody')}
      />
      <Form.Item label=" ">
        <Space wrap>
          <Button type="primary" loading={saving} onClick={getNewEchCert}>
            {t('pages.inbounds.form.getNewEchCert')}
          </Button>
          <Button danger onClick={clearEchCert}>
            {t('clear')}
          </Button>
        </Space>
      </Form.Item>
    </>
  );
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
  const uid = useId();
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({
    control,
    name: 'streamSettings.tlsSettings.certificates',
  });
  return (
    <>
      <FormField name={['streamSettings', 'tlsSettings', 'serverName']} label="SNI">
        <Input placeholder={t('pages.inbounds.form.serverNameIndication')} />
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'cipherSuites']}
        label={t('pages.inbounds.form.cipherSuites')}
      >
        <CipherSuitesSelect placeholder={t('pages.inbounds.form.autoOption')} />
      </FormField>
      <Form.Item htmlFor={`${uid}-minVersion`} label={t('pages.inbounds.form.minMaxVersion')}>
        <Space.Compact block>
          <FormField name={['streamSettings', 'tlsSettings', 'minVersion']} noStyle>
            <Select
              id={`${uid}-minVersion`}
              style={{ width: '50%' }}
              options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
            />
          </FormField>
          <FormField name={['streamSettings', 'tlsSettings', 'maxVersion']} noStyle>
            <Select
              aria-label={t('pages.inbounds.form.maxVersion')}
              style={{ width: '50%' }}
              options={Object.values(TLS_VERSION_OPTION).map((v) => ({ value: v, label: v }))}
            />
          </FormField>
        </Space.Compact>
      </Form.Item>
      <FormField name={['streamSettings', 'tlsSettings', 'settings', 'fingerprint']} label="uTLS">
        <Select
          options={[
            { value: '', label: 'None' },
            ...Object.values(UTLS_FINGERPRINT).map((fp) => ({ value: fp, label: fp })),
          ]}
        />
      </FormField>
      <FormField name={['streamSettings', 'tlsSettings', 'alpn']} label="ALPN">
        <Select
          mode="multiple"
          tokenSeparators={[',']}
          style={{ width: '100%' }}
          options={Object.values(ALPN_OPTION).map((a) => ({ value: a, label: a }))}
        />
      </FormField>
      <FormField
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
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'rejectUnknownSni']}
        label={t('pages.inbounds.form.rejectUnknownSni')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'disableSystemRoot']}
        label={t('pages.inbounds.form.disableSystemRoot')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['streamSettings', 'tlsSettings', 'enableSessionResumption']}
        label={t('pages.inbounds.form.sessionResumption')}
        valueProp="checked"
      >
        <Switch />
      </FormField>

      <Form.Item label={t('certificate')}>
        <Button
          aria-label={t('add')}
          type="primary"
          size="small"
          onClick={() =>
            append({
              useFile: true,
              certificateFile: '',
              keyFile: '',
              certificate: [],
              key: [],
              ocspStapling: 0,
              oneTimeLoading: false,
              usage: 'encipherment',
              buildChain: false,
            })
          }
        >
          <PlusOutlined aria-hidden />
        </Button>
      </Form.Item>
      {fields.map((field, idx) => (
        <CertRow
          key={field.id}
          index={idx}
          total={fields.length}
          saving={saving}
          onRemove={() => remove(idx)}
          setCertFromPanel={setCertFromPanel}
          clearCertFiles={clearCertFiles}
        />
      ))}
      <FormField
        name={['streamSettings', 'tlsSettings', 'masterKeyLog']}
        label={t('pages.inbounds.form.masterKeyLog')}
        tooltip={t('pages.inbounds.form.masterKeyLogTip')}
      >
        <Input placeholder="/path/to/sslkeylog.txt" />
      </FormField>
      <EchSockoptSection />
      <EchKeySection saving={saving} getNewEchCert={getNewEchCert} clearEchCert={clearEchCert} />
      <Form.Item
        htmlFor={`${uid}-pinnedPeerCertSha256`}
        label={t('pages.inbounds.form.pinnedPeerCertSha256')}
        tooltip={t('pages.inbounds.form.pinnedPeerCertSha256Tip')}
      >
        <Space.Compact block>
          <FormField
            name={['streamSettings', 'tlsSettings', 'settings', 'pinnedPeerCertSha256']}
            noStyle
          >
            <Select
              id={`${uid}-pinnedPeerCertSha256`}
              aria-describedby={`${uid}-pinnedPeerCertSha256-hint`}
              mode="tags"
              tokenSeparators={[',', ' ']}
              placeholder={t('pages.inbounds.form.pinnedPeerCertSha256Placeholder')}
              style={{ width: 'calc(100% - 64px)' }}
            />
          </FormField>
          <Button
            icon={<FileProtectOutlined aria-hidden />}
            onClick={pinFromCert}
            loading={saving}
            aria-label={t('pages.inbounds.form.pinFromCert')}
            title={t('pages.inbounds.form.pinFromCert')}
          />
          <Button
            icon={<CloudDownloadOutlined aria-hidden />}
            onClick={() => pinFromRemote()}
            loading={saving}
            aria-label={t('pages.inbounds.form.pinFromRemote')}
            title={t('pages.inbounds.form.pinFromRemote')}
          />
        </Space.Compact>
        <span id={`${uid}-pinnedPeerCertSha256-hint`} hidden>
          {t('pages.inbounds.form.pinnedPeerCertSha256Tip')}
        </span>
      </Form.Item>
      <FormField
        name={['streamSettings', 'tlsSettings', 'settings', 'verifyPeerCertByName']}
        label={t('pages.inbounds.form.verifyPeerCertByName')}
        tooltip={t('pages.inbounds.form.verifyPeerCertByNameTip')}
      >
        <Input placeholder="example.com" />
      </FormField>
    </>
  );
}
