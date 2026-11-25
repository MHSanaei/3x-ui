import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  ConfigProvider,
  Select,
  Space,
  Switch,
  Tabs,
  Typography,
  theme,
} from 'antd';
import { BellOutlined, SendOutlined, SettingOutlined } from '@ant-design/icons';
import { HttpUtil, LanguageManager } from '@/utils';
import type { AllSetting } from '@/models/setting';
import type { WechatMode } from '@/schemas/setting';
import { SettingListItem } from '@/components/ui';
import { WechatNotifications } from '@/components/ui/notifications/WechatNotifications';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { NotifyTimeField } from './NotifyTimeField';
import SecretInput from './SecretInput';

interface WechatTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
  hasPendingChanges?: boolean;
}

export default function WechatTab({
  allSetting,
  updateSetting,
  hasPendingChanges = false,
}: WechatTabProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { isMobile } = useMediaQuery();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; msg: string } | null>(null);
  const languageOptions = useMemo(
    () =>
      LanguageManager.supportedLanguages.map((lang) => ({
        value: lang.value,
        label: `${lang.icon} ${lang.name}`,
      })),
    [],
  );

  async function testWechat() {
    setTesting(true);
    setTestResult(null);
    try {
      const result = await HttpUtil.post('/panel/api/setting/testWechat', undefined, {
        silent: true,
      });
      setTestResult({
        success: !!result.success,
        msg:
          result.msg ||
          t(result.success ? 'pages.settings.wechatTestSuccess' : 'pages.settings.requestFailed'),
      });
    } catch (error: unknown) {
      setTestResult({
        success: false,
        msg: error instanceof Error ? error.message : t('pages.settings.requestFailed'),
      });
    } finally {
      setTesting(false);
    }
  }

  return (
    <ConfigProvider theme={{ components: { Tabs: { itemSelectedColor: token.colorText } } }}>
      <Tabs
        defaultActiveKey="settings"
        items={[
          {
            key: 'settings',
            label: catTabLabel(
              <SettingOutlined aria-hidden />,
              t('pages.settings.panelSettings'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.wechatEnable')}
                  description={t('pages.settings.wechatEnableDesc')}
                >
                  <Switch
                    checked={allSetting.wechatEnable}
                    onChange={(value) => updateSetting({ wechatEnable: value })}
                  />
                </SettingListItem>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.wechatMode')}
                  description={t('pages.settings.wechatModeDesc')}
                >
                  <Select<WechatMode>
                    value={allSetting.wechatMode}
                    onChange={(value) => updateSetting({ wechatMode: value })}
                    style={{ width: '100%' }}
                    options={[
                      { value: 'wecom', label: t('pages.settings.wechatModeWecom') },
                      { value: 'webhook', label: t('pages.settings.wechatModeWebhook') },
                    ]}
                  />
                </SettingListItem>
                <Alert
                  type="info"
                  showIcon
                  title={t('pages.settings.wechatPersonalGateway')}
                  style={{ marginBottom: 12 }}
                />
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.wechatWebhookURL')}
                  description={
                    allSetting.hasWechatWebhookURL && !allSetting.clearWechatWebhookURL
                      ? t('pages.settings.wechatWebhookURLConfigured')
                      : t('pages.settings.wechatWebhookURLDesc')
                  }
                >
                  <SecretInput
                    value={allSetting.wechatWebhookURL}
                    configured={allSetting.hasWechatWebhookURL}
                    clearArmed={allSetting.clearWechatWebhookURL}
                    placeholder={t('pages.settings.wechatSecretPlaceholder')}
                    onChange={(value) => updateSetting({ wechatWebhookURL: value })}
                    onClearArmedChange={(value) => updateSetting({ clearWechatWebhookURL: value })}
                  />
                </SettingListItem>
                {allSetting.wechatMode === 'webhook' && (
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.wechatToken')}
                    description={
                      allSetting.hasWechatToken && !allSetting.clearWechatToken
                        ? t('pages.settings.wechatTokenConfigured')
                        : t('pages.settings.wechatTokenDesc')
                    }
                  >
                    <SecretInput
                      value={allSetting.wechatToken}
                      configured={allSetting.hasWechatToken}
                      clearArmed={allSetting.clearWechatToken}
                      placeholder={t('pages.settings.wechatSecretPlaceholder')}
                      onChange={(value) => updateSetting({ wechatToken: value })}
                      onClearArmedChange={(value) => updateSetting({ clearWechatToken: value })}
                    />
                  </SettingListItem>
                )}
                <SettingListItem paddings="small" title={t('pages.settings.wechatLanguage')}>
                  <Select
                    value={allSetting.wechatLang}
                    onChange={(value) => updateSetting({ wechatLang: value })}
                    options={languageOptions}
                    style={{ width: '100%' }}
                  />
                </SettingListItem>
                <Space orientation="vertical" size={8} style={{ width: '100%', marginTop: 16 }}>
                  <Typography.Text type="secondary">
                    {t('pages.settings.wechatSaveBeforeTest')}
                  </Typography.Text>
                  <Button
                    type="primary"
                    icon={<SendOutlined aria-hidden />}
                    loading={testing}
                    disabled={!allSetting.wechatEnable || hasPendingChanges}
                    onClick={testWechat}
                  >
                    {t('pages.settings.wechatTest')}
                  </Button>
                  {testResult && !hasPendingChanges && (
                    <Alert
                      type={testResult.success ? 'success' : 'error'}
                      title={testResult.msg}
                      showIcon
                      closable={{ onClose: () => setTestResult(null) }}
                    />
                  )}
                </Space>
              </>
            ),
          },
          {
            key: 'notifications',
            label: catTabLabel(
              <BellOutlined aria-hidden />,
              t('pages.settings.notifications'),
              isMobile,
            ),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.wechatReportEnable')}
                  description={t('pages.settings.wechatReportEnableDesc')}
                >
                  <Switch
                    checked={allSetting.wechatReportEnable}
                    onChange={(value) => updateSetting({ wechatReportEnable: value })}
                  />
                </SettingListItem>
                {allSetting.wechatReportEnable && (
                  <SettingListItem
                    paddings="small"
                    title={t('pages.settings.wechatReportTime')}
                    description={t('pages.settings.wechatReportTimeDesc')}
                  >
                    <NotifyTimeField
                      value={allSetting.wechatRunTime}
                      onChange={(value) => updateSetting({ wechatRunTime: value })}
                      ariaLabel={t('pages.settings.wechatReportTime')}
                    />
                  </SettingListItem>
                )}
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.wechatEvents')}
                  description={t('pages.settings.wechatEventsDesc')}
                >
                  <WechatNotifications allSetting={allSetting} updateSetting={updateSetting} />
                </SettingListItem>
              </>
            ),
          },
        ]}
      />
    </ConfigProvider>
  );
}
