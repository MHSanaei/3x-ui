import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
} from 'antd';
import {
  ApiOutlined,
  ClearOutlined,
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import { HttpUtil } from '@/utils';
import type {
  PsiphonConfig,
  PsiphonCountry,
  PsiphonInstance,
  PsiphonProbeResult,
} from '@/schemas/protocols/outbound/psiphon';
import './PsiphonModal.css';

interface PsiphonModalProps {
  open: boolean;
  onClose: () => void;
  onRefreshXrayData?: () => void;
}

export function PsiphonModal({ open, onClose, onRefreshXrayData }: PsiphonModalProps) {
  const { t } = useTranslation();

  const [instances, setInstances] = useState<PsiphonInstance[]>([]);
  const [countries, setCountries] = useState<PsiphonCountry[]>([]);
  const [selectedCountries, setSelectedCountries] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [testingAll, setTestingAll] = useState(false);
  const [pingingIds, setPingingIds] = useState<Record<string, boolean>>({});

  const [configOpen, setConfigOpen] = useState(false);
  const [configJson, setConfigJson] = useState('');
  const [configSaving, setConfigSaving] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [instMsg, countryMsg] = await Promise.all([
        HttpUtil.get<PsiphonInstance[]>('/panel/api/psiphon/instances', undefined, {
          silent: true,
        }),
        HttpUtil.get<PsiphonCountry[]>('/panel/api/psiphon/countries', undefined, { silent: true }),
      ]);
      if (instMsg?.success && Array.isArray(instMsg.obj)) {
        setInstances(instMsg.obj);
      }
      if (countryMsg?.success && Array.isArray(countryMsg.obj)) {
        setCountries(countryMsg.obj);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchConfig = useCallback(async () => {
    const msg = await HttpUtil.get<PsiphonConfig>('/panel/api/psiphon/config', undefined, {
      silent: true,
    });
    if (msg?.success && msg.obj) {
      setConfigJson(msg.obj.configJson || '');
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    void (async () => {
      await fetchData();
      if (cancelled) return;
      await fetchConfig();
    })();
    return () => {
      cancelled = true;
    };
  }, [open, fetchData, fetchConfig]);

  const handleSaveConfig = async () => {
    setConfigSaving(true);
    try {
      const msg = await HttpUtil.post('/panel/api/psiphon/config', { configJson });
      if (msg?.success) {
        setConfigOpen(false);
      }
    } finally {
      setConfigSaving(false);
    }
  };

  const handleAddSelected = async () => {
    if (selectedCountries.length === 0) return;
    setLoading(true);
    try {
      const msg = await HttpUtil.post('/panel/api/psiphon/instances', {
        countries: selectedCountries,
      });
      if (msg?.success) {
        setSelectedCountries([]);
        await fetchData();
        onRefreshXrayData?.();
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSelectAllSupported = () => {
    const unadded = countries.filter((c) => !c.added).map((c) => c.code);
    setSelectedCountries(unadded);
  };

  const handleDelete = async (id: string) => {
    const msg = await HttpUtil.delete(`/panel/api/psiphon/instances/${id}`);
    if (msg?.success) {
      await fetchData();
      onRefreshXrayData?.();
    }
  };

  const handleToggle = async (id: string) => {
    const msg = await HttpUtil.post(`/panel/api/psiphon/instances/${id}/toggle`);
    if (msg?.success) {
      await fetchData();
      onRefreshXrayData?.();
    }
  };

  const handlePing = async (id: string) => {
    setPingingIds((prev) => ({ ...prev, [id]: true }));
    try {
      const msg = await HttpUtil.post<PsiphonProbeResult>(
        `/panel/api/psiphon/instances/${id}/ping`,
      );
      const probeObj = msg?.obj;
      if (msg?.success && probeObj && probeObj.ok) {
        setInstances((prev) =>
          prev.map((item) =>
            item.id === id
              ? {
                  ...item,
                  latencyMs: probeObj.latencyMs,
                  exitIp: probeObj.exitIp ?? item.exitIp,
                  status: 'connected',
                }
              : item,
          ),
        );
      }
    } finally {
      setPingingIds((prev) => ({ ...prev, [id]: false }));
    }
  };

  const handlePingAll = async () => {
    setTestingAll(true);
    try {
      const msg = await HttpUtil.post<Record<string, PsiphonProbeResult>>(
        '/panel/api/psiphon/ping-all',
      );
      if (msg?.success && msg.obj) {
        const results = msg.obj;
        setInstances((prev) =>
          prev.map((item) => {
            const r = results[item.id];
            if (r?.ok) {
              return {
                ...item,
                latencyMs: r.latencyMs,
                exitIp: r.exitIp ?? item.exitIp,
                status: 'connected',
              };
            }
            return item;
          }),
        );
      }
    } finally {
      setTestingAll(false);
    }
  };

  const countryOptions = useMemo(() => {
    return countries.map((c) => ({
      value: c.code,
      label: `${c.flag} ${c.name} (${c.code})`,
      disabled: c.added,
    }));
  }, [countries]);

  const columns: ColumnsType<PsiphonInstance> = [
    {
      title: t('pages.xray.psiphon.countries'),
      key: 'country',
      render: (_, record) => {
        const found = countries.find((c) => c.code === record.countryCode);
        const flag = found?.flag || '🌐';
        const name = found?.name || record.countryCode;
        return (
          <Space orientation="vertical" size={2}>
            <span>
              {flag} {name}
            </span>
            <Typography.Text type="secondary" className="psiphon-mono">
              psiphon-{record.countryCode}
            </Typography.Text>
          </Space>
        );
      },
    },
    {
      title: t('pages.xray.outbounds.status'),
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        let color = 'default';
        let label = t('pages.xray.psiphon.statusStopped');
        if (status === 'connected') {
          color = 'success';
          label = t('pages.xray.psiphon.statusConnected');
        } else if (status === 'starting') {
          color = 'processing';
          label = t('pages.xray.psiphon.statusStarting');
        } else if (status === 'error') {
          color = 'error';
          label = t('pages.xray.psiphon.statusError');
        }
        return <Tag color={color}>{label}</Tag>;
      },
    },
    {
      title: t('pages.inbounds.port'),
      dataIndex: 'socksPort',
      key: 'socksPort',
      render: (port: number) => <span className="psiphon-mono">127.0.0.1:{port}</span>,
    },
    {
      title: t('pages.xray.outbounds.exitIp'),
      dataIndex: 'exitIp',
      key: 'exitIp',
      render: (ip: string) => (ip ? <span className="psiphon-mono">{ip}</span> : '-'),
    },
    {
      title: t('pages.xray.outbounds.ping'),
      key: 'ping',
      render: (_, record) => {
        const ms = record.latencyMs;
        if (ms <= 0) {
          return <Tag>{t('pages.xray.psiphon.untested')}</Tag>;
        }
        let color = 'success';
        if (ms > 300) {
          color = 'error';
        } else if (ms >= 150) {
          color = 'warning';
        }
        return <Tag color={color}>{ms} ms</Tag>;
      },
    },
    {
      title: t('actions'),
      key: 'actions',
      render: (_, record) => {
        const isPinging = Boolean(pingingIds[record.id]);
        return (
          <Space>
            <Button
              size="small"
              icon={<ThunderboltOutlined />}
              loading={isPinging}
              onClick={() => handlePing(record.id)}
            >
              {t('pages.xray.psiphon.testPing')}
            </Button>
            <Switch
              size="small"
              checked={record.enabled}
              onChange={() => handleToggle(record.id)}
            />
            <Popconfirm
              title={t('pages.xray.psiphon.deleteConfirm')}
              onConfirm={() => handleDelete(record.id)}
              okText={t('confirm')}
              cancelText={t('cancel')}
            >
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <>
      <Modal
        title={
          <Space>
            <ApiOutlined />
            <span>{t('pages.xray.psiphon.title')}</span>
          </Space>
        }
        open={open}
        onCancel={onClose}
        footer={null}
        width={880}
        className="psiphon-modal"
      >
        <div className="psiphon-header-actions">
          <Space>
            <Button icon={<SettingOutlined />} onClick={() => setConfigOpen(true)}>
              {t('pages.xray.psiphon.configTitle')}
            </Button>
            <Button
              type="primary"
              icon={<ThunderboltOutlined />}
              loading={testingAll}
              onClick={handlePingAll}
            >
              {t('pages.xray.psiphon.testAll')}
            </Button>
          </Space>
          <Button icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
            {t('refresh')}
          </Button>
        </div>

        <div className="psiphon-bulk-section">
          <Typography.Title level={5} style={{ marginTop: 0 }}>
            {t('pages.xray.psiphon.addSelected')}
          </Typography.Title>
          <Select
            mode="multiple"
            style={{ width: '100%' }}
            placeholder={t('pages.xray.psiphon.selectCountries')}
            value={selectedCountries}
            onChange={setSelectedCountries}
            options={countryOptions}
            allowClear
          />
          {selectedCountries.length > 10 && (
            <Alert
              type="warning"
              showIcon
              message={t('pages.xray.psiphon.memoryWarning')}
              style={{ marginTop: 12 }}
            />
          )}
          <div className="psiphon-bulk-controls">
            <Button
              type="primary"
              icon={<PlusOutlined />}
              disabled={selectedCountries.length === 0}
              loading={loading}
              onClick={handleAddSelected}
            >
              {t('pages.xray.psiphon.addSelected')} ({selectedCountries.length})
            </Button>
            <Button onClick={handleSelectAllSupported}>{t('pages.xray.psiphon.selectAll')}</Button>
            <Button
              icon={<ClearOutlined />}
              disabled={selectedCountries.length === 0}
              onClick={() => setSelectedCountries([])}
            >
              {t('clear')}
            </Button>
          </div>
        </div>

        <Table
          rowKey="id"
          dataSource={instances}
          columns={columns}
          loading={loading}
          pagination={false}
          size="middle"
        />
      </Modal>

      <Modal
        title={t('pages.xray.psiphon.configTitle')}
        open={configOpen}
        onCancel={() => setConfigOpen(false)}
        onOk={handleSaveConfig}
        confirmLoading={configSaving}
        okText={t('pages.xray.psiphon.saveConfig')}
      >
        <Typography.Paragraph type="secondary">
          {t('pages.xray.psiphon.configDesc')}
        </Typography.Paragraph>
        <Input.TextArea
          rows={10}
          value={configJson}
          onChange={(e) => setConfigJson(e.target.value)}
          placeholder={`{\n  "PropagationChannelId": "...",\n  "SponsorId": "..."\n}`}
          style={{ fontFamily: 'monospace' }}
        />
      </Modal>
    </>
  );
}

export default PsiphonModal;
