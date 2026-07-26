import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Space, Switch, Tabs } from 'antd';
import { BellOutlined, SendOutlined, SettingOutlined } from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { WebhookNotifications } from '@/components/ui/notifications/WebhookNotifications';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import SecretInput from './SecretInput';

interface WebhookTabProps {
    allSetting: AllSetting;
    updateSetting: (patch: Partial<AllSetting>) => void;
}

interface WebhookTestResult {
    success: boolean;
    stage?: string;
    msg: string;
}

// crypto.getRandomValues-backed hex secret, matching the byte strength of a
// typical webhook signing key (32 bytes -> 64 hex chars).
function generateSecret(): string {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export default function WebhookTab({ allSetting, updateSetting }: WebhookTabProps) {
    const { t } = useTranslation();
    const { isMobile } = useMediaQuery();
    const [testLoading, setTestLoading] = useState(false);
    const [testResult, setTestResult] = useState<WebhookTestResult | null>(null);

    const stageLabel: Record<string, string> = {
        config: t('pages.settings.webhookStageConfig'),
        send: t('pages.settings.webhookStageSend'),
    };

    async function handleTestWebhook() {
        setTestLoading(true);
        setTestResult(null);
        try {
            const res = await HttpUtil.post('/panel/api/setting/testWebhook') as WebhookTestResult;
            setTestResult(res);
        } catch (e: unknown) {
            setTestResult({ success: false, msg: e instanceof Error ? e.message : t('pages.settings.requestFailed') });
        } finally {
            setTestLoading(false);
        }
    }

    function handleGenerateSecret() {
        updateSetting({ webhookSecret: generateSecret(), clearWebhookSecret: false });
    }

    return (
        <Tabs defaultActiveKey="1" items={[
            {
                key: '1',
                label: catTabLabel(<SettingOutlined />, t('pages.settings.webhookSettings'), isMobile),
                children: (
                    <>
                        <SettingListItem paddings="small" title={t('pages.settings.webhookEnable')} description={t('pages.settings.webhookEnableDesc')}>
                            <Switch checked={allSetting.webhookEnable} onChange={(v) => updateSetting({ webhookEnable: v })} />
                        </SettingListItem>

                        <SettingListItem paddings="small" title={t('pages.settings.webhookURL')} description={t('pages.settings.webhookURLDesc')}>
                            <Input value={allSetting.webhookURL} placeholder="https://example.com/hooks/3x-ui"
                                   onChange={(e) => updateSetting({ webhookURL: e.target.value })} />
                        </SettingListItem>

                        <SettingListItem paddings="small" title={t('pages.settings.webhookSecret')}
                                         description={allSetting.hasWebhookSecret && !allSetting.clearWebhookSecret ? t('pages.settings.webhookSecretConfigured') : t('pages.settings.webhookSecretDesc')}>
                            <Space.Compact style={{ width: '100%' }}>
                                <SecretInput value={allSetting.webhookSecret}
                                             configured={allSetting.hasWebhookSecret}
                                             clearArmed={allSetting.clearWebhookSecret}
                                             placeholder={t('pages.settings.webhookSecretPlaceholder')}
                                             onChange={(v) => updateSetting({ webhookSecret: v })}
                                             onClearArmedChange={(armed) => updateSetting({ clearWebhookSecret: armed })} />
                                <Button onClick={handleGenerateSecret}>
                                    {t('pages.settings.webhookSecretGenerate')}
                                </Button>
                            </Space.Compact>
                        </SettingListItem>

                        <Space orientation="vertical" size={8} style={{ width: '100%', marginTop: 16 }}>
                            <Button type="primary" icon={<SendOutlined />} loading={testLoading} onClick={handleTestWebhook}>
                                {t('pages.settings.testWebhook')}
                            </Button>
                            {testResult && (
                                <Alert
                                    type={testResult.success ? 'success' : 'error'}
                                    title={
                                        testResult.success
                                            ? t('pages.settings.' + testResult.msg)
                                            : <span><b>{stageLabel[testResult.stage || ''] || testResult.stage}:</b> {t('pages.settings.' + testResult.msg)}</span>
                                    }
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
                label: catTabLabel(<BellOutlined />, t('pages.settings.webhookNotifications'), isMobile),
                children: (
                    <SettingListItem paddings="small" title={t('pages.settings.webhookEventBusNotify')} description={t('pages.settings.webhookEventBusNotifyDesc')}>
                        <WebhookNotifications allSetting={allSetting} updateSetting={updateSetting} />
                    </SettingListItem>
                ),
            },
        ]} />
    );
}