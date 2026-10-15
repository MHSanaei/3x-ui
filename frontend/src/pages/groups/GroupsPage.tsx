import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Dropdown,
  Form,
  Input,
  Layout,
  Modal,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import type { MenuProps, TableColumnsType } from 'antd';
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  GlobalOutlined,
  LinkOutlined,
  MoreOutlined,
  PieChartOutlined,
  PlusOutlined,
  RetweetOutlined,
  TagsOutlined,
  TeamOutlined,
  UsergroupAddOutlined,
  UsergroupDeleteOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTitle } from '@/hooks/usePageTitle';
import { useClients } from '@/hooks/useClients';
import { HttpUtil, SizeFormatter } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';
import { LazyMount } from '@/components/utility';
import { keys } from '@/api/queryKeys';
import {
  ClientRecordSchema,
  GroupSummaryListSchema,
  type ClientRecord,
  type GroupSummary,
} from '@/schemas/client';
import { parseMsg } from '@/utils/zodValidate';
import { useHostsQuery } from '@/api/queries/useHostsQuery';

const ClientRecordListSchema = z
  .array(ClientRecordSchema)
  .nullable()
  .transform((v) => v ?? []);

const SubLinksModal = lazy(() => import('../clients/SubLinksModal'));
const ClientBulkAdjustModal = lazy(() => import('../clients/ClientBulkAdjustModal'));
const GroupAddClientsModal = lazy(() => import('./GroupAddClientsModal'));
const GroupRemoveClientsModal = lazy(() => import('./GroupRemoveClientsModal'));

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

async function fetchGroups(): Promise<GroupSummary[]> {
  const msg = await HttpUtil.get('/panel/api/clients/groups', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load groups');
  const validated = parseMsg(msg, GroupSummaryListSchema, 'clients/groups');
  return validated.obj ?? [];
}

async function fetchEmailsForGroup(name: string): Promise<string[]> {
  const msg = await HttpUtil.get<string[]>(
    `/panel/api/clients/groups/${encodeURIComponent(name)}/emails`,
    undefined,
    { silent: true },
  );
  if (!msg?.success || !Array.isArray(msg.obj)) return [];
  return msg.obj;
}

export default function GroupsPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);
  const queryClient = useQueryClient();

  const { subSettings, bulkAdjust, bulkAddToGroup, bulkRemoveFromGroup, bulkDelete } = useClients({
    list: false,
  });

  const groupsQuery = useQuery({
    queryKey: keys.clients.groups(),
    queryFn: fetchGroups,
  });
  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data]);
  const { hosts } = useHostsQuery();
  const loading = groupsQuery.isFetching;
  const fetched = groupsQuery.data !== undefined || groupsQuery.isError;
  const fetchError = groupsQuery.error ? (groupsQuery.error as Error).message : '';

  const invalidate = useCallback(() => {
    queryClient.invalidateQueries({ queryKey: keys.clients.root() });
  }, [queryClient]);

  const createMut = useMutation({
    mutationFn: (body: { name: string }) =>
      HttpUtil.post('/panel/api/clients/groups/create', body, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const renameMut = useMutation({
    mutationFn: (body: { oldName: string; newName: string }) =>
      HttpUtil.post('/panel/api/clients/groups/rename', body, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const deleteMut = useMutation({
    mutationFn: (body: { name: string }) =>
      HttpUtil.post('/panel/api/clients/groups/delete', body, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const groupResetMut = useMutation({
    mutationFn: (body: { name: string }) =>
      HttpUtil.post('/panel/api/clients/groups/resetTraffic', body, JSON_HEADERS),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');

  const [renameOpen, setRenameOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<GroupSummary | null>(null);
  const [renameValue, setRenameValue] = useState('');

  const [subLinksOpen, setSubLinksOpen] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [addClientsOpen, setAddClientsOpen] = useState(false);
  const [removeClientsOpen, setRemoveClientsOpen] = useState(false);
  const [groupEmails, setGroupEmails] = useState<string[]>([]);
  const [groupForAction, setGroupForAction] = useState<GroupSummary | null>(null);
  const [hostAssignmentOpen, setHostAssignmentOpen] = useState(false);
  const [hostAssignmentIds, setHostAssignmentIds] = useState<string[]>([]);
  const [groupHostAssignments, setGroupHostAssignments] = useState<Record<string, string[]>>({});
  const hostAssignmentMut = useMutation({
    mutationFn: ({ name, hostGroupIds }: { name: string; hostGroupIds: string[] }) =>
      HttpUtil.post(
        `/panel/api/clients/groups/${encodeURIComponent(name)}/hosts`,
        { hostGroupIds },
        JSON_HEADERS,
      ),
    onSuccess: (msg) => {
      if (msg?.success) setHostAssignmentOpen(false);
    },
  });

  useEffect(() => {
    let cancelled = false;
    void Promise.all(
      groups.map(async (group) => {
        const msg = await HttpUtil.get<string[]>(
          `/panel/api/clients/groups/${encodeURIComponent(group.name)}/hosts`,
          undefined,
          { silent: true },
        );
        return [group.name, msg?.success && Array.isArray(msg.obj) ? msg.obj : []] as const;
      }),
    ).then((entries) => {
      if (!cancelled) setGroupHostAssignments(Object.fromEntries(entries));
    });
    return () => {
      cancelled = true;
    };
  }, [groups]);

  const allClientsQuery = useQuery<ClientRecord[]>({
    queryKey: keys.clients.all(),
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/clients/list', undefined, { silent: true });
      if (!msg?.success) throw new Error(msg?.msg || 'Failed to load clients');
      const validated = parseMsg(msg, ClientRecordListSchema, 'clients/list');
      return validated.obj ?? [];
    },
    enabled: addClientsOpen || removeClientsOpen || subLinksOpen,
    staleTime: 30_000,
  });
  const allClients = allClientsQuery.data ?? [];

  const totalGroups = groups.length;
  const totalClients = useMemo(
    () => groups.reduce((acc, g) => acc + (g.clientCount || 0), 0),
    [groups],
  );
  const totalTraffic = useMemo(
    () => groups.reduce((acc, g) => acc + (g.trafficUsed || 0), 0),
    [groups],
  );
  const totalUpload = useMemo(() => groups.reduce((acc, g) => acc + (g.up || 0), 0), [groups]);
  const totalDownload = useMemo(() => groups.reduce((acc, g) => acc + (g.down || 0), 0), [groups]);

  function openCreate() {
    setCreateName('');
    setCreateOpen(true);
  }

  async function confirmCreate() {
    const name = createName.trim();
    if (!name) return;
    if (groups.some((g) => g.name.toLowerCase() === name.toLowerCase())) {
      messageApi.error(t('pages.groups.renameCollision', { name }));
      return;
    }
    const msg = await createMut.mutateAsync({ name });
    if (msg?.success) {
      messageApi.success(t('pages.groups.createSuccess', { name }));
      setCreateOpen(false);
    }
  }

  function openRename(g: GroupSummary) {
    setRenameTarget(g);
    setRenameValue(g.name);
    setRenameOpen(true);
  }

  async function confirmRename() {
    if (!renameTarget) return;
    const next = renameValue.trim();
    if (!next || next === renameTarget.name) {
      setRenameOpen(false);
      return;
    }
    if (
      groups.some(
        (g) => g.name.toLowerCase() === next.toLowerCase() && g.name !== renameTarget.name,
      )
    ) {
      messageApi.error(t('pages.groups.renameCollision', { name: next }));
      return;
    }
    const msg = await renameMut.mutateAsync({ oldName: renameTarget.name, newName: next });
    if (msg?.success) {
      const affected = (msg.obj as { affected?: number } | undefined)?.affected ?? 0;
      messageApi.success(t('pages.groups.renameSuccess', { count: affected }));
      setRenameOpen(false);
    }
  }

  function onDelete(g: GroupSummary) {
    modal.confirm({
      title: t('pages.groups.deleteConfirmTitle', { name: g.name }),
      content: t('pages.groups.deleteConfirmContent', { count: g.clientCount }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await deleteMut.mutateAsync({ name: g.name });
        if (msg?.success) {
          const affected = (msg.obj as { affected?: number } | undefined)?.affected ?? 0;
          messageApi.success(t('pages.groups.deleteSuccess', { count: affected }));
        }
      },
    });
  }

  async function openSubLinksFor(g: GroupSummary) {
    if (!g.clientCount) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    const emails = await fetchEmailsForGroup(g.name);
    if (emails.length === 0) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    setGroupForAction(g);
    setGroupEmails(emails);
    setSubLinksOpen(true);
  }

  async function openAdjustFor(g: GroupSummary) {
    if (!g.clientCount) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    const emails = await fetchEmailsForGroup(g.name);
    if (emails.length === 0) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    setGroupForAction(g);
    setGroupEmails(emails);
    setAdjustOpen(true);
  }

  function openAddClientsFor(g: GroupSummary) {
    setGroupForAction(g);
    setAddClientsOpen(true);
  }

  function openRemoveClientsFor(g: GroupSummary) {
    if (!g.clientCount) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    setGroupForAction(g);
    setRemoveClientsOpen(true);
  }

  async function openHostAssignmentsFor(g: GroupSummary) {
    const msg = await HttpUtil.get<string[]>(
      `/panel/api/clients/groups/${encodeURIComponent(g.name)}/hosts`,
      undefined,
      { silent: true },
    );
    setGroupForAction(g);
    const ids = msg?.success && Array.isArray(msg.obj) ? msg.obj : [];
    setHostAssignmentIds(ids);
    setGroupHostAssignments((current) => ({ ...current, [g.name]: ids }));
    setHostAssignmentOpen(true);
  }

  async function saveHostAssignments() {
    if (!groupForAction) return;
    await hostAssignmentMut.mutateAsync({
      name: groupForAction.name,
      hostGroupIds: hostAssignmentIds,
    });
    setGroupHostAssignments((current) => ({
      ...current,
      [groupForAction.name]: hostAssignmentIds,
    }));
  }

  function onDeleteClients(g: GroupSummary) {
    if (!g.clientCount) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    modal.confirm({
      title: t('pages.groups.deleteClientsConfirmTitle', { name: g.name }),
      content: t('pages.groups.deleteClientsConfirmContent', { count: g.clientCount }),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const emails = await fetchEmailsForGroup(g.name);
        if (emails.length === 0) return;
        const msg = await bulkDelete(emails);
        if (msg?.success) {
          const ok = msg.obj?.deleted ?? 0;
          const skipped = msg.obj?.skipped ?? [];
          const failed = skipped.length;
          if (failed === 0) {
            messageApi.success(t('pages.groups.deleteClientsSuccess', { count: ok }));
          } else {
            const firstError = skipped[0]?.reason ?? msg?.msg ?? '';
            messageApi.warning(
              firstError
                ? `${t('pages.groups.deleteClientsMixed', { ok, failed })} — ${firstError}`
                : t('pages.groups.deleteClientsMixed', { ok, failed }),
            );
          }
        }
      },
    });
  }

  function onResetTraffic(g: GroupSummary) {
    if (!g.clientCount) {
      messageApi.info(t('pages.groups.emptyForAction'));
      return;
    }
    modal.confirm({
      title: t('pages.groups.resetConfirmTitle', { name: g.name }),
      content: t('pages.groups.resetConfirmContent'),
      okText: t('reset'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await groupResetMut.mutateAsync({ name: g.name });
        if (msg?.success) {
          messageApi.success(t('pages.groups.resetSuccess', { name: g.name }));
        }
      },
    });
  }

  function rowActions(row: GroupSummary): MenuProps['items'] {
    return [
      {
        key: 'hostAssignments',
        icon: <GlobalOutlined />,
        label: t('pages.groups.assignHostGroups'),
        onClick: () => openHostAssignmentsFor(row),
      },
      {
        key: 'subLinks',
        icon: <LinkOutlined />,
        label: t('pages.clients.subLinksSelected', { count: row.clientCount || 0 }),
        disabled: !row.clientCount,
        onClick: () => openSubLinksFor(row),
      },
      {
        key: 'adjust',
        icon: <ClockCircleOutlined />,
        label: t('pages.clients.adjustSelected', { count: row.clientCount || 0 }),
        disabled: !row.clientCount,
        onClick: () => openAdjustFor(row),
      },
      {
        key: 'reset',
        icon: <RetweetOutlined />,
        label: t('pages.groups.resetTraffic'),
        disabled: !row.clientCount,
        onClick: () => onResetTraffic(row),
      },
      {
        key: 'addClients',
        icon: <UsergroupAddOutlined />,
        label: t('pages.groups.addToGroup'),
        onClick: () => openAddClientsFor(row),
      },
      {
        key: 'rename',
        icon: <EditOutlined />,
        label: t('pages.groups.rename'),
        onClick: () => openRename(row),
      },
      { type: 'divider' },
      {
        key: 'removeClients',
        icon: <UsergroupDeleteOutlined />,
        label: t('pages.groups.removeFromGroup'),
        danger: true,
        disabled: !row.clientCount,
        onClick: () => openRemoveClientsFor(row),
      },
      {
        key: 'deleteClients',
        icon: <DeleteOutlined />,
        label: t('pages.groups.deleteClients'),
        danger: true,
        disabled: !row.clientCount,
        onClick: () => onDeleteClients(row),
      },
      {
        key: 'delete',
        icon: <DeleteOutlined />,
        label: t('pages.groups.deleteGroupOnly'),
        danger: true,
        onClick: () => onDelete(row),
      },
    ];
  }

  function assignedHostLabels(groupName: string) {
    const assignedIds = groupHostAssignments[groupName] ?? [];
    const labels = new Map<string, string>();
    for (const host of hosts) {
      if (assignedIds.includes(host.groupId) && !labels.has(host.groupId)) {
        labels.set(host.groupId, host.remark || host.hosts?.[0] || host.groupId);
      }
    }
    return assignedIds.map((groupId) => ({
      groupId,
      label: labels.get(groupId) ?? groupId,
    }));
  }

  const columns: TableColumnsType<GroupSummary> = [
    {
      title: t('pages.clients.actions'),
      key: 'actions',
      width: 90,
      render: (_v, row) => (
        <Space size={4}>
          <Dropdown trigger={['click']} menu={{ items: rowActions(row) }}>
            <Button
              aria-label={t('more')}
              size="small"
              type="text"
              style={{ fontSize: 16 }}
              icon={<MoreOutlined />}
            />
          </Dropdown>
          <Tooltip title={t('pages.groups.rename')}>
            <Button
              aria-label={t('pages.groups.rename')}
              size="small"
              type="text"
              style={{ fontSize: 16 }}
              icon={<EditOutlined />}
              onClick={() => openRename(row)}
            />
          </Tooltip>
        </Space>
      ),
    },
    {
      title: t('pages.groups.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => (
        <Tag color="geekblue" style={{ margin: 0, fontSize: 13 }}>
          {name}
        </Tag>
      ),
    },
    {
      title: t('pages.clients.attachedHosts'),
      key: 'attachedHosts',
      render: (_value, row) => {
        const labels = assignedHostLabels(row.name);
        return labels.length > 0 ? (
          <Space size={[4, 4]} wrap>
            {labels.map(({ groupId, label }) => (
              <Tag key={groupId} color="cyan" style={{ margin: 0 }}>
                {label}
              </Tag>
            ))}
          </Space>
        ) : (
          <span style={{ color: 'var(--ant-color-text-tertiary)' }}>—</span>
        );
      },
    },
    {
      title: t('pages.groups.clientCount'),
      dataIndex: 'clientCount',
      key: 'clientCount',
      width: 180,
      render: (count: number) => <span>{count || 0}</span>,
    },
    {
      title: t('pages.groups.upload'),
      dataIndex: 'up',
      key: 'up',
      width: 140,
      render: (bytes: number) => <span>{SizeFormatter.sizeFormat(bytes || 0)}</span>,
    },
    {
      title: t('pages.groups.download'),
      dataIndex: 'down',
      key: 'down',
      width: 140,
      render: (bytes: number) => <span>{SizeFormatter.sizeFormat(bytes || 0)}</span>,
    },
    {
      title: t('pages.groups.trafficUsed'),
      dataIndex: 'trafficUsed',
      key: 'trafficUsed',
      width: 160,
      render: (bytes: number) => <span>{SizeFormatter.sizeFormat(bytes || 0)}</span>,
    },
  ];

  const pageClass = useMemo(() => {
    const classes = ['groups-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={
                    <Button type="primary" loading={loading} onClick={() => groupsQuery.refetch()}>
                      {t('refresh')}
                    </Button>
                  }
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.groups.totalGroups')}
                            value={String(totalGroups)}
                            prefix={<TagsOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.groups.totalGroupedClients')}
                            value={String(totalClients)}
                            prefix={<TeamOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.groups.totalUpDown')}
                            value={0}
                            formatter={() => (
                              <span>
                                <ArrowUpOutlined /> {SizeFormatter.sizeFormat(totalUpload)}
                                {' / '}
                                <ArrowDownOutlined /> {SizeFormatter.sizeFormat(totalDownload)}
                              </span>
                            )}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.groups.totalTraffic')}
                            value={SizeFormatter.sizeFormat(totalTraffic)}
                            prefix={<PieChartOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={
                        <div className="card-toolbar">
                          <Button
                            aria-label={t('pages.groups.addGroup')}
                            type="primary"
                            icon={<PlusOutlined />}
                            onClick={openCreate}
                          >
                            {!isMobile && t('pages.groups.addGroup')}
                          </Button>
                        </div>
                      }
                    >
                      <Table<GroupSummary>
                        dataSource={groups}
                        columns={columns}
                        rowKey="name"
                        size="small"
                        pagination={false}
                        loading={loading}
                        locale={{
                          emptyText: (
                            <div className="card-empty">
                              <TagsOutlined style={{ fontSize: 32, marginBottom: 8 }} />
                              <div>{t('noData')}</div>
                            </div>
                          ),
                        }}
                      />
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <Modal
          open={createOpen}
          title={t('pages.groups.addGroup')}
          okText={t('create')}
          cancelText={t('cancel')}
          confirmLoading={createMut.isPending}
          onCancel={() => setCreateOpen(false)}
          onOk={confirmCreate}
          destroyOnHidden
        >
          <Form layout="vertical">
            <Form.Item label={t('pages.groups.name')}>
              <Input
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
                onPressEnter={confirmCreate}
                placeholder={t('pages.clients.groupPlaceholder')}
                autoFocus
              />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          open={hostAssignmentOpen}
          title={
            groupForAction
              ? t('pages.groups.hostGroupsFor', { name: groupForAction.name })
              : t('pages.groups.hostGroups')
          }
          okText={t('save')}
          cancelText={t('cancel')}
          confirmLoading={hostAssignmentMut.isPending}
          onCancel={() => setHostAssignmentOpen(false)}
          onOk={saveHostAssignments}
          destroyOnHidden
        >
          <Select
            mode="multiple"
            allowClear
            style={{ width: '100%' }}
            value={hostAssignmentIds}
            onChange={setHostAssignmentIds}
            options={hosts.map((host) => ({
              value: host.groupId,
              label: host.remark || host.groupId,
            }))}
            placeholder={t('pages.groups.hostsEmptyMeansLegacy')}
          />
        </Modal>

        <Modal
          open={renameOpen}
          title={renameTarget ? t('pages.groups.renameTitle', { name: renameTarget.name }) : ''}
          okText={t('save')}
          cancelText={t('cancel')}
          confirmLoading={renameMut.isPending}
          onCancel={() => setRenameOpen(false)}
          onOk={confirmRename}
          destroyOnHidden
        >
          <Form layout="vertical">
            <Form.Item label={t('pages.groups.name')}>
              <Input
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onPressEnter={confirmRename}
                placeholder={t('pages.clients.groupPlaceholder')}
                autoFocus
              />
            </Form.Item>
          </Form>
        </Modal>

        <LazyMount when={subLinksOpen}>
          <SubLinksModal
            open={subLinksOpen}
            emails={groupEmails}
            clients={allClients}
            subSettings={subSettings}
            onOpenChange={setSubLinksOpen}
          />
        </LazyMount>

        <LazyMount when={adjustOpen}>
          <ClientBulkAdjustModal
            open={adjustOpen}
            count={groupEmails.length}
            onOpenChange={setAdjustOpen}
            onSubmit={async (addDays, addBytes, flow, limitHwid, adTag) => {
              const msg = await bulkAdjust(groupEmails, addDays, addBytes, flow, limitHwid, adTag);
              if (msg?.success) {
                const obj = msg.obj ?? { adjusted: 0 };
                messageApi.success(
                  t('pages.groups.adjustSuccess', {
                    count: obj.adjusted ?? 0,
                    name: groupForAction?.name ?? '',
                  }),
                );
                return obj;
              }
              return null;
            }}
          />
        </LazyMount>

        <LazyMount when={addClientsOpen}>
          <GroupAddClientsModal
            open={addClientsOpen}
            groupName={groupForAction?.name ?? null}
            candidates={allClients.filter((c) => c.group !== groupForAction?.name)}
            onClose={() => setAddClientsOpen(false)}
            onSubmit={async (emails) => {
              const msg = await bulkAddToGroup(emails, groupForAction?.name ?? '');
              if (msg?.success) {
                return (msg.obj as { affected?: number } | undefined) ?? { affected: 0 };
              }
              return null;
            }}
          />
        </LazyMount>

        <LazyMount when={removeClientsOpen}>
          <GroupRemoveClientsModal
            open={removeClientsOpen}
            groupName={groupForAction?.name ?? null}
            members={allClients.filter((c) => c.group === groupForAction?.name)}
            onClose={() => setRemoveClientsOpen(false)}
            onSubmit={async (emails) => {
              const msg = await bulkRemoveFromGroup(emails);
              if (msg?.success) {
                return (msg.obj as { affected?: number } | undefined) ?? { affected: 0 };
              }
              return null;
            }}
          />
        </LazyMount>
      </Layout>
    </ConfigProvider>
  );
}
