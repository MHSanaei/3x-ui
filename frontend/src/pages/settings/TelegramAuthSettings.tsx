import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, Modal, Space, Spin, message } from 'antd';
import { ClipboardManager, HttpUtil } from '@/utils';
import { SettingListItem } from '@/components/ui';

interface TelegramAuthStatus {
  available: boolean;
  linked: boolean;
  telegramUserId: number;
  botUsername?: string;
}

interface TelegramChallenge {
  code: string;
  expiresAt: number;
}

interface Props {
  twoFactorEnabled: boolean;
}

const POLL_INTERVAL_MS = 2000;

export default function TelegramAuthSettings({ twoFactorEnabled }: Props) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [status, setStatus] = useState<TelegramAuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [action, setAction] = useState<'link' | 'unlink' | null>(null);
  const [password, setPassword] = useState('');
  const [twoFactorCode, setTwoFactorCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [challenge, setChallenge] = useState<TelegramChallenge | null>(null);

  const fetchStatus = useCallback(async () => {
    const result = await HttpUtil.get<TelegramAuthStatus>(
      '/panel/api/setting/telegramAuth/status',
      undefined,
      { silent: true },
    );
    if (result.success && result.obj) setStatus(result.obj);
    setLoading(false);
  }, []);

  useEffect(() => {
    (async () => {
      await fetchStatus();
    })();
  }, [fetchStatus]);

  useEffect(() => {
    if (!challenge) return;
    let active = true;
    const poll = async () => {
      if (Date.now() >= challenge.expiresAt) {
        setChallenge(null);
        messageApi.warning(t('pages.settings.security.telegramExpired'));
        return;
      }
      const result = await HttpUtil.get<TelegramAuthStatus>(
        '/panel/api/setting/telegramAuth/status',
        undefined,
        { silent: true },
      );
      if (!active || !result.success || !result.obj) return;
      setStatus(result.obj);
      if (result.obj.linked) {
        setChallenge(null);
        messageApi.success(t('pages.settings.security.telegramLinked'));
      }
    };
    const timer = window.setInterval(() => void poll(), POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [challenge, messageApi, t]);

  function openAction(next: 'link' | 'unlink') {
    setPassword('');
    setTwoFactorCode('');
    setAction(next);
  }

  async function submitAction() {
    if (!action || !password || (twoFactorEnabled && !twoFactorCode)) return;
    setSubmitting(true);
    try {
      const result = await HttpUtil.post<TelegramChallenge>(
        `/panel/api/setting/telegramAuth/${action}`,
        { password, twoFactorCode },
      );
      if (!result.success) return;
      if (action === 'link' && result.obj?.code) {
        setChallenge(result.obj);
      } else if (action === 'unlink') {
        window.location.replace(window.X_UI_BASE_PATH || '/');
        return;
      }
      setAction(null);
      setPassword('');
      setTwoFactorCode('');
    } finally {
      setSubmitting(false);
    }
  }

  async function copyCommand() {
    if (!challenge) return;
    const copied = await ClipboardManager.copyText(`/link ${challenge.code}`);
    if (copied) messageApi.success(t('copied'));
    else messageApi.error(t('copyFail'));
  }

  const botLink =
    challenge && status?.botUsername && /^[a-zA-Z0-9_]{5,32}$/.test(status.botUsername)
      ? `https://t.me/${status.botUsername}?start=link_${challenge.code}`
      : null;

  return (
    <>
      {messageContextHolder}
      <Spin spinning={loading}>
        <SettingListItem
          paddings="small"
          title={t('pages.settings.security.telegramTitle')}
          description={
            status?.available
              ? t('pages.settings.security.telegramDescription')
              : t('pages.settings.security.telegramUnavailable')
          }
        >
          {status && (status.available || status.linked) && (
            <Space>
              {status.linked && (
                <span>
                  {t('pages.settings.security.telegramAccount', { id: status.telegramUserId })}
                </span>
              )}
              <Button
                type={status.linked ? 'default' : 'primary'}
                danger={status.linked}
                onClick={() => openAction(status.linked ? 'unlink' : 'link')}
              >
                {t(
                  status.linked
                    ? 'pages.settings.security.telegramUnlink'
                    : 'pages.settings.security.telegramLink',
                )}
              </Button>
            </Space>
          )}
        </SettingListItem>
        {challenge && !status?.linked && (
          <div className="telegram-auth-challenge">
            <p>{t('pages.settings.security.telegramLinkInstruction')}</p>
            <div className="telegram-auth-command">
              <code>/link {challenge.code}</code>
              <Button size="small" onClick={copyCommand}>
                {t('copy')}
              </Button>
            </div>
            {botLink && (
              <Button type="primary" href={botLink} target="_blank" rel="noopener noreferrer">
                {t('pages.settings.security.telegramOpenBot')}
              </Button>
            )}
            <p>{t('pages.settings.security.telegramWaiting')}</p>
            <Button onClick={() => setChallenge(null)}>{t('cancel')}</Button>
          </div>
        )}
      </Spin>

      <Modal
        open={action !== null}
        title={t(
          action === 'unlink'
            ? 'pages.settings.security.telegramUnlink'
            : 'pages.settings.security.telegramLink',
        )}
        okText={t('confirm')}
        okButtonProps={{ disabled: !password || (twoFactorEnabled && !twoFactorCode) }}
        confirmLoading={submitting}
        onOk={submitAction}
        onCancel={() => setAction(null)}
        destroyOnHidden
      >
        <p>{t('pages.settings.security.telegramVerify')}</p>
        <Form layout="vertical">
          <Form.Item label={t('password')} htmlFor="telegram-auth-password" required>
            <Input.Password
              id="telegram-auth-password"
              value={password}
              autoComplete="current-password"
              onChange={(event) => setPassword(event.target.value)}
            />
          </Form.Item>
          {twoFactorEnabled && (
            <Form.Item label={t('twoFactorCode')} htmlFor="telegram-auth-code" required>
              <Input
                id="telegram-auth-code"
                value={twoFactorCode}
                autoComplete="one-time-code"
                onChange={(event) => setTwoFactorCode(event.target.value)}
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </>
  );
}
