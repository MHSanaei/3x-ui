import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Popconfirm, Select, Space, Switch, Tag, Typography, message } from 'antd';
import { DeleteOutlined, PauseCircleOutlined, PlayCircleOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import type { AllSetting } from '@/models/setting';
import { onNumber } from '@/utils/onNumber';
import { DefaultSettingTag, SettingListItem } from '@/components/ui';

interface FrontProxyTabProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
}

interface FrontProxyStatus {
  running: boolean;
  port: number;
  templates: string[];
  decoyUploaded: boolean;
}

export default function FrontProxyTab({ allSetting, updateSetting }: FrontProxyTabProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [status, setStatus] = useState<FrontProxyStatus | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchStatus = useCallback(async () => {
    const msg = await HttpUtil.post<FrontProxyStatus>('/panel/api/xray/frontproxy/status');
    if (msg?.success && msg.obj) setStatus(msg.obj);
  }, []);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const proxiedSubURI = useMemo(() => {
    const domain = (allSetting.frontProxyDomain ?? '').trim();
    if (!domain) return '';
    const path = (allSetting.subPath ?? '/').trim();
    return `https://${domain}${path.startsWith('/') ? path : `/${path}`}`;
  }, [allSetting.frontProxyDomain, allSetting.subPath]);

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
        const msg = await HttpUtil.post<FrontProxyStatus>('/panel/api/xray/frontproxy/decoy/upload', formData, {
          headers: { 'Content-Type': 'multipart/form-data' },
        });
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
        description={(
          <>
            <div>{t('pages.settings.frontProxy.introDesc')}</div>
            <div style={{ marginTop: 8 }}>
              {t('pages.settings.frontProxy.realityTargetHint')}{' '}
              <Typography.Text code copyable>{target}</Typography.Text>
            </div>
          </>
        )}
      />

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.state')} description={t('pages.settings.frontProxy.stateDesc')}>
        <Space wrap>
          {status?.running
            ? <Tag color="green">{t('pages.settings.frontProxy.running')}</Tag>
            : <Tag color="orange">{t('pages.settings.frontProxy.stopped')}</Tag>}
          {status?.running ? (
            <Button danger icon={<PauseCircleOutlined />} loading={loading} onClick={() => dispatch('stop', 'pages.settings.frontProxy.stoppedToast')}>
              {t('pages.settings.frontProxy.stopButton')}
            </Button>
          ) : (
            <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={() => dispatch('start', 'pages.settings.frontProxy.startedToast')}>
              {t('pages.settings.frontProxy.startButton')}
            </Button>
          )}
          <Button aria-label={t('refresh')} icon={<ReloadOutlined />} onClick={fetchStatus} />
        </Space>
      </SettingListItem>

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.enable')} description={t('pages.settings.frontProxy.enableDesc')}>
        <Switch checked={allSetting.frontProxyEnable} onChange={(v) => updateSetting({ frontProxyEnable: v })} />
      </SettingListItem>

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.listen')} description={t('pages.settings.frontProxy.listenDesc')}>
        <Input value={allSetting.frontProxyListen} placeholder="127.0.0.1"
          onChange={(e) => updateSetting({ frontProxyListen: e.target.value })} />
      </SettingListItem>

      <SettingListItem
        paddings="small"
        title={t('pages.settings.frontProxy.port')}
        badge={<DefaultSettingTag settingKey="frontProxyPort" value={allSetting.frontProxyPort} />}
        description={t('pages.settings.frontProxy.portDesc')}
      >
        <InputNumber value={allSetting.frontProxyPort} min={1} max={65535} style={{ width: '100%' }}
          onChange={onNumber((v) => updateSetting({ frontProxyPort: v }))} />
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
            ) : t('pages.settings.frontProxy.upstreamSubOff')}
          </Typography.Text>
        </Space>
      </SettingListItem>

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.certMode')} description={t('pages.settings.frontProxy.certModeDesc')}>
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

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.domain')} description={t('pages.settings.frontProxy.domainDesc')}>
        <Input value={allSetting.frontProxyDomain} placeholder="panel.example.com"
          onChange={(e) => updateSetting({ frontProxyDomain: e.target.value })} />
      </SettingListItem>

      {allSetting.subEnable && (
        <SettingListItem paddings="small" title={t('pages.settings.frontProxy.subUri')} description={t('pages.settings.frontProxy.subUriDesc')}>
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
              <Typography.Text type="warning">{t('pages.settings.frontProxy.subUriHasPort')}</Typography.Text>
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
          <SettingListItem paddings="small" title={t('pages.settings.frontProxy.email')} description={t('pages.settings.frontProxy.emailDesc')}>
            <Input value={allSetting.frontProxyEmail} placeholder="admin@example.com"
              onChange={(e) => updateSetting({ frontProxyEmail: e.target.value })} />
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

      <SettingListItem paddings="small" title={t('pages.settings.frontProxy.decoyMode')} description={t('pages.settings.frontProxy.decoyModeDesc')}>
        <Select
          value={allSetting.frontProxyDecoyMode}
          style={{ width: '100%' }}
          onChange={(v) => updateSetting({ frontProxyDecoyMode: v })}
          options={[
            { value: 'template', label: t('pages.settings.frontProxy.decoyTemplateMode') },
            { value: 'upload', label: t('pages.settings.frontProxy.decoyUploadMode') },
            { value: 'proxy', label: t('pages.settings.frontProxy.decoyProxyMode') },
          ]}
        />
      </SettingListItem>

      {allSetting.frontProxyDecoyMode === 'template' && (
        <SettingListItem paddings="small" title={t('pages.settings.frontProxy.decoyTemplate')} description={t('pages.settings.frontProxy.decoyTemplateDesc')}>
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
        <SettingListItem paddings="small" title={t('pages.settings.frontProxy.decoyUpload')} description={t('pages.settings.frontProxy.decoyUploadDesc')}>
          <Space wrap>
            {status?.decoyUploaded
              ? <Tag color="green">{t('pages.settings.frontProxy.decoyPresent')}</Tag>
              : <Tag color="orange">{t('pages.settings.frontProxy.decoyMissing')}</Tag>}
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
        <SettingListItem paddings="small" title={t('pages.settings.frontProxy.decoyProxyUrl')} description={t('pages.settings.frontProxy.decoyProxyUrlDesc')}>
          <Input value={allSetting.frontProxyDecoyProxyURL} placeholder="https://example.com"
            onChange={(e) => updateSetting({ frontProxyDecoyProxyURL: e.target.value })} />
        </SettingListItem>
      )}
    </>
  );
}
