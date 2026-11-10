import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  Row,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { CheckCircleOutlined, ClearOutlined, ThunderboltOutlined } from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { useAllSettings } from '@/api/queries/useAllSettings';
import { HttpUtil, Msg } from '@/utils';
import AppSidebar from '@/layouts/AppSidebar';
import { JsonEditor } from '@/components/form';
import { setMessageInstance } from '@/utils/messageBus';

// NodeRoutingPage is the master panel's control surface for node routing: it
// holds the shared routing policy (nodeRoutingRules) and pushes it to each
// connected node, which re-applies its Xray with the policy taking precedence
// over the node's own rules.
export default function NodeRoutingPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const {
    allSetting,
    updateSetting,
    savePayload,
    spinning: settingsSpinning,
    fetched: settingsFetched,
  } = useAllSettings();
  const { nodes, fetched, loading, refetch } = useNodesQuery();

  const [policyText, setPolicyText] = useState(allSetting.nodeRoutingRules || '');
  const [applying, setApplying] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setPolicyText(allSetting.nodeRoutingRules || ''), 0);
    return () => clearTimeout(t);
  }, [allSetting.nodeRoutingRules]);

  const policyValid = useMemo(() => {
    const text = policyText.trim();
    if (text === '') return true;
    try {
      const parsed = JSON.parse(text);
      return Array.isArray(parsed) && parsed.every((r) => r && typeof r === 'object');
    } catch {
      return false;
    }
  }, [policyText]);

  const policyRuleCount = useMemo(() => {
    const text = policyText.trim();
    if (text === '') return 0;
    try {
      const parsed = JSON.parse(text);
      return Array.isArray(parsed) ? parsed.length : 0;
    } catch {
      return 0;
    }
  }, [policyText]);

  const enabled = allSetting.nodeRoutingControl;

  const pageClass = `inbounds-page ${isDark ? 'is-dark' : ''} ${isUltra ? 'is-ultra' : ''}`.trim();

  async function savePolicy() {
    const result = await savePayload({ nodeRoutingRules: policyText.trim() });
    if (result?.success) {
      messageApi.success(t('pages.nodeRouting.policySaved'));
    }
  }

  async function applyToNode(node: NodeRecord, clear = false) {
    setApplying(true);
    try {
      const msg = (await HttpUtil.post('/panel/api/nodes/routing/apply', {
        nodeIds: [node.id],
        rules: clear ? '' : policyText.trim(),
        clear,
      })) as Msg<unknown>;
      if (msg?.success) {
        messageApi.success(
          clear
            ? t('pages.nodeRouting.cleared', { name: node.name })
            : t('pages.nodeRouting.applied', { name: node.name }),
        );
        refetch();
      } else {
        messageApi.error(
          t('pages.nodeRouting.applyFailed', { name: node.name, reason: msg?.msg || '' }),
        );
      }
    } finally {
      setApplying(false);
    }
  }

  async function applyToAll() {
    const targets = nodes.filter((n) => n.enable && n.status !== 'offline');
    if (targets.length === 0) {
      messageApi.warning(t('pages.nodeRouting.noNodes'));
      return;
    }
    setApplying(true);
    try {
      const msg = (await HttpUtil.post('/panel/api/nodes/routing/apply', {
        nodeIds: targets.map((n) => n.id),
        rules: policyText.trim(),
      })) as Msg<Array<{ nodeId: number; applied: boolean; error?: string }>>;
      if (msg?.success) {
        const results = Array.isArray(msg.obj) ? msg.obj : [];
        const ok = results.filter((r) => r.applied).length;
        const failed = results.length - ok;
        if (failed === 0) {
          messageApi.success(t('pages.nodeRouting.appliedAll', { count: ok }));
        } else {
          messageApi.warning(t('pages.nodeRouting.appliedMixed', { ok, failed }));
        }
        refetch();
      } else {
        messageApi.error(t('pages.nodeRouting.applyFailed', { name: '', reason: msg?.msg || '' }));
      }
    } finally {
      setApplying(false);
    }
  }

  const columns = [
    { title: t('pages.nodeRouting.node'), dataIndex: 'name', key: 'name' },
    {
      title: t('pages.nodeRouting.status'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string, n: NodeRecord) => (
        <Tag color={n.enable ? (s === 'online' ? 'green' : 'orange') : 'default'}>
          {s || 'unknown'}
        </Tag>
      ),
    },
    {
      title: t('pages.nodeRouting.egress'),
      dataIndex: 'outboundTag',
      key: 'outboundTag',
      render: (v: string) =>
        v ? <Tag color="blue">{v}</Tag> : <Tag>{t('pages.nodeRouting.direct')}</Tag>,
    },
    {
      title: t('pages.nodeRouting.policy'),
      key: 'policy',
      render: (_: unknown, n: NodeRecord) =>
        n.masterRoutingRules && n.masterRoutingRules.trim() !== '' ? (
          <Tag color="purple" icon={<CheckCircleOutlined aria-hidden />}>
            {t('pages.nodeRouting.policyActive')}
          </Tag>
        ) : (
          <Tag>{t('pages.nodeRouting.none')}</Tag>
        ),
    },
    {
      title: t('pages.nodeRouting.actions'),
      key: 'actions',
      render: (_: unknown, n: NodeRecord) => (
        <Space size={4}>
          <Button
            size="small"
            type="primary"
            disabled={!enabled || !n.enable || n.status === 'offline'}
            loading={applying}
            icon={<ThunderboltOutlined aria-hidden />}
            onClick={() => applyToNode(n)}
          >
            {t('pages.nodeRouting.apply')}
          </Button>
          <Button
            size="small"
            disabled={!enabled || !n.enable || n.status === 'offline'}
            loading={applying}
            icon={<ClearOutlined aria-hidden />}
            onClick={() => applyToNode(n, true)}
          >
            {t('pages.nodeRouting.clear')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin
              aria-live="off"
              spinning={!fetched || !settingsFetched}
              delay={200}
              description={t('loading')}
              size="large"
            >
              <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                <Col span={24}>
                  <Card hoverable size="small">
                    <Row className="header-row" gutter={[8, 12]}>
                      <Col xs={24} sm={14}>
                        <Typography.Title level={4} style={{ marginTop: 0 }}>
                          {t('pages.nodeRouting.title')}
                        </Typography.Title>
                      </Col>
                      <Col xs={24} sm={10} style={{ textAlign: 'right' }}>
                        <Space>
                          <Typography.Text type="secondary">
                            {t('pages.nodeRouting.control')}
                          </Typography.Text>
                          <Switch
                            aria-label={t('pages.nodeRouting.control')}
                            checked={enabled}
                            onChange={(v) => {
                              updateSetting({ nodeRoutingControl: v });
                              void savePayload({ nodeRoutingControl: v });
                            }}
                            loading={settingsSpinning}
                          />
                        </Space>
                      </Col>
                    </Row>
                  </Card>
                </Col>

                <Col span={24}>
                  <Card hoverable size="small" title={t('pages.nodeRouting.policyTitle')}>
                    {!enabled && (
                      <Alert
                        type="info"
                        showIcon
                        title={t('pages.nodeRouting.controlDisabled')}
                        style={{ marginBottom: 12 }}
                      />
                    )}
                    <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
                      {t('pages.nodeRouting.policyDesc')}
                    </Typography.Paragraph>
                    <JsonEditor
                      value={policyText}
                      onChange={setPolicyText}
                      minHeight="180px"
                      maxHeight="320px"
                      readOnly={!enabled}
                    />
                    {!policyValid && (
                      <Alert
                        type="error"
                        showIcon
                        title={t('pages.nodeRouting.policyInvalid')}
                        style={{ marginTop: 8 }}
                      />
                    )}
                    <Space style={{ marginTop: 12 }} wrap>
                      <Button
                        type="primary"
                        disabled={!enabled || !policyValid || policyText.trim() === ''}
                        loading={settingsSpinning}
                        onClick={savePolicy}
                      >
                        {t('pages.nodeRouting.savePolicy')}
                      </Button>
                      <Button
                        disabled={!enabled || policyText.trim() === ''}
                        onClick={() => setPolicyText('')}
                      >
                        {t('pages.nodeRouting.clearPolicy')}
                      </Button>
                      {policyRuleCount > 0 && policyValid && (
                        <Tag color="blue">
                          {t('pages.nodeRouting.ruleCount', { count: policyRuleCount })}
                        </Tag>
                      )}
                    </Space>
                  </Card>
                </Col>

                <Col span={24}>
                  <Card
                    hoverable
                    size="small"
                    title={t('pages.nodeRouting.nodesTitle')}
                    extra={
                      <Button
                        type="primary"
                        size="small"
                        disabled={!enabled || !policyValid || policyText.trim() === ''}
                        loading={applying}
                        onClick={applyToAll}
                      >
                        {t('pages.nodeRouting.applyAll')}
                      </Button>
                    }
                  >
                    <Table
                      rowKey="id"
                      size="small"
                      pagination={false}
                      dataSource={nodes}
                      columns={columns}
                      loading={loading}
                    />
                  </Card>
                </Col>
              </Row>
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
