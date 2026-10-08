import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Select, Space, Switch, Tabs } from 'antd';
import { BellOutlined, SendOutlined, SettingOutlined, WarningOutlined } from '@ant-design/icons';
import { HttpUtil, LanguageManager } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { TelegramNotifications } from '@/components/ui/notifications/TelegramNotifications';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import { NotifyTimeField } from './NotifyTimeField';
import SecretInput from './SecretInput';

interface TelegramTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

export default function TelegramTab({ allSetting, updateSetting }: TelegramTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; msg: string } | null>(null);
  const [inlineWarning, setInlineWarning] = useState<string | null>(null);
  const [privacyWarning, setPrivacyWarning] = useState<string | null>(null);
  const [capsLoading, setCapsLoading] = useState(false);

  // The bot's lists live in inline mode, which only BotFather can enable, so the
  // panel checks the live capability and warns instead of leaving dead buttons.
  async function checkInlineMode() {
    setCapsLoading(true);
    try {
      const res = (await HttpUtil.post('/panel/api/setting/tgBotCapabilities')) as {
        success?: boolean;
        obj?: { inlineEnabled?: boolean; groupPrivacy?: boolean; username?: string };
      };
      if (res.success && res.obj && !res.obj.inlineEnabled) {
        setInlineWarning(res.obj.username || '');
      } else {
        setInlineWarning(null);
      }
      // Privacy mode is the mirror-image trap: with it on, a bare /start typed
      // in a group never reaches the bot, so the command looks dead.
      setPrivacyWarning(res.success && !!res.obj?.groupPrivacy ? res.obj?.username || '' : null);
    } catch {
      // The bot may simply be stopped; the token field still tells that story.
      setInlineWarning(null);
    } finally {
      setCapsLoading(false);
    }
  }

  async function handleTestTgBot() {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = (await HttpUtil.post('/panel/api/setting/testTgBot')) as {
        success?: boolean;
        msg?: string;
      };
      setTestResult({ success: !!res.success, msg: res.msg || '' });
    } catch (e: unknown) {
      setTestResult({
        success: false,
        msg: e instanceof Error ? e.message : t('pages.settings.requestFailed'),
      });
    } finally {
      setTestLoading(false);
    }
  }

  const langOptions = useMemo(
    () =>
      LanguageManager.supportedLanguages.map(
        (l: { value: string; name: string; icon: string }) => ({
          value: l.value,
          label: (
            <>
              <span role="img" aria-label={l.name}>
                {l.icon}
              </span>
              &nbsp;&nbsp;<span>{l.name}</span>
            </>
          ),
        }),
      ),
    [],
  );

  return (
    <Tabs
      defaultActiveKey="1"
      items={[
        {
          key: '1',
          label: catTabLabel(<SettingOutlined />, t('pages.settings.panelSettings'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.telegramBotEnable')}
                description={t('pages.settings.telegramBotEnableDesc')}
              >
                <Switch
                  checked={allSetting.tgBotEnable}
                  onChange={(v) => updateSetting({ tgBotEnable: v })}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.telegramToken')}
                description={
                  allSetting.hasTgBotToken && !allSetting.clearTgBotToken
                    ? t('pages.settings.telegramTokenConfigured')
                    : t('pages.settings.telegramTokenDesc')
                }
              >
                <SecretInput
                  value={allSetting.tgBotToken}
                  configured={allSetting.hasTgBotToken}
                  clearArmed={allSetting.clearTgBotToken}
                  placeholder={t('pages.settings.telegramTokenPlaceholder')}
                  onChange={(v) => updateSetting({ tgBotToken: v })}
                  onClearArmedChange={(armed) => updateSetting({ clearTgBotToken: armed })}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.telegramChatId')}
                description={t('pages.settings.telegramChatIdDesc')}
              >
                <Input
                  value={allSetting.tgBotChatId}
                  onChange={(e) => updateSetting({ tgBotChatId: e.target.value })}
                />
              </SettingListItem>

              <SettingListItem paddings="small" title={t('pages.settings.telegramBotLanguage')}>
                <Select
                  value={allSetting.tgLang}
                  onChange={(v) => updateSetting({ tgLang: v })}
                  style={{ width: '100%' }}
                  options={langOptions}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.telegramAPIServer')}
                description={t('pages.settings.telegramAPIServerDesc')}
              >
                <Input
                  value={allSetting.tgBotAPIServer}
                  placeholder="https://api.example.com"
                  onChange={(e) => updateSetting({ tgBotAPIServer: e.target.value })}
                />
              </SettingListItem>

              {inlineWarning !== null && (
                <Alert
                  type="warning"
                  showIcon
                  icon={<WarningOutlined />}
                  style={{ marginTop: 16 }}
                  title={t('pages.settings.tgInlineMissing')}
                  description={t('pages.settings.tgInlineMissingDesc')}
                  action={
                    <Button size="small" loading={capsLoading} onClick={checkInlineMode}>
                      {t('pages.settings.tgInlineRecheck')}
                    </Button>
                  }
                />
              )}

              {privacyWarning !== null && (
                <Alert
                  type="warning"
                  showIcon
                  icon={<WarningOutlined />}
                  style={{ marginTop: 16 }}
                  title={t('pages.settings.tgPrivacyOn')}
                  description={t('pages.settings.tgPrivacyOnDesc')}
                />
              )}

              <Space orientation="vertical" size={8} style={{ width: '100%', marginTop: 16 }}>
                <Button
                  type="primary"
                  icon={<SendOutlined />}
                  loading={testLoading || capsLoading}
                  onClick={() => {
                    setTestResult(null);
                    void handleTestTgBot();
                    void checkInlineMode();
                  }}
                >
                  {t('pages.settings.testTgBot')}
                </Button>
                {testResult && (
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
          key: '2',
          label: catTabLabel(<BellOutlined />, t('pages.settings.notifications'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.telegramNotifyTime')}
                description={t('pages.settings.telegramNotifyTimeDesc')}
              >
                <NotifyTimeField
                  value={allSetting.tgRunTime}
                  onChange={(v) => updateSetting({ tgRunTime: v })}
                />
              </SettingListItem>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.tgNotifyBackup')}
                description={t('pages.settings.tgNotifyBackupDesc')}
              >
                <Switch
                  checked={allSetting.tgBotBackup}
                  onChange={(v) => updateSetting({ tgBotBackup: v })}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.tgEventBusNotify')}
                description={t('pages.settings.tgEventBusNotifyDesc')}
              >
                <TelegramNotifications allSetting={allSetting} updateSetting={updateSetting} />
              </SettingListItem>
            </>
          ),
        },
      ]}
    />
  );
}
