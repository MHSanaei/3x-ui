import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Col,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';

import type { SetTemplate, XraySettingsValue } from '@/hooks/useXraySetting';
import { HttpUtil } from '@/utils';

interface ExitPoolTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  inboundTags: string[];
}
interface ExitRow {
  slot: number;
  port: number;
  host: string;
  region: string;
  country: string;
  exitIp: string;
  status: string;
  latencyMs?: number;
  sourceCategory: ExitSourceCategory;
  sourceName?: string;
  ipType?: string;
  native?: boolean;
  stable: boolean;
}
interface InboundRow {
  id: number;
  tag: string;
  remark: string;
  port: number;
  protocol: string;
}
interface MemberRow {
  slot: number;
  region: string;
  country: string;
  exitIp: string;
  sourceCategory: ExitSourceCategory;
  email: string;
}

type ExitSourceCategory = 'xnvgatebox_stable' | 'new_residential' | 'new_native_datacenter';
type ExitSourceFilter = ExitSourceCategory | 'all';

const SOURCE_LABELS: Record<ExitSourceCategory, string> = {
  xnvgatebox_stable: 'xnvgatebox 稳定池',
  new_residential: '新增住宅池',
  new_native_datacenter: '新增原生机房池',
};

function sourceLabel(source: ExitSourceCategory | undefined) {
  return SOURCE_LABELS[source || 'xnvgatebox_stable'];
}
interface GroupRow {
  subId: string;
  name: string;
  subUrl: string;
  inboundId: number;
  inboundTag: string;
  members: MemberRow[];
  memberCount: number;
}
interface PoolState {
  installed: boolean;
  error?: string;
  exits: ExitRow[];
  inbounds: InboundRow[];
  groups: GroupRow[];
}

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

export default function ExitPoolTab(_: ExitPoolTabProps) {
  const [messageApi, contextHolder] = message.useMessage();
  const [state, setState] = useState<PoolState | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [inboundId, setInboundId] = useState<number>();
  const [slots, setSlots] = useState<number[]>([]);
  const [name, setName] = useState('Multi-region');
  const [subId, setSubId] = useState('');
  const [exitSort, setExitSort] = useState<'region' | 'latency'>('region');
  const [sourceFilter, setSourceFilter] = useState<ExitSourceFilter>('xnvgatebox_stable');
  const [showConsole, setShowConsole] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await HttpUtil.get<PoolState>('/panel/api/exit-pool', undefined, {
        silent: true,
      });
      if (!response.success || !response.obj) {
        messageApi.error(response.msg || 'Failed to read the exit pool');
        return;
      }
      setState(response.obj);
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void refresh();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refresh]);

  const sortedExits = useMemo(() => {
    const rows = [...(state?.exits || [])];
    rows.sort((left, right) => {
      const statusOrder = Number(right.status === 'up') - Number(left.status === 'up');
      if (statusOrder !== 0) return statusOrder;
      if (exitSort === 'latency') {
        const a = left.latencyMs && left.latencyMs > 0 ? left.latencyMs : Number.MAX_VALUE;
        const b = right.latencyMs && right.latencyMs > 0 ? right.latencyMs : Number.MAX_VALUE;
        return a - b || left.slot - right.slot;
      }
      const a = `${left.country || ''} ${left.region || ''}`.trim();
      const b = `${right.country || ''} ${right.region || ''}`.trim();
      return (
        a.localeCompare(b, 'zh-CN', { numeric: true, sensitivity: 'base' }) ||
        left.slot - right.slot
      );
    });
    return rows;
  }, [exitSort, state?.exits]);

  const readyExits = useMemo(
    () =>
      sortedExits.filter(
        (exit) =>
          exit.status === 'up' &&
          (sourceFilter === 'all' || (exit.sourceCategory || 'xnvgatebox_stable') === sourceFilter),
      ),
    [sortedExits, sourceFilter],
  );
  const visibleExits = useMemo(
    () =>
      sortedExits.filter(
        (exit) =>
          sourceFilter === 'all' || (exit.sourceCategory || 'xnvgatebox_stable') === sourceFilter,
      ),
    [sortedExits, sourceFilter],
  );
  const orderedSlots = useMemo(() => {
    const rank = new Map(readyExits.map((exit, index) => [exit.slot, index]));
    return [...slots].sort(
      (left, right) =>
        (rank.get(left) ?? Number.MAX_SAFE_INTEGER) -
          (rank.get(right) ?? Number.MAX_SAFE_INTEGER) || left - right,
    );
  }, [readyExits, slots]);
  const consoleUrl = `${(window.X_UI_BASE_PATH || '/').replace(/\/?$/, '/')}panel/api/exit-pool/console/`;

  async function syncGroup() {
    if (!inboundId || slots.length === 0 || !name.trim()) {
      messageApi.warning('请选择母节点、至少一个可用地区出口，并填写订阅组名称');
      return;
    }
    const selectedSources = new Set(
      (state?.exits || [])
        .filter((exit) => slots.includes(exit.slot))
        .map((exit) => exit.sourceCategory),
    );
    if (selectedSources.size !== 1) {
      messageApi.warning(
        '一个订阅组只能使用一种 IP 来源；请分别创建 xnvgatebox 稳定池、新增住宅池或新增原生机房池订阅。',
      );
      return;
    }
    setSaving(true);
    try {
      const response = await HttpUtil.post<GroupRow>(
        '/panel/api/exit-pool/sync',
        { inboundId, slots: orderedSlots, name: name.trim(), subId },
        { ...JSON_HEADERS, silent: true },
      );
      if (!response.success) {
        messageApi.error(response.msg || '创建订阅组失败');
        return;
      }
      messageApi.success(subId ? '订阅组已同步' : '多地区订阅组已创建');
      setSubId('');
      setSlots([]);
      await refresh();
    } finally {
      setSaving(false);
    }
  }

  function editGroup(group: GroupRow) {
    setSubId(group.subId);
    setName(group.name);
    setInboundId(group.inboundId);
    setSlots(group.members.map((member) => member.slot));
    const categories = new Set(group.members.map((member) => member.sourceCategory));
    setSourceFilter(categories.size === 1 ? [...categories][0] : 'all');
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  async function deleteGroup(group: GroupRow) {
    const response = await HttpUtil.post(
      `/panel/api/exit-pool/delete/${encodeURIComponent(group.subId)}`,
      undefined,
      { silent: true },
    );
    if (!response.success) {
      messageApi.error(response.msg || '删除订阅组失败');
      return;
    }
    messageApi.success('订阅组及其专用地区客户端已删除');
    if (subId === group.subId) {
      setSubId('');
      setSlots([]);
    }
    await refresh();
  }

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      {contextHolder}
      <Alert
        showIcon
        type="info"
        title="一个母节点，一个订阅，多地区出口"
        description="系统会在选中的 VLESS、VMess 或 Trojan 入站内为每个地区生成独立身份，并按身份路由到对应出口。所有身份共用同一个 subId，因此客户端导入一次订阅后即可选择不同地区；不会复制入站、不会增加公网端口。出口池仅承载 TCP，UDP 会在该出口失败关闭，不会回落到 VPS 直连。"
      />
      {!state?.installed && (
        <Alert
          showIcon
          type="warning"
          title="出口池尚未安装"
          description={
            <Typography.Text code copyable>
              x-ui exit-pool install
            </Typography.Text>
          }
        />
      )}
      {state?.error && (
        <Alert showIcon type="error" title="出口池暂不可用" description={state.error} />
      )}

      <Card
        size="small"
        title={subId ? `编辑订阅组：${name}` : '创建多地区订阅组'}
        extra={
          <Button loading={loading} onClick={() => void refresh()}>
            刷新出口
          </Button>
        }
      >
        <Space wrap style={{ marginBottom: 12 }}>
          <Typography.Text strong>出口排列方式：</Typography.Text>
          <Select
            aria-label="出口排列方式"
            value={exitSort}
            onChange={setExitSort}
            options={[
              { value: 'region', label: '按地区名称排序' },
              { value: 'latency', label: '按延迟从低到高排序' },
            ]}
            style={{ width: 210 }}
          />
          <Typography.Text strong>出口分类：</Typography.Text>
          <Select
            aria-label="出口分类"
            value={sourceFilter}
            onChange={(value) => {
              setSourceFilter(value);
              setSlots([]);
            }}
            options={[
              { value: 'xnvgatebox_stable', label: 'xnvgatebox 稳定池（推荐）' },
              { value: 'new_residential', label: '新增住宅池' },
              { value: 'new_native_datacenter', label: '新增原生机房池' },
              { value: 'all', label: '全部分类（仅查看，不可混合创建）' },
            ]}
            style={{ width: 230 }}
          />
          <Typography.Text type="secondary">此设置同时控制下方选择框和可用出口列表</Typography.Text>
        </Space>
        <Row gutter={[12, 12]}>
          <Col xs={24} lg={7}>
            <Select
              value={inboundId}
              placeholder="选择一个现有节点作为母节点"
              options={(state?.inbounds || []).map((inbound) => ({
                value: inbound.id,
                label: `${inbound.remark || inbound.tag} · ${inbound.protocol.toUpperCase()} :${inbound.port}`,
              }))}
              onChange={setInboundId}
              disabled={Boolean(subId)}
              style={{ width: '100%' }}
            />
          </Col>
          <Col xs={24} lg={6}>
            <Input
              value={name}
              maxLength={64}
              onChange={(event) => setName(event.target.value)}
              placeholder="订阅组名称"
            />
          </Col>
          <Col xs={24} lg={8}>
            <Select
              mode="multiple"
              value={orderedSlots}
              placeholder="选择多个国家或地区出口"
              options={readyExits.map((exit) => ({
                value: exit.slot,
                label: `${sourceLabel(exit.sourceCategory)} · ${exit.sourceName || '来源未命名'} · ${exit.country || exit.region || 'Unknown'} · ${exit.exitIp || exit.host} · ${Math.round(exit.latencyMs || 0)}ms`,
              }))}
              onChange={(next) => {
                const selected = new Set(next);
                setSlots(
                  readyExits.filter((exit) => selected.has(exit.slot)).map((exit) => exit.slot),
                );
              }}
              style={{ width: '100%' }}
            />
          </Col>
          <Col xs={24} lg={3}>
            <Button type="primary" block loading={saving} onClick={() => void syncGroup()}>
              {subId ? '同步' : '创建'}
            </Button>
          </Col>
        </Row>
        {subId && (
          <Button
            style={{ marginTop: 12 }}
            onClick={() => {
              setSubId('');
              setSlots([]);
            }}
          >
            取消编辑
          </Button>
        )}
      </Card>

      <Card
        size="small"
        title={`可用出口 · ${exitSort === 'region' ? '按地区名称' : '按延迟从低到高'}`}
        extra={
          <Select
            aria-label="可用出口排序"
            value={exitSort}
            onChange={setExitSort}
            options={[
              { value: 'region', label: '地区排序' },
              { value: 'latency', label: '延迟排序' },
            ]}
            style={{ width: 120 }}
          />
        }
      >
        <Table
          rowKey="slot"
          size="small"
          loading={loading}
          pagination={false}
          dataSource={visibleExits}
          columns={[
            {
              title: '分类',
              render: (_, row: ExitRow) => (
                <Tag
                  color={
                    row.sourceCategory === 'xnvgatebox_stable'
                      ? 'blue'
                      : row.sourceCategory === 'new_residential'
                        ? 'green'
                        : 'gold'
                  }
                >
                  {sourceLabel(row.sourceCategory)}
                </Tag>
              ),
            },
            { title: '地区', render: (_, row: ExitRow) => row.country || row.region || '-' },
            { title: '具体来源', render: (_, row: ExitRow) => row.sourceName || '-' },
            { title: '出口 IP', dataIndex: 'exitIp' },
            {
              title: '延迟',
              render: (_, row: ExitRow) =>
                row.latencyMs ? `${Math.round(row.latencyMs)} ms` : '-',
            },
            {
              title: '状态',
              render: (_, row: ExitRow) => (
                <Tag color={row.status === 'up' ? 'green' : 'orange'}>{row.status}</Tag>
              ),
            },
          ]}
        />
      </Card>

      <Card size="small" title="多地区订阅组">
        <Table
          rowKey="subId"
          size="small"
          pagination={false}
          dataSource={state?.groups || []}
          locale={{ emptyText: '尚未创建多地区订阅组' }}
          columns={[
            { title: '名称', dataIndex: 'name' },
            { title: '母节点', dataIndex: 'inboundTag' },
            {
              title: '地区节点',
              render: (_, row: GroupRow) => (
                <Space wrap>
                  {row.members.map((member) => (
                    <Tag key={member.slot}>
                      {sourceLabel(member.sourceCategory)} ·{' '}
                      {member.country || member.region || `Slot ${member.slot}`}
                    </Tag>
                  ))}
                </Space>
              ),
            },
            {
              title: '订阅 ID',
              render: (_, row: GroupRow) => (
                <Typography.Text code copyable>
                  {row.subId}
                </Typography.Text>
              ),
            },
            {
              title: '综合订阅链接',
              render: (_, row: GroupRow) =>
                row.subUrl ? (
                  <Typography.Text
                    copyable={{ text: row.subUrl }}
                    style={{ maxWidth: 260 }}
                    ellipsis
                  >
                    {row.subUrl}
                  </Typography.Text>
                ) : (
                  <Typography.Text type="secondary">请先在面板设置中启用订阅服务</Typography.Text>
                ),
            },
            {
              title: '操作',
              render: (_, row: GroupRow) => (
                <Space>
                  <Button size="small" onClick={() => editGroup(row)}>
                    编辑地区
                  </Button>
                  <Popconfirm
                    title="删除该订阅组及自动生成的地区客户端？"
                    onConfirm={() => void deleteGroup(row)}
                  >
                    <Button danger size="small">
                      删除
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          直接复制表格中的“综合订阅链接”导入客户端即可。内部地区身份仍默认隐藏，避免污染普通客户端列表；需要排查时可在客户端页面开启“显示出口池系统客户端”。订阅中的节点名称会显示订阅组、地区和出口编号，但都使用同一个服务器端口。
        </Typography.Paragraph>
      </Card>

      <Card
        size="small"
        title="xnvgatebox · 集成控制台"
        extra={
          <Button disabled={!state?.installed} onClick={() => setShowConsole((value) => !value)}>
            {showConsole ? '关闭控制台' : '打开控制台'}
          </Button>
        }
      >
        <Typography.Paragraph type="secondary">
          控制台通过3x-ui登录态访问，xnvgatebox 管理端口仅监听本机，不再提供公网独立入口。
        </Typography.Paragraph>
        {showConsole && (
          <iframe
            title="xnvgatebox 集成控制台"
            src={consoleUrl}
            style={{ width: '100%', height: '80vh', minHeight: 720, border: 0, borderRadius: 8 }}
          />
        )}
      </Card>
    </Space>
  );
}
