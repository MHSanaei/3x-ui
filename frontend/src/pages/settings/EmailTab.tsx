import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Select, Space, Switch, Tabs } from 'antd';
import { MailOutlined, SendOutlined, SettingOutlined } from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem, EventBusCheckboxes } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';

interface EmailTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

interface SmtpTestResult {
  success: boolean;
  stage?: string;
  msg: string;
}

export default function EmailTab({ allSetting, updateSetting }: EmailTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<SmtpTestResult | null>(null);

  const stageLabel: Record<string, string> = {
    connect: t('pages.settings.smtpStageConnect'),
    auth: t('pages.settings.smtpStageAuth'),
    send: t('pages.settings.smtpStageSend'),
  };

  async function handleTestSmtp() {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = await HttpUtil.post('/panel/api/setting/testSmtp') as SmtpTestResult;
      setTestResult(res);
    } catch (e: unknown) {
      setTestResult({ success: false, msg: e instanceof Error ? e.message : t('pages.settings.requestFailed') });
    } finally {
      setTestLoading(false);
    }
  }

  return (
    <Tabs defaultActiveKey="1" items={[
      {
        key: '1',
        label: catTabLabel(<SettingOutlined />, t('pages.settings.smtpSettings'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.smtpEnable')} description={t('pages.settings.smtpEnableDesc')}>
              <Switch checked={allSetting.smtpEnable} onChange={(v) => updateSetting({ smtpEnable: v })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpHost')} description={t('pages.settings.smtpHostDesc')}>
              <Input value={allSetting.smtpHost} placeholder="smtp.gmail.com"
                onChange={(e) => updateSetting({ smtpHost: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpPort')} description={t('pages.settings.smtpPortDesc')}>
              <InputNumber value={allSetting.smtpPort} min={1} max={65535} style={{ width: '100%' }}
                onChange={(v) => updateSetting({ smtpPort: Number(v) || 587 })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpUsername')} description={t('pages.settings.smtpUsernameDesc')}>
              <Input value={allSetting.smtpUsername} placeholder="user@gmail.com"
                onChange={(e) => updateSetting({ smtpUsername: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpPassword')}
              description={allSetting.hasSmtpPassword ? t('pages.settings.smtpPasswordConfigured') : t('pages.settings.smtpPasswordDesc')}>
              <Input.Password value={allSetting.smtpPassword}
                placeholder={allSetting.hasSmtpPassword ? t('pages.settings.smtpPasswordPlaceholder') : ''}
                onChange={(e) => updateSetting({ smtpPassword: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpTo')} description={t('pages.settings.smtpToDesc')}>
              <Input value={allSetting.smtpTo} placeholder="admin@example.com, ops@example.com"
                onChange={(e) => updateSetting({ smtpTo: e.target.value })} />
            </SettingListItem>

            <SettingListItem paddings="small" title={t('pages.settings.smtpEncryption')} description={t('pages.settings.smtpEncryptionDesc')}>
              <Select
                value={allSetting.smtpEncryptionType}
                onChange={(v) => updateSetting({ smtpEncryptionType: v })}
                options={[
                  { value: 'none', label: t('pages.settings.smtpEncryptionNone') },
                  { value: 'starttls', label: t('pages.settings.smtpEncryptionStartTLS') },
                  { value: 'tls', label: t('pages.settings.smtpEncryptionTLS') },
                ]}
                style={{ width: '100%' }}
              />
            </SettingListItem>

            <Space orientation="vertical" size={8} style={{ width: '100%', marginTop: 16 }}>
              <Button type="primary" icon={<SendOutlined />} loading={testLoading} onClick={handleTestSmtp}>
                {t('pages.settings.testSmtp')}
              </Button>
              {testResult && (
                <Alert
                  type={testResult.success ? 'success' : 'error'}
                  message={
                    testResult.success
                      ? t('pages.settings.' + testResult.msg)
                      : <span><b>{stageLabel[testResult.stage || ''] || testResult.stage}:</b> {t('pages.settings.' + testResult.msg)}</span>
                  }
                  showIcon
                  closable
                  onClose={() => setTestResult(null)}
                />
              )}
            </Space>
          </>
        ),
      },
      {
        key: '2',
        label: catTabLabel(<MailOutlined />, t('pages.settings.emailNotifications'), isMobile),
        children: (
          <>
            <SettingListItem paddings="small" title={t('pages.settings.smtpEventBusNotify')} description={t('pages.settings.smtpEventBusNotifyDesc')}>
              <EventBusCheckboxes
                value={allSetting.smtpEnabledEvents}
                onChange={(v) => updateSetting({ smtpEnabledEvents: v })}
                extra={{ 'cpu.high': { key: 'smtpCpu', value: allSetting.smtpCpu } }}
                onExtraChange={(key, v) => updateSetting({ [key]: Number(v) || 0 })}
              />
            </SettingListItem>
          </>
        ),
      },
    ]} />
  );
}
