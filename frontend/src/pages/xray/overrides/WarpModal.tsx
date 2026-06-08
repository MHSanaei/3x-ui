import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Collapse,
  Divider,
  Form,
  Input,
  message,
  Modal,
  Tag,
} from 'antd';
import { ApiOutlined, SyncOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { HttpUtil, SizeFormatter, ObjectUtil, Wireguard } from '@/utils';
import './WarpModal.css';

interface WarpModalProps {
  open: boolean;
  templateSettings: { outbounds?: { tag?: string }[] } | null;
  onClose: () => void;
  onAddOutbound: (outbound: Record<string, unknown>) => void;
  onResetOutbound: (payload: { index: number; outbound: Record<string, unknown> }) => void;
  onRemoveOutbound: (tag: string) => void;
}

interface WarpData {
  access_token?: string;
  device_id?: string;
  license_key?: string;
  private_key?: string;
  client_id?: string;
}

interface WarpConfig {
  name?: string;
  model?: string;
  enabled?: boolean;
  config?: {
    client_id?: string;
    interface?: { addresses?: { v4?: string; v6?: string } };
    peers?: { public_key?: string; endpoint?: { host?: string } }[];
  };
  account?: {
    account_type?: string;
    role?: string;
    premium_data?: number;
    quota?: number;
    usage?: number;
  };
}

function addressesFor(addrs: { v4?: string; v6?: string }): string[] {
  const out: string[] = [];
  if (addrs.v4) out.push(`${addrs.v4}/32`);
  if (addrs.v6) out.push(`${addrs.v6}/128`);
  return out;
}

function reservedFor(clientId?: string): number[] {
  if (!clientId) return [];
  const decoded = atob(clientId);
  const out: number[] = [];
  for (let i = 0; i < decoded.length; i += 1) out.push(decoded.charCodeAt(i));
  return out;
}

export default function WarpModal({
  open,
  templateSettings,
  onClose,
  onAddOutbound,
  onResetOutbound,
  onRemoveOutbound,
}: WarpModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [warpData, setWarpData] = useState<WarpData | null>(null);
  const [warpConfig, setWarpConfig] = useState<WarpConfig | null>(null);
  const [warpPlus, setWarpPlus] = useState('');
  const [updateInterval, setUpdateInterval] = useState<number>(0);
  const [licenseError, setLicenseError] = useState('');
  const [stagedOutbound, setStagedOutbound] = useState<Record<string, unknown> | null>(null);

  const warpOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag === 'warp');
  }, [templateSettings?.outbounds]);

  const collectConfig = useCallback(
    (data: WarpData | null, config: WarpConfig | null): Record<string, unknown> | null => {
      const cfg = config?.config;
      if (!cfg?.peers?.length) return null;
      const peer = cfg.peers[0];
      const outbound: Record<string, unknown> = {
        tag: 'warp',
        protocol: 'wireguard',
        settings: {
          mtu: 1420,
          secretKey: data?.private_key,
          address: addressesFor(cfg.interface?.addresses || {}),
          reserved: reservedFor(cfg.client_id ?? data?.client_id),
          domainStrategy: 'ForceIP',
          peers: [{ publicKey: peer.public_key, endpoint: peer.endpoint?.host }],
          noKernelTun: false,
        },
      };
      setStagedOutbound(outbound);
      return outbound;
    },
    [],
  );

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/warp/data');
      if (msg?.success) {
        const raw = msg.obj;
        setWarpData(raw && raw.length > 0 ? JSON.parse(raw) : null);
      }
      const settingMsg = await HttpUtil.post<Record<string, unknown>>('/panel/api/setting/all');
      if (settingMsg?.success && settingMsg.obj) {
        setUpdateInterval(Number(settingMsg.obj.warpUpdateInterval) || 0);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    setWarpConfig(null);
    setStagedOutbound(null);
    setLicenseError('');
    fetchData();
  }, [open, fetchData]);

  async function register() {
    setLoading(true);
    try {
      const keys = Wireguard.generateKeypair();
      const msg = await HttpUtil.post<string>('/panel/api/xray/warp/reg', keys);
      if (msg?.success && msg.obj) {
        const resp = JSON.parse(msg.obj);
        setWarpData(resp.data);
        setWarpConfig(resp.config);
        collectConfig(resp.data, resp.config);
      }
    } finally {
      setLoading(false);
    }
  }

  async function getConfig() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/warp/config');
      if (msg?.success && msg.obj) {
        const parsed = JSON.parse(msg.obj);
        setWarpConfig(parsed);
        collectConfig(warpData, parsed);
      }
    } finally {
      setLoading(false);
    }
  }

  async function changeIp() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/warp/changeIp');
      if (msg?.success && msg.obj) {
        const parsed = JSON.parse(msg.obj);
        setWarpData(parsed.data);
        setWarpConfig(parsed.config);
        const built = collectConfig(parsed.data, parsed.config);
        // The backend already persisted the new keys into the saved Xray
        // template; keep the in-memory editor in sync so a later template
        // save doesn't revert them to the old keys.
        if (built && warpOutboundIndex >= 0) {
          onResetOutbound({ index: warpOutboundIndex, outbound: built });
        }
        messageApi.success(t('pages.xray.warp.changeIpSuccess', 'WARP IP changed successfully!'));
      }
    } finally {
      setLoading(false);
    }
  }

  async function saveInterval() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/warp/interval', { interval: updateInterval });
      if (msg?.success) {
        messageApi.success(t('pages.setting.toasts.saveSuccess', 'Settings saved successfully'));
      }
    } finally {
      setLoading(false);
    }
  }

  async function updateLicense() {
    if (warpPlus.length < 26) return;
    setLoading(true);
    setLicenseError('');
    try {
      const msg = await HttpUtil.post<string>('/panel/api/xray/warp/license', { license: warpPlus });
      if (msg?.success && msg.obj) {
        setWarpData(JSON.parse(msg.obj));
        setWarpConfig(null);
        setWarpPlus('');
      } else {
        setLicenseError(msg?.msg || t('pages.xray.warp.licenseError'));
      }
    } finally {
      setLoading(false);
    }
  }

  async function delConfig() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/xray/warp/del');
      if (msg?.success) {
        setWarpData(null);
        setWarpConfig(null);
        setStagedOutbound(null);
        onRemoveOutbound('warp');
        onClose();
      }
    } finally {
      setLoading(false);
    }
  }

  function addOutbound() {
    if (!stagedOutbound) {
      messageApi.warning(t('pages.xray.warp.fetchFirst'));
      return;
    }
    onAddOutbound(stagedOutbound);
    onClose();
  }
  function resetOutbound() {
    if (!stagedOutbound) return;
    onResetOutbound({ index: warpOutboundIndex, outbound: stagedOutbound });
    onClose();
  }

  const hasWarp = !ObjectUtil.isEmpty(warpData);
  const hasConfig = !ObjectUtil.isEmpty(warpConfig);

  return (
    <>
      {messageContextHolder}
      <Modal open={open} title="Cloudflare WARP" footer={null} onCancel={onClose}>
      {!hasWarp ? (
        <Button type="primary" loading={loading} icon={<ApiOutlined />} onClick={register}>
          {t('pages.xray.warp.createAccount')}
        </Button>
      ) : (
        <>
          <table className="warp-data-table">
            <tbody>
              <tr className="row-odd">
                <td>{t('pages.xray.warp.accessToken')}</td>
                <td>{warpData?.access_token}</td>
              </tr>
              <tr>
                <td>{t('pages.xray.warp.deviceId')}</td>
                <td>{warpData?.device_id}</td>
              </tr>
              <tr className="row-odd">
                <td>{t('pages.xray.warp.licenseKey')}</td>
                <td>{warpData?.license_key}</td>
              </tr>
              <tr>
                <td>{t('pages.xray.warp.privateKey')}</td>
                <td>{warpData?.private_key}</td>
              </tr>
            </tbody>
          </table>

          <Button loading={loading} type="primary" danger className="mt-8" icon={<DeleteOutlined />} onClick={delConfig}>
            {t('pages.xray.warp.deleteAccount')}
          </Button>

          <Divider className="zero-margin">{t('pages.xray.warp.settings')}</Divider>

          <Collapse
            className="my-10"
            items={[
              {
                key: '1',
                label: t('pages.xray.warp.licenseKeyLabel'),
                children: (
                  <Form colon={false} labelCol={{ md: { span: 6 } }} wrapperCol={{ md: { span: 14 } }}>
                    <Form.Item label={t('pages.xray.warp.key')}>
                      <Input
                        value={warpPlus}
                        placeholder={t('pages.xray.warp.keyPlaceholder')}
                        onChange={(e) => {
                          setWarpPlus(e.target.value);
                          setLicenseError('');
                        }}
                      />
                      <div className="license-actions mt-8">
                        <Button
                          type="primary"
                          disabled={warpPlus.length < 26}
                          loading={loading}
                          onClick={updateLicense}
                        >
                          {t('update')}
                        </Button>
                        {licenseError && (
                          <Alert title={licenseError} type="error" showIcon className="license-error" />
                        )}
                      </div>
                    </Form.Item>
                  </Form>
                ),
              },
              {
                key: '2',
                label: t('pages.xray.warp.autoUpdateIp', 'Auto Update IP Address'),
                children: (
                  <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 12 } }}>
                    <Form.Item label={t('pages.xray.warp.intervalDays', 'Interval (Days)')} extra={t('pages.xray.warp.intervalDesc', '0 to disable. Changes IP address automatically.')}>
                      <Input
                        type="number"
                        min={0}
                        value={updateInterval}
                        onChange={(e) => setUpdateInterval(Number(e.target.value))}
                      />
                      <Button className="mt-8" type="primary" loading={loading} onClick={saveInterval}>
                        {t('save', 'Save')}
                      </Button>
                    </Form.Item>
                  </Form>
                ),
              },
            ]}
          />

          <Divider className="zero-margin">{t('pages.xray.warp.accountInfo')}</Divider>
          <div className="my-8">
            <Button loading={loading} type="primary" icon={<SyncOutlined />} onClick={getConfig}>
              {t('refresh')}
            </Button>
            <Button loading={loading} type="primary" className="ml-8" icon={<SyncOutlined />} onClick={changeIp}>
              {t('pages.xray.warp.changeIp', 'Change IP')}
            </Button>
          </div>

          {hasConfig && (
            <>
              <table className="warp-data-table">
                <tbody>
                  <tr className="row-odd">
                    <td>{t('pages.xray.warp.deviceName')}</td>
                    <td>{warpConfig?.name}</td>
                  </tr>
                  <tr>
                    <td>{t('pages.xray.warp.deviceModel')}</td>
                    <td>{warpConfig?.model}</td>
                  </tr>
                  <tr className="row-odd">
                    <td>{t('pages.xray.warp.deviceEnabled')}</td>
                    <td>{String(warpConfig?.enabled)}</td>
                  </tr>
                  {warpConfig?.account && (
                    <>
                      <tr>
                        <td>{t('pages.xray.warp.accountType')}</td>
                        <td>{warpConfig.account.account_type}</td>
                      </tr>
                      <tr className="row-odd">
                        <td>{t('pages.xray.warp.role')}</td>
                        <td>{warpConfig.account.role}</td>
                      </tr>
                      <tr>
                        <td>{t('pages.xray.warp.warpPlusData')}</td>
                        <td>{SizeFormatter.sizeFormat(warpConfig.account.premium_data)}</td>
                      </tr>
                      <tr className="row-odd">
                        <td>{t('pages.xray.warp.quota')}</td>
                        <td>{SizeFormatter.sizeFormat(warpConfig.account.quota)}</td>
                      </tr>
                      {warpConfig.account.usage != null && (
                        <tr>
                          <td>{t('pages.xray.warp.usage')}</td>
                          <td>{SizeFormatter.sizeFormat(warpConfig.account.usage)}</td>
                        </tr>
                      )}
                    </>
                  )}
                </tbody>
              </table>

              <Divider className="my-10">{t('pages.xray.outbound.outboundStatus')}</Divider>
              {warpOutboundIndex >= 0 ? (
                <>
                  <Tag color="green">{t('enabled')}</Tag>
                  <Button type="primary" danger loading={loading} className="ml-8" onClick={resetOutbound}>
                    {t('reset')}
                  </Button>
                </>
              ) : (
                <>
                  <Tag color="orange">{t('disabled')}</Tag>
                  <Button type="primary" loading={loading} className="ml-8" icon={<PlusOutlined />} onClick={addOutbound}>
                    {t('pages.xray.warp.addOutbound')}
                  </Button>
                </>
              )}
            </>
          )}
        </>
      )}
      </Modal>
    </>
  );
}
