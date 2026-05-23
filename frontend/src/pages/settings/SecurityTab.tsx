import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Collapse,
  Empty,
  Form,
  Input,
  Modal,
  Space,
  Spin,
  Switch,
  message,
} from 'antd';
import { ClipboardManager, HttpUtil, RandomUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import SettingListItem from '@/components/SettingListItem';
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
  token: string;
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
  const [visibleTokenIds, setVisibleTokenIds] = useState<Set<number>>(() => new Set());
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [creating, setCreating] = useState(false);

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

  function toggleTokenVisibility(id: number) {
    setVisibleTokenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

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
      const msg = await HttpUtil.post('/panel/setting/apiTokens/create', { name }) as ApiMsg<{ id?: number }>;
      if (msg?.success) {
        setCreateOpen(false);
        await loadApiTokens();
        if (msg.obj?.id != null) {
          const id = msg.obj.id;
          setVisibleTokenIds((prev) => {
            const next = new Set(prev);
            next.add(id);
            return next;
          });
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

  function maskToken(token: string): string {
    if (!token) return '';
    return '•'.repeat(Math.min(token.length, 24));
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
      <Collapse defaultActiveKey="1" items={[
        {
          key: '1',
          label: t('pages.settings.security.admin'),
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
          label: t('pages.settings.security.twoFactor'),
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
          label: t('pages.nodes.apiToken'),
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
                    <div className="api-token-value-wrap">
                      <code className="api-token-value">
                        {visibleTokenIds.has(row.id) ? row.token : maskToken(row.token)}
                      </code>
                      <Button size="small" onClick={() => toggleTokenVisibility(row.id)}>
                        {visibleTokenIds.has(row.id)
                          ? (t('pages.settings.security.hide') || 'Hide')
                          : (t('pages.settings.security.show') || 'Show')}
                      </Button>
                      <Button size="small" onClick={() => copyToken(row.token)}>{t('copy')}</Button>
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
