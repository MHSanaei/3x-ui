import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Space, Switch, Tabs } from 'antd';
import { BellOutlined, SendOutlined, SettingOutlined } from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { DiscordNotifications } from '@/components/ui/notifications/DiscordNotifications';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import SecretInput from './SecretInput';

interface DiscordTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

interface DiscordTestResult {
  success: boolean;
  msg: string;
}

export default function DiscordTab({ allSetting, updateSetting }: DiscordTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<DiscordTestResult | null>(null);

  async function handleTestDiscord() {
    setTestLoading(true);
    setTestResult(null);
    try {
      const res = (await HttpUtil.post('/panel/api/setting/testDiscord')) as DiscordTestResult;
      setTestResult(res);
    } catch (e: unknown) {
      setTestResult({
        success: false,
        msg: e instanceof Error ? e.message : t('pages.settings.requestFailed'),
      });
    } finally {
      setTestLoading(false);
    }
  }

  return (
    <Tabs
      defaultActiveKey="1"
      items={[
        {
          key: '1',
          label: catTabLabel(<SettingOutlined />, t('pages.settings.discordSettings'), isMobile),
          children: (
            <>
              <SettingListItem
                paddings="small"
                title={t('pages.settings.discordBotEnable')}
                description={t('pages.settings.discordBotEnableDesc')}
              >
                <Switch
                  checked={allSetting.discordBotEnable}
                  onChange={(v) => updateSetting({ discordBotEnable: v })}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.discordBotToken')}
                description={
                  allSetting.hasDiscordBotToken && !allSetting.clearDiscordBotToken
                    ? t('pages.settings.discordTokenConfigured')
                    : t('pages.settings.discordBotTokenDesc')
                }
              >
                <SecretInput
                  value={allSetting.discordBotToken}
                  configured={allSetting.hasDiscordBotToken}
                  clearArmed={allSetting.clearDiscordBotToken}
                  placeholder={t('pages.settings.discordTokenPlaceholder')}
                  onChange={(v) => updateSetting({ discordBotToken: v })}
                  onClearArmedChange={(armed) => updateSetting({ clearDiscordBotToken: armed })}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.discordChannelId')}
                description={t('pages.settings.discordChannelIdDesc')}
              >
                <Input
                  value={allSetting.discordChannelId}
                  placeholder="e.g. 123456789012345678"
                  onChange={(e) => updateSetting({ discordChannelId: e.target.value })}
                  style={{ width: '100%' }}
                />
              </SettingListItem>

              <SettingListItem
                paddings="small"
                title={t('pages.settings.testDiscord')}
                description={t('pages.settings.testDiscordDesc')}
              >
                <Space direction="vertical" align="end" style={{ width: '100%' }}>
                  <Button
                    type="primary"
                    icon={<SendOutlined />}
                    loading={testLoading}
                    disabled={!allSetting.discordBotEnable}
                    onClick={handleTestDiscord}
                  >
                    {t('pages.settings.testDiscord')}
                  </Button>
                </Space>
              </SettingListItem>

              {testResult && (
                <Alert
                  style={{ marginTop: 16 }}
                  type={testResult.success ? 'success' : 'error'}
                  message={testResult.msg}
                  showIcon
                  closable
                  onClose={() => setTestResult(null)}
                />
              )}
            </>
          ),
        },
        {
          key: '2',
          label: catTabLabel(<BellOutlined />, t('pages.settings.discordEventBusNotify'), isMobile),
          children: <DiscordNotifications allSetting={allSetting} updateSetting={updateSetting} />,
        },
      ]}
    />
  );
}
