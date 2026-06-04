import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Empty,
  Form,
  Input,
  Modal,
  Space,
  Spin,
  Switch,
  Tabs,
  message,
} from 'antd';
import { ApiOutlined, SafetyOutlined, UserOutlined } from '@ant-design/icons';
import { ClipboardManager, HttpUtil, RandomUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from './catTabLabel';
import TwoFactorModal from './TwoFactorModal';
import './SecurityTab.css';

interface ApiMsg<T = unknown> {
  success?: boolean;
  msg?: string;
  obj?: T;
}

interface ApiTokenRow {
  id: number;
  name: string;
  enabled: boolean;
  createdAt: number;
}

interface SecurityTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

type TfaType = 'set' | 'confirm';

interface TfaState {
  open: boolean;
  title: string;
  description: string;
  token: string;
  type: TfaType;
  onConfirm: (success: boolean, code?: string) => void;
}

const TFA_INITIAL: TfaState = {
  open: false,
  title: '',
  description: '',
  token: '',
  type: 'set',
  onConfirm: () => {},
};

export default function SecurityTab({ allSetting, updateSetting }: SecurityTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [tfa, setTfa] = useState<TfaState>(TFA_INITIAL);
  const [user, setUser] = useState({
    oldUsername: '',
    oldPassword: '',
    newUsername: '',
    newPassword: '',
  });
  const [updating, setUpdating] = useState(false);

  const [apiTokens, setApiTokens] = useState<ApiTokenRow[]>([]);
  const [apiTokensLoading, setApiTokensLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [creating, setCreating] = useState(false);
  const [createdToken, setCreatedToken] = useState<{ name: string; token: string } | null>(null);

  const openTfa = useCallback((opts: Omit<TfaState, 'open'>) => {
    setTfa({ ...opts, open: true });
  }, []);

  const onTfaConfirm = useCallback((success: boolean, code?: string) => {
    tfa.onConfirm(success, code);
  }, [tfa]);

  function updateUserField<K extends keyof typeof user>(key: K, value: string) {
    setUser((prev) => ({ ...prev, [key]: value }));
  }

  const sendUpdateUser = useCallback(async () => {
    setUpdating(true);
    try {
      const msg = await HttpUtil.post('/panel/setting/updateUser', user) as ApiMsg;
      if (msg?.success) {
        await HttpUtil.post('/logout');
        const basePath = window.X_UI_BASE_PATH || '/';
        window.location.replace(basePath);
      }
    } finally {
      setUpdating(false);
    }
  }, [user]);

  function onUpdateUserClick() {
    if (allSetting.twoFactorEnable) {
      openTfa({
        title: t('pages.settings.security.twoFactorModalChangeCredentialsTitle'),
        description: t('pages.settings.security.twoFactorModalChangeCredentialsStep'),
        token: allSetting.twoFactorToken,
        type: 'confirm',
        onConfirm: (ok: boolean) => { if (ok) sendUpdateUser(); },
      });
    } else {
      sendUpdateUser();
    }
  }

  const loadApiTokens = useCallback(async () => {
    setApiTokensLoading(true);
    try {
      const msg = await HttpUtil.get('/panel/setting/apiTokens') as ApiMsg<ApiTokenRow[]>;
      if (msg?.success) setApiTokens(Array.isArray(msg.obj) ? msg.obj : []);
    } finally {
      setApiTokensLoading(false);
    }
  }, []);

  useEffect(() => {
     
    loadApiTokens();
  }, [loadApiTokens]);

  async function copyToken(token: string) {
    if (!token) return;
    const ok = await ClipboardManager.copyText(token);
    if (ok) messageApi.success(t('copySuccess'));
    else messageApi.error(t('copyFail') ?? 'Copy failed');
  }

  function openCreateModal() {
    setCreateName('');
    setCreateOpen(true);
  }

  async function confirmCreateToken() {
    const name = createName.trim();
    if (!name) {
      messageApi.error(t('pages.settings.security.apiTokenNameRequired') || 'Name is required');
      return;
    }
    setCreating(true);
    try {
      const msg = await HttpUtil.post('/panel/setting/apiTokens/create', { name }) as ApiMsg<{ token?: string }>;
      if (msg?.success) {
        setCreateOpen(false);
        await loadApiTokens();
        if (msg.obj?.token) {
          setCreatedToken({ name, token: msg.obj.token });
        }
      }
    } finally {
      setCreating(false);
    }
  }

  function confirmDeleteToken(row: ApiTokenRow) {
    modal.confirm({
      title: `${t('delete')} "${row.name}"?`,
      content: t('pages.settings.security.apiTokenDeleteWarning')
        || 'Any caller using this token will stop authenticating immediately.',
      okText: t('delete'),
      cancelText: t('cancel'),
      okType: 'danger',
      onOk: async () => {
        const msg = await HttpUtil.post(`/panel/setting/apiTokens/delete/${row.id}`) as ApiMsg;
        if (msg?.success) await loadApiTokens();
      },
    });
  }

  async function toggleTokenEnabled(row: ApiTokenRow) {
    const target = !row.enabled;
    const msg = await HttpUtil.post(`/panel/setting/apiTokens/setEnabled/${row.id}`, { enabled: target }) as ApiMsg;
    if (msg?.success) {
      setApiTokens((prev) => prev.map((r) => (r.id === row.id ? { ...r, enabled: target } : r)));
    }
  }

  function formatTokenDate(ts: number): string {
    if (!ts) return '';
    return new Date(ts * 1000).toLocaleString();
  }

  function toggleTwoFactor() {
    if (!allSetting.twoFactorEnable) {
      const newToken = RandomUtil.randomBase32String();
      openTfa({
        title: t('pages.settings.security.twoFactorModalSetTitle'),
        description: '',
        token: newToken,
        type: 'set',
        onConfirm: (ok: boolean) => {
          if (ok) {
            messageApi.success(t('pages.settings.security.twoFactorModalSetSuccess'));
            updateSetting({ twoFactorToken: newToken, twoFactorEnable: true });
          } else {
            updateSetting({ twoFactorEnable: false });
          }
        },
      });
    } else {
      openTfa({
        title: t('pages.settings.security.twoFactorModalDeleteTitle'),
        description: t('pages.settings.security.twoFactorModalRemoveStep'),
        token: allSetting.twoFactorToken,
        type: 'confirm',
        onConfirm: (ok: boolean) => {
          if (!ok) return;
          messageApi.success(t('pages.settings.security.twoFactorModalDeleteSuccess'));
          updateSetting({ twoFactorEnable: false, twoFactorToken: '' });
        },
      });
    }
  }

  return (
    <>
      {messageContextHolder}
      {modalContextHolder}
      <Tabs defaultActiveKey="1" items={[
        {
          key: '1',
          label: catTabLabel(<UserOutlined />, t('pages.settings.security.admin'), isMobile),
          children: (
            <>
              <SettingListItem paddings="small" title={t('pages.settings.oldUsername')}>
                <Input value={user.oldUsername} autoComplete="username"
                  onChange={(e) => updateUserField('oldUsername', e.target.value)} />
              </SettingListItem>
              <SettingListItem paddings="small" title={t('pages.settings.currentPassword')}>
                <Input.Password value={user.oldPassword} autoComplete="current-password"
                  onChange={(e) => updateUserField('oldPassword', e.target.value)} />
              </SettingListItem>
              <SettingListItem paddings="small" title={t('pages.settings.newUsername')}>
                <Input value={user.newUsername}
                  onChange={(e) => updateUserField('newUsername', e.target.value)} />
              </SettingListItem>
              <SettingListItem paddings="small" title={t('pages.settings.newPassword')}>
                <Input.Password value={user.newPassword} autoComplete="new-password"
                  onChange={(e) => updateUserField('newPassword', e.target.value)} />
              </SettingListItem>
              <div className="security-actions">
                <Space style={{ padding: '0 20px' }}>
                  <Button type="primary" loading={updating} onClick={onUpdateUserClick}>
                    {t('confirm')}
                  </Button>
                </Space>
              </div>
            </>
          ),
        },
        {
          key: '2',
          label: catTabLabel(<SafetyOutlined />, t('pages.settings.security.twoFactor'), isMobile),
          children: (
            <SettingListItem
              paddings="small"
              title={t('pages.settings.security.twoFactorEnable')}
              description={t('pages.settings.security.twoFactorEnableDesc')}
            >
              <Switch checked={allSetting.twoFactorEnable} onClick={toggleTwoFactor} />
            </SettingListItem>
          ),
        },
        {
          key: '3',
          label: catTabLabel(<ApiOutlined />, t('pages.nodes.apiToken'), isMobile),
          children: (
            <div className="api-token-section">
              <div className="api-token-header">
                <p className="api-token-hint">{t('pages.nodes.apiTokenHint')}</p>
                <Button type="primary" size="small" onClick={openCreateModal}>
                  + {t('pages.settings.security.apiTokenNew') || 'New token'}
                </Button>
              </div>
              <Spin spinning={apiTokensLoading}>
                {!apiTokens.length && !apiTokensLoading && (
                  <Empty description={t('pages.settings.security.apiTokenEmpty') || 'No tokens yet'} />
                )}
                {apiTokens.map((row) => (
                  <div key={row.id} className={`api-token-row${row.enabled ? '' : ' disabled'}`}>
                    <div className="api-token-row-head">
                      <div className="api-token-name-wrap">
                        <span className="api-token-name">{row.name}</span>
                        <span className="api-token-created">{formatTokenDate(row.createdAt)}</span>
                      </div>
                      <div className="api-token-actions">
                        <Switch size="small" checked={row.enabled} onChange={() => toggleTokenEnabled(row)} />
                        <Button size="small" danger type="text" onClick={() => confirmDeleteToken(row)}>
                          {t('delete')}
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}
              </Spin>
            </div>
          ),
        },
      ]} />

      <Modal
        open={createOpen}
        title={t('pages.settings.security.apiTokenNew') || 'New API token'}
        confirmLoading={creating}
        okText={t('confirm')}
        cancelText={t('cancel')}
        onOk={confirmCreateToken}
        onCancel={() => setCreateOpen(false)}
      >
        <Form layout="vertical">
          <Form.Item label={t('pages.settings.security.apiTokenName') || 'Name'} required>
            <Input
              value={createName}
              maxLength={64}
              placeholder={t('pages.settings.security.apiTokenNamePlaceholder') || 'e.g. central-panel-a'}
              onChange={(e) => setCreateName(e.target.value)}
              onPressEnter={confirmCreateToken}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={!!createdToken}
        title={t('pages.settings.security.apiTokenCreatedTitle') || 'Token created'}
        okText={t('done')}
        onOk={() => setCreatedToken(null)}
        onCancel={() => setCreatedToken(null)}
        cancelButtonProps={{ style: { display: 'none' } }}
      >
        <p className="api-token-created-notice">
          {t('pages.settings.security.apiTokenCreatedNotice')
            || 'Copy this token now. For security it is not stored in readable form and will not be shown again.'}
        </p>
        <div className="api-token-value-wrap">
          <code className="api-token-value">{createdToken?.token}</code>
          <Button size="small" type="primary" onClick={() => createdToken && copyToken(createdToken.token)}>
            {t('copy')}
          </Button>
        </div>
      </Modal>

      <TwoFactorModal
        open={tfa.open}
        title={tfa.title}
        description={tfa.description}
        token={tfa.token}
        type={tfa.type}
        onConfirm={onTfaConfirm}
        onOpenChange={(open) => setTfa((prev) => ({ ...prev, open }))}
      />
    </>
  );
}
