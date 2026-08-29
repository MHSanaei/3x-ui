/* oxlint-disable react/set-state-in-effect -- fork code predating the oxlint migration: the
   latest-ref idiom and effect-driven state here are deliberate and
   VPS-verified. Revisit as a standalone refactor, not during a version sync. */
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CloudDownloadOutlined,
  DeleteOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SyncOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';

import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { onNumber } from '@/utils/onNumber';
import { DefaultSettingTag, SettingListItem } from '@/components/ui';

interface FrontProxyTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

interface FrontProxyCertStatus {
  state: 'obtaining' | 'obtained' | 'failed' | '';
  domain?: string;
  error?: string;
  notAfter?: string;
}

interface FrontProxyStatus {
  running: boolean;
  port: number;
  templates: string[];
  decoyUploaded: boolean;
  cert: FrontProxyCertStatus;
}

interface AdGuardStatus {
  installed: boolean;
  running: boolean;
  webPort: number;
  dnsPort: number;
  user: string;
  password?: string;
  isDecoy: boolean;
  lastLog?: string;
}

export default function FrontProxyTab({ allSetting, updateSetting }: FrontProxyTabProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [status, setStatus] = useState<FrontProxyStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [adGuard, setAdGuard] = useState<AdGuardStatus | null>(null);
  const [adGuardLoading, setAdGuardLoading] = useState(false);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.post<FrontProxyStatus>('/panel/api/xray/frontproxy/status');
    if (msg?.success && msg.obj) setStatus(msg.obj);
  }, []);

  const fetchAdGuard = useCallback(async () => {
    const msg = await HttpUtil.post<AdGuardStatus>('/panel/api/xray/adguard/status');
    if (msg?.success && msg.obj) setAdGuard(msg.obj);
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  useEffect(() => {
    fetchAdGuard();
  }, [fetchAdGuard]);

  useEffect(() => {
    if (status?.cert?.state !== 'obtaining') return;
    const id = setInterval(fetchStatus, 1500);
    return () => clearInterval(id);
  }, [status?.cert?.state, fetchStatus]);

  const proxiedSubURI = useMemo(() => {
    const domain = (allSetting.frontProxyDomain ?? '').trim();
    if (!domain) return '';
    const path = (allSetting.subPath ?? '/').trim();
    return `https://${domain}${path.startsWith('/') ? path : `/${path}`}`;
  }, [allSetting.frontProxyDomain, allSetting.subPath]);

  const proxiedPanelURL = useMemo(() => {
    const domain = (allSetting.frontProxyDomain ?? '').trim();
    if (!domain) return '';
    const base = (allSetting.webBasePath ?? '/').trim();
    const rooted = base.startsWith('/') ? base : `/${base}`;
    return `https://${domain}${rooted.endsWith('/') ? rooted : `${rooted}/`}panel/`;
  }, [allSetting.frontProxyDomain, allSetting.webBasePath]);

  const dohURL = useMemo(() => {
    const domain = (allSetting.frontProxyDomain ?? '').trim();
    return domain ? `https://${domain}/dns-query` : '';
  }, [allSetting.frontProxyDomain]);

  const subURIHasPort = useMemo(() => {
    const current = (allSetting.subURI ?? '').trim();
    if (!current) return true;
    try {
      return new URL(current).port !== '';
    } catch {
      return false;
    }
  }, [allSetting.subURI]);

  async function dispatch(action: 'start' | 'stop' | 'removeDecoy', okKey: string) {
    setLoading(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/xray/frontproxy/${action}`);
      if (msg?.success) {
        messageApi.success(t(okKey));
      } else {
        messageApi.error(msg?.msg || t('fail'));
      }
      await fetchStatus();
    } finally {
      setLoading(false);
    }
  }

  async function dispatchAdGuard(
    action: 'install' | 'uninstall' | 'start' | 'stop',
    okKey: string,
  ) {
    setAdGuardLoading(true);
    try {
      const msg = await HttpUtil.post<AdGuardStatus>(`/panel/api/xray/adguard/${action}`);
      if (msg?.success) {
        messageApi.success(t(okKey));
        if (msg.obj) setAdGuard(msg.obj);
      } else {
        messageApi.error(msg?.msg || t('fail'));
        await fetchAdGuard();
      }
      await fetchStatus();
    } finally {
      setAdGuardLoading(false);
    }
  }

  function uploadDecoy() {
    const fileInput = document.createElement('input');
    fileInput.type = 'file';
    fileInput.accept = '.zip';
    fileInput.addEventListener('change', async (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (!file) return;

      const formData = new FormData();
      formData.append('site', file);

      setLoading(true);
      try {
        const msg = await HttpUtil.post<FrontProxyStatus>(
          '/panel/api/xray/frontproxy/decoy/upload',
          formData,
          {
            headers: { 'Content-Type': 'multipart/form-data' },
          },
        );
        if (msg?.success) {
          messageApi.success(t('pages.settings.frontProxy.decoyUploaded'));
          if (msg.obj) setStatus(msg.obj);
        } else {
          messageApi.error(msg?.msg || t('fail'));
        }
      } finally {
        setLoading(false);
      }
    });
    fileInput.click();
  }

  const target = `127.0.0.1:${allSetting.frontProxyPort}`;

  return (
    <>
      {messageContextHolder}
      <Alert
        type="info"
        showIcon
        style={{ margin: '0 20px 12px' }}
        title={t('pages.settings.frontProxy.introTitle')}
        description={
          <>
            <div>{t('pages.settings.frontProxy.introDesc')}</div>
            <div style={{ marginTop: 8 }}>
              {t('pages.settings.frontProxy.realityTargetHint')}{' '}
              <Typography.Text code copyable>
                {target}
              </Typography.Text>
            </div>
          </>
        }
      />

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.state')}
        description={t('pages.settings.frontProxy.stateDesc')}
      >
        <Space wrap>
          {status?.running ? (
            <Tag color="green">{t('pages.settings.frontProxy.running')}</Tag>
          ) : (
            <Tag color="orange">{t('pages.settings.frontProxy.stopped')}</Tag>
          )}
          {status?.running ? (
            <Button
              danger
              icon={<PauseCircleOutlined />}
              loading={loading}
              onClick={() => dispatch('stop', 'pages.settings.frontProxy.stoppedToast')}
            >
              {t('pages.settings.frontProxy.stopButton')}
            </Button>
          ) : (
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              loading={loading}
              onClick={() => dispatch('start', 'pages.settings.frontProxy.startedToast')}
            >
              {t('pages.settings.frontProxy.startButton')}
            </Button>
          )}
          <Button aria-label={t('refresh')} icon={<ReloadOutlined />} onClick={fetchStatus} />
        </Space>
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.enable')}
        description={t('pages.settings.frontProxy.enableDesc')}
      >
        <Switch
          checked={allSetting.frontProxyEnable}
          onChange={(v) => updateSetting({ frontProxyEnable: v })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.listen')}
        description={t('pages.settings.frontProxy.listenDesc')}
      >
        <Input
          value={allSetting.frontProxyListen}
          placeholder="127.0.0.1"
          onChange={(e) => updateSetting({ frontProxyListen: e.target.value })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.port')}
        badge={<DefaultSettingTag settingKey="frontProxyPort" value={allSetting.frontProxyPort} />}
        description={t('pages.settings.frontProxy.portDesc')}
      >
        <InputNumber
          value={allSetting.frontProxyPort}
          min={1}
          max={65535}
          style={{ width: '100%' }}
          onChange={onNumber((v) => updateSetting({ frontProxyPort: v }))}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.upstreams')}
        description={t('pages.settings.frontProxy.upstreamsDesc')}
      >
        <Space direction="vertical" size={4}>
          <Typography.Text type="secondary">
            {t('pages.settings.frontProxy.upstreamPanel')}
            {': '}
            <Typography.Text code>{allSetting.webBasePath}</Typography.Text>
            {' → 127.0.0.1:'}
            {allSetting.webPort}
          </Typography.Text>
          <Typography.Text type="secondary">
            {t('pages.settings.frontProxy.upstreamSub')}
            {': '}
            {allSetting.subEnable ? (
              <>
                <Typography.Text code>{allSetting.subPath}</Typography.Text>
                {' → 127.0.0.1:'}
                {allSetting.subPort}
              </>
            ) : (
              t('pages.settings.frontProxy.upstreamSubOff')
            )}
          </Typography.Text>
        </Space>
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.certMode')}
        description={t('pages.settings.frontProxy.certModeDesc')}
      >
        <Select
          value={allSetting.frontProxyCertMode}
          style={{ width: '100%' }}
          onChange={(v) => updateSetting({ frontProxyCertMode: v })}
          options={[
            { value: 'manual', label: t('pages.settings.frontProxy.certManual') },
            { value: 'auto', label: t('pages.settings.frontProxy.certAuto') },
          ]}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.domain')}
        description={t('pages.settings.frontProxy.domainDesc')}
      >
        <Input
          value={allSetting.frontProxyDomain}
          placeholder="panel.example.com"
          onChange={(e) => updateSetting({ frontProxyDomain: e.target.value })}
        />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.panelUrl')}
        description={t('pages.settings.frontProxy.panelUrlDesc')}
      >
        <Typography.Text code copyable={!!proxiedPanelURL}>
          {proxiedPanelURL || t('pages.settings.frontProxy.subUriNeedsDomain')}
        </Typography.Text>
      </SettingListItem>

      {allSetting.subEnable && (
        <SettingListItem
          paddings="small"
          title={t('pages.settings.frontProxy.subUri')}
          description={t('pages.settings.frontProxy.subUriDesc')}
        >
          <Space direction="vertical" size={6} style={{ width: '100%' }}>
            <Typography.Text code copyable={!!proxiedSubURI}>
              {proxiedSubURI || t('pages.settings.frontProxy.subUriNeedsDomain')}
            </Typography.Text>
            <Button
              size="small"
              disabled={!proxiedSubURI || allSetting.subURI === proxiedSubURI}
              onClick={() => updateSetting({ subURI: proxiedSubURI })}
            >
              {t('pages.settings.frontProxy.subUriApply')}
            </Button>
            {subURIHasPort && (
              <Typography.Text type="warning">
                {t('pages.settings.frontProxy.subUriHasPort')}
              </Typography.Text>
            )}
          </Space>
        </SettingListItem>
      )}

      {allSetting.frontProxyCertMode === 'auto' ? (
        <>
          <Alert
            type="warning"
            showIcon
            style={{ margin: '0 20px 12px' }}
            description={t('pages.settings.frontProxy.acmeHttp01Hint')}
          />
          <SettingListItem
            paddings="small"
            title={t('pages.settings.frontProxy.email')}
            description={t('pages.settings.frontProxy.emailDesc')}
          >
            <Input
              value={allSetting.frontProxyEmail}
              placeholder="admin@example.com"
              onChange={(e) => updateSetting({ frontProxyEmail: e.target.value })}
            />
          </SettingListItem>
        </>
      ) : (
        <Alert
          type="warning"
          showIcon
          style={{ margin: '0 20px 12px' }}
          description={t('pages.settings.frontProxy.certManualHint')}
        />
      )}

      {status?.cert && status.cert.state !== '' && (
        <SettingListItem
          paddings="small"
          title={t('pages.settings.frontProxy.certStatus')}
          description={t('pages.settings.frontProxy.certStatusDesc')}
        >
          {status.cert.state === 'obtaining' && (
            <Tag icon={<SyncOutlined spin />} color="processing">
              {t('pages.settings.frontProxy.certStatusObtaining')}
            </Tag>
          )}
          {status.cert.state === 'obtained' && (
            <Space wrap>
              <Tag color="green">{t('pages.settings.frontProxy.certStatusObtained')}</Tag>
              {status.cert.notAfter && (
                <Typography.Text type="secondary">
                  {t('pages.settings.frontProxy.certStatusValidUntil', {
                    date: dayjs(status.cert.notAfter).format('YYYY-MM-DD HH:mm:ss'),
                  })}
                </Typography.Text>
              )}
            </Space>
          )}
          {status.cert.state === 'failed' && (
            <Alert
              type="error"
              showIcon
              style={{ width: '100%' }}
              message={t('pages.settings.frontProxy.certStatusFailed')}
              description={status.cert.error}
            />
          )}
        </SettingListItem>
      )}

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.decoyMode')}
        description={t('pages.settings.frontProxy.decoyModeDesc')}
      >
        <Select
          value={allSetting.frontProxyDecoyMode}
          style={{ width: '100%' }}
          onChange={(v) => updateSetting({ frontProxyDecoyMode: v })}
          options={[
            { value: 'template', label: t('pages.settings.frontProxy.decoyTemplateMode') },
            { value: 'upload', label: t('pages.settings.frontProxy.decoyUploadMode') },
            { value: 'proxy', label: t('pages.settings.frontProxy.decoyProxyMode') },
            { value: 'adguard', label: t('pages.settings.frontProxy.decoyAdGuardMode') },
          ]}
        />
      </SettingListItem>

      {allSetting.frontProxyDecoyMode === 'template' && (
        <SettingListItem
          paddings="small"
          title={t('pages.settings.frontProxy.decoyTemplate')}
          description={t('pages.settings.frontProxy.decoyTemplateDesc')}
        >
          <Select
            value={allSetting.frontProxyDecoyTemplate}
            style={{ width: '100%' }}
            onChange={(v) => updateSetting({ frontProxyDecoyTemplate: v })}
            options={(status?.templates ?? [allSetting.frontProxyDecoyTemplate]).map((name) => ({
              value: name,
              label: t(`pages.settings.frontProxy.templates.${name}`, { defaultValue: name }),
            }))}
          />
        </SettingListItem>
      )}

      {allSetting.frontProxyDecoyMode === 'upload' && (
        <SettingListItem
          paddings="small"
          title={t('pages.settings.frontProxy.decoyUpload')}
          description={t('pages.settings.frontProxy.decoyUploadDesc')}
        >
          <Space wrap>
            {status?.decoyUploaded ? (
              <Tag color="green">{t('pages.settings.frontProxy.decoyPresent')}</Tag>
            ) : (
              <Tag color="orange">{t('pages.settings.frontProxy.decoyMissing')}</Tag>
            )}
            <Button icon={<UploadOutlined />} loading={loading} onClick={uploadDecoy}>
              {t('pages.settings.frontProxy.decoyUploadButton')}
            </Button>
            {status?.decoyUploaded && (
              <Popconfirm
                title={t('pages.settings.frontProxy.decoyRemoveConfirm')}
                okText={t('delete')}
                okType="danger"
                cancelText={t('cancel')}
                onConfirm={() => dispatch('removeDecoy', 'pages.settings.frontProxy.decoyRemoved')}
              >
                <Button danger icon={<DeleteOutlined />} loading={loading}>
                  {t('delete')}
                </Button>
              </Popconfirm>
            )}
          </Space>
        </SettingListItem>
      )}

      {allSetting.frontProxyDecoyMode === 'proxy' && (
        <SettingListItem
          paddings="small"
          title={t('pages.settings.frontProxy.decoyProxyUrl')}
          description={t('pages.settings.frontProxy.decoyProxyUrlDesc')}
        >
          <Input
            value={allSetting.frontProxyDecoyProxyURL}
            placeholder="https://example.com"
            onChange={(e) => updateSetting({ frontProxyDecoyProxyURL: e.target.value })}
          />
        </SettingListItem>
      )}

      {allSetting.frontProxyDecoyMode === 'adguard' && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ margin: '0 20px 12px' }}
            description={t('pages.settings.frontProxy.adGuardHint')}
          />
          <SettingListItem
            paddings="small"
            title={t('pages.settings.frontProxy.adGuard')}
            description={t('pages.settings.frontProxy.adGuardDesc')}
          >
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space wrap>
                {!adGuard?.installed && (
                  <Tag color="orange">{t('pages.settings.frontProxy.adGuardMissing')}</Tag>
                )}
                {adGuard?.installed && adGuard.running && (
                  <Tag color="green">{t('pages.settings.frontProxy.adGuardRunning')}</Tag>
                )}
                {adGuard?.installed && !adGuard.running && (
                  <Tag color="orange">{t('pages.settings.frontProxy.adGuardStopped')}</Tag>
                )}
                {!adGuard?.installed && (
                  <Button
                    type="primary"
                    icon={<CloudDownloadOutlined />}
                    loading={adGuardLoading}
                    onClick={() =>
                      dispatchAdGuard('install', 'pages.settings.frontProxy.adGuardInstalledToast')
                    }
                  >
                    {t('pages.settings.frontProxy.adGuardInstallButton')}
                  </Button>
                )}
                {adGuard?.installed && adGuard.running && (
                  <Button
                    danger
                    icon={<PauseCircleOutlined />}
                    loading={adGuardLoading}
                    onClick={() =>
                      dispatchAdGuard('stop', 'pages.settings.frontProxy.adGuardStoppedToast')
                    }
                  >
                    {t('pages.settings.frontProxy.stopButton')}
                  </Button>
                )}
                {adGuard?.installed && !adGuard.running && (
                  <Button
                    type="primary"
                    icon={<PlayCircleOutlined />}
                    loading={adGuardLoading}
                    onClick={() =>
                      dispatchAdGuard('start', 'pages.settings.frontProxy.adGuardStartedToast')
                    }
                  >
                    {t('pages.settings.frontProxy.startButton')}
                  </Button>
                )}
                {adGuard?.installed && (
                  <Popconfirm
                    title={t('pages.settings.frontProxy.adGuardRemoveConfirm')}
                    okText={t('delete')}
                    okType="danger"
                    cancelText={t('cancel')}
                    onConfirm={() =>
                      dispatchAdGuard('uninstall', 'pages.settings.frontProxy.adGuardRemovedToast')
                    }
                  >
                    <Button danger icon={<DeleteOutlined />} loading={adGuardLoading}>
                      {t('delete')}
                    </Button>
                  </Popconfirm>
                )}
                <Button
                  aria-label={t('refresh')}
                  icon={<ReloadOutlined />}
                  onClick={fetchAdGuard}
                />
              </Space>

              {adGuard?.installed && (
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    {t('pages.settings.frontProxy.adGuardLogin')}
                    {': '}
                    <Typography.Text code copyable>
                      {adGuard.user}
                    </Typography.Text>
                    {adGuard.password && (
                      <Typography.Text code copyable>
                        {adGuard.password}
                      </Typography.Text>
                    )}
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    {t('pages.settings.frontProxy.adGuardDoh')}
                    {': '}
                    <Typography.Text code copyable={!!dohURL}>
                      {dohURL || t('pages.settings.frontProxy.subUriNeedsDomain')}
                    </Typography.Text>
                  </Typography.Text>
                  <Typography.Text type="secondary">
                    {t('pages.settings.frontProxy.adGuardPorts', {
                      web: adGuard.webPort,
                      dns: adGuard.dnsPort,
                    })}
                  </Typography.Text>
                </Space>
              )}
            </Space>
          </SettingListItem>
        </>
      )}
    </>
  );
}
