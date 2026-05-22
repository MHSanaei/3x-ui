import { useCallback, useEffect, useMemo, useState } from 'react';
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
  const [messageApi, messageContextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [warpData, setWarpData] = useState<WarpData | null>(null);
  const [warpConfig, setWarpConfig] = useState<WarpConfig | null>(null);
  const [warpPlus, setWarpPlus] = useState('');
  const [licenseError, setLicenseError] = useState('');
  const [stagedOutbound, setStagedOutbound] = useState<Record<string, unknown> | null>(null);

  const warpOutboundIndex = useMemo(() => {
    const list = templateSettings?.outbounds;
    if (!list) return -1;
    return list.findIndex((o) => o?.tag === 'warp');
  }, [templateSettings?.outbounds]);

  const collectConfig = useCallback((data: WarpData | null, config: WarpConfig | null) => {
    const cfg = config?.config;
    if (!cfg?.peers?.length) return;
    const peer = cfg.peers[0];
    setStagedOutbound({
      tag: 'warp',
      protocol: 'wireguard',
      settings: {
        mtu: 1420,
        secretKey: data?.private_key,
        address: addressesFor(cfg.interface?.addresses || {}),
        reserved: reservedFor(data?.client_id),
        domainStrategy: 'ForceIP',
        peers: [{ publicKey: peer.public_key, endpoint: peer.endpoint?.host }],
        noKernelTun: false,
      },
    });
  }, []);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/warp/data');
      if (msg?.success) {
        const raw = msg.obj;
        setWarpData(raw && raw.length > 0 ? JSON.parse(raw) : null);
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
      const msg = await HttpUtil.post('/panel/xray/warp/reg', keys);
      if (msg?.success) {
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
      const msg = await HttpUtil.post('/panel/xray/warp/config');
      if (msg?.success) {
        const parsed = JSON.parse(msg.obj);
        setWarpConfig(parsed);
        collectConfig(warpData, parsed);
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
      const msg = await HttpUtil.post('/panel/xray/warp/license', { license: warpPlus });
      if (msg?.success) {
        setWarpData(JSON.parse(msg.obj));
        setWarpConfig(null);
        setWarpPlus('');
      } else {
        setLicenseError(msg?.msg || 'Failed to set WARP license.');
      }
    } finally {
      setLoading(false);
    }
  }

  async function delConfig() {
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/xray/warp/del');
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
      messageApi.warning('Fetch the WARP config first.');
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
          Create WARP account
        </Button>
      ) : (
        <>
          <table className="warp-data-table">
            <tbody>
              <tr className="row-odd">
                <td>Access token</td>
                <td>{warpData?.access_token}</td>
              </tr>
              <tr>
                <td>Device ID</td>
                <td>{warpData?.device_id}</td>
              </tr>
              <tr className="row-odd">
                <td>License key</td>
                <td>{warpData?.license_key}</td>
              </tr>
              <tr>
                <td>Private key</td>
                <td>{warpData?.private_key}</td>
              </tr>
            </tbody>
          </table>

          <Button loading={loading} type="primary" danger className="mt-8" icon={<DeleteOutlined />} onClick={delConfig}>
            Delete account
          </Button>

          <Divider className="zero-margin">Settings</Divider>

          <Collapse
            className="my-10"
            items={[
              {
                key: '1',
                label: 'WARP / WARP+ license key',
                children: (
                  <Form colon={false} labelCol={{ md: { span: 6 } }} wrapperCol={{ md: { span: 14 } }}>
                    <Form.Item label="Key">
                      <Input
                        value={warpPlus}
                        placeholder="26-char WARP+ key"
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
                          Update
                        </Button>
                        {licenseError && (
                          <Alert title={licenseError} type="error" showIcon className="license-error" />
                        )}
                      </div>
                    </Form.Item>
                  </Form>
                ),
              },
            ]}
          />

          <Divider className="zero-margin">Account info</Divider>
          <Button className="my-8" loading={loading} type="primary" icon={<SyncOutlined />} onClick={getConfig}>
            Refresh
          </Button>

          {hasConfig && (
            <>
              <table className="warp-data-table">
                <tbody>
                  <tr className="row-odd">
                    <td>Device name</td>
                    <td>{warpConfig?.name}</td>
                  </tr>
                  <tr>
                    <td>Device model</td>
                    <td>{warpConfig?.model}</td>
                  </tr>
                  <tr className="row-odd">
                    <td>Device enabled</td>
                    <td>{String(warpConfig?.enabled)}</td>
                  </tr>
                  {warpConfig?.account && (
                    <>
                      <tr>
                        <td>Account type</td>
                        <td>{warpConfig.account.account_type}</td>
                      </tr>
                      <tr className="row-odd">
                        <td>Role</td>
                        <td>{warpConfig.account.role}</td>
                      </tr>
                      <tr>
                        <td>WARP+ data</td>
                        <td>{SizeFormatter.sizeFormat(warpConfig.account.premium_data)}</td>
                      </tr>
                      <tr className="row-odd">
                        <td>Quota</td>
                        <td>{SizeFormatter.sizeFormat(warpConfig.account.quota)}</td>
                      </tr>
                      {warpConfig.account.usage != null && (
                        <tr>
                          <td>Usage</td>
                          <td>{SizeFormatter.sizeFormat(warpConfig.account.usage)}</td>
                        </tr>
                      )}
                    </>
                  )}
                </tbody>
              </table>

              <Divider className="my-10">Outbound status</Divider>
              {warpOutboundIndex >= 0 ? (
                <>
                  <Tag color="green">Enabled</Tag>
                  <Button type="primary" danger loading={loading} className="ml-8" onClick={resetOutbound}>
                    Reset
                  </Button>
                </>
              ) : (
                <>
                  <Tag color="orange">Disabled</Tag>
                  <Button type="primary" loading={loading} className="ml-8" icon={<PlusOutlined />} onClick={addOutbound}>
                    Add outbound
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
