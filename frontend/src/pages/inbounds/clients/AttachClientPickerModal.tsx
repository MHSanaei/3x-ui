import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  AutoComplete,
  Button,
  Input,
  Modal,
  Pagination,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { selectTagRender } from '@/components/form';
import { formatInboundLabel } from '@/lib/inbounds/label';
import type { DBInbound } from '@/models/dbinbound';
import { HttpUtil, PromiseUtil } from '@/utils';
import { isInboundMultiUser } from '../list';

const PAGE_SIZE = 100;
const CANDIDATES = '/panel/api/clients/attachCandidates';

interface Candidate {
  id: number;
  email: string;
  comment: string;
  group: string;
  enable: boolean;
  alreadyAttached: boolean;
}

interface CandidatePage {
  items: Candidate[];
  total: number;
  eligible: number;
  groups: string[];
  scopeToken?: string;
  maxId?: number;
}

interface AttachResult {
  attached: number;
  skipped: number;
  errors: number;
}

interface FrozenSelection {
  token: string;
  eligible: number;
  maxId: number;
}

interface AttachClientPickerProps {
  inbound: DBInbound;
  sourceInbounds?: DBInbound[];
  onClose: () => void;
  onAttached?: () => void;
}

export default function AttachClientPickerModal({
  inbound,
  sourceInbounds,
  onClose,
  onAttached,
}: AttachClientPickerProps) {
  const { t } = useTranslation();
  const [messages, holder] = message.useMessage();
  const sourceId = sourceInbounds ? inbound.id : 0;
  const [targets, setTargets] = useState(sourceId ? [] : [inbound.id]);
  const targetScope = targets.join(',');
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [group, setGroup] = useState('');
  const [rows, setRows] = useState<CandidatePage>();
  const [loadError, setLoadError] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<'freeze' | 'submit'>();
  const action = useRef(false);
  const [frozen, setFrozen] = useState<FrozenSelection>();
  // Explicitly selected rows, or excluded rows when the scope is frozen.
  const [choices, setChoices] = useState(new Map<number, string>());
  const [exclusionPage, setExclusionPage] = useState(1);
  const selected = frozen ? frozen.eligible - choices.size : choices.size;

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(
      () => {
        HttpUtil.get(
          CANDIDATES,
          { sourceId, inboundIds: targetScope, page, search, group },
          { silent: true },
        )
          .then((response) => {
            if (cancelled) return;
            if (response?.success) {
              setRows(response.obj as CandidatePage);
              setLoadError('');
            } else {
              setRows(undefined);
              setLoadError(response?.msg || t('somethingWentWrong'));
            }
          })
          .finally(() => {
            if (!cancelled) setLoading(false);
          });
      },
      search || group ? 220 : 0,
    );
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [sourceId, targetScope, page, search, group, t]);

  function clear() {
    setFrozen(undefined);
    setChoices(new Map());
    setExclusionPage(1);
  }

  function changeTargets(ids: number[]) {
    clear();
    setTargets(ids);
    setLoading(true);
    setPage(1);
  }

  function selectRows(keys: React.Key[]) {
    const checked = new Set(keys);
    setChoices((current) => {
      const next = new Map(current);
      for (const row of rows?.items ?? []) {
        if (row.alreadyAttached || (frozen && row.id > frozen.maxId)) continue;
        if (frozen ? !checked.has(row.id) : checked.has(row.id)) next.set(row.id, row.email);
        else next.delete(row.id);
      }
      return next;
    });
  }

  async function selectAll() {
    if (action.current || !targets.length) return;
    action.current = true;
    setBusy('freeze');
    try {
      const response = await HttpUtil.get(
        CANDIDATES,
        { sourceId, inboundIds: targetScope, freeze: true },
        { silent: true },
      );
      const scope = response?.obj as CandidatePage | undefined;
      if (!response?.success || !scope?.scopeToken) {
        messages.error(response?.msg || t('somethingWentWrong'));
        return;
      }
      clear();
      setFrozen({ token: scope.scopeToken, eligible: scope.eligible, maxId: scope.maxId ?? 0 });
    } finally {
      action.current = false;
      setBusy(undefined);
    }
  }

  async function submit() {
    if (action.current || !targets.length || selected <= 0) return;
    action.current = true;
    setBusy('submit');
    try {
      const selection = frozen
        ? {
            mode: 'allMatching',
            sourceId,
            scopeToken: frozen.token,
            excludedClientIds: [...choices.keys()],
          }
        : { mode: 'explicit', sourceId, clientIds: [...choices.keys()] };
      const response = await HttpUtil.post<{ jobId: string }>(
        '/panel/api/clients/bulkAttach',
        { inboundIds: targets, selection },
        { headers: { 'Content-Type': 'application/json' }, silent: true },
      );
      if (!response?.success || !response.obj?.jobId) {
        messages.error(response?.msg || t('somethingWentWrong'));
        return;
      }
      const statusURL = '/panel/api/clients/bulkAttach/' + encodeURIComponent(response.obj.jobId);
      let result: AttachResult | undefined;
      let failedPolls = 0;
      while (!result) {
        const status = await HttpUtil.get<{
          running: boolean;
          result?: AttachResult;
          error?: string;
        }>(statusURL, undefined, { silent: true });
        if (!status?.success) {
          if (++failedPolls < 3) {
            await PromiseUtil.sleep(1000);
            continue;
          }
          messages.error(status?.msg || t('somethingWentWrong'));
          return;
        }
        failedPolls = 0;
        if (!status.obj?.running) {
          if (!status.obj?.result) {
            messages.error(status.obj?.error || t('somethingWentWrong'));
            return;
          }
          result = status.obj.result;
        } else {
          await PromiseUtil.sleep(1000);
        }
      }
      const { attached, skipped, errors } = result;
      if (errors > 0)
        messages.warning(
          t('pages.inbounds.attachClientsResultMixed', { attached, skipped, errors }),
        );
      else messages.success(t('pages.inbounds.attachClientsResult', { attached, skipped }));
      onAttached?.();
      onClose();
    } finally {
      action.current = false;
      setBusy(undefined);
    }
  }

  const columns: ColumnsType<Candidate> = [
    { title: t('pages.inbounds.email'), dataIndex: 'email', ellipsis: true },
    sourceId
      ? { title: t('comment'), dataIndex: 'comment', ellipsis: true }
      : { title: t('pages.clients.group'), dataIndex: 'group', ellipsis: true },
    {
      title: t('enable'),
      key: 'status',
      render: (_, row) =>
        row.alreadyAttached ? (
          <Tag>{t('pages.inbounds.attachExistingStatusAttached')}</Tag>
        ) : row.enable ? (
          <Tag color="success">{t('enable')}</Tag>
        ) : (
          <Tag>{t('pages.inbounds.attachClientsStatusDisabled')}</Tag>
        ),
    },
  ];
  const label = formatInboundLabel(inbound.tag, inbound.remark);
  const targetOptions = sourceInbounds
    ?.filter((item) => item.id !== sourceId && isInboundMultiUser(item))
    .map((item) => ({ value: item.id, label: formatInboundLabel(item.tag, item.remark) }));
  const lastExclusionPage = Math.max(1, Math.ceil(choices.size / 10));
  const currentExclusionPage = Math.min(exclusionPage, lastExclusionPage);
  const exclusions = frozen
    ? [...choices].slice((currentExclusionPage - 1) * 10, currentExclusionPage * 10)
    : [];

  return (
    <Modal
      open
      title={t(
        sourceId ? 'pages.inbounds.attachClientsTitle' : 'pages.inbounds.attachExistingTitle',
        { remark: label },
      )}
      width={720}
      onCancel={() => {
        if (!action.current) onClose();
      }}
      onOk={submit}
      closable={!busy}
      keyboard={!busy}
      okText={t('pages.inbounds.attachClients')}
      cancelText={t('cancel')}
      okButtonProps={{
        loading: busy === 'submit',
        disabled: !!busy || loading || !!loadError || !targets.length || selected <= 0,
      }}
      cancelButtonProps={{ disabled: !!busy }}
    >
      {holder}
      <Typography.Paragraph type="secondary">
        {t(sourceId ? 'pages.inbounds.attachClientsDesc' : 'pages.inbounds.attachExistingDesc', {
          count: rows?.eligible ?? 0,
        })}
      </Typography.Paragraph>
      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
        {targetOptions &&
          (targetOptions.length ? (
            <Select
              mode="multiple"
              maxCount={100}
              tagRender={selectTagRender}
              style={{ width: '100%' }}
              aria-label={t('pages.inbounds.attachClientsTargets')}
              placeholder={t('pages.inbounds.attachClientsTargets')}
              value={targets}
              onChange={changeTargets}
              options={targetOptions}
              disabled={!!busy}
              showSearch={{ optionFilterProp: 'label' }}
            />
          ) : (
            <Alert type="info" title={t('pages.inbounds.attachClientsNoTargets')} />
          ))}
        <Space wrap>
          <Input.Search
            allowClear
            aria-label={t('search')}
            value={search}
            placeholder={t('pages.inbounds.attachClientsSearchPlaceholder')}
            onChange={(e) => {
              setLoading(true);
              setPage(1);
              setSearch(e.target.value);
            }}
          />
          {!sourceId && (
            <AutoComplete
              allowClear
              aria-label={t('pages.clients.group')}
              value={group}
              placeholder={t('pages.clients.group')}
              style={{ minWidth: 150 }}
              options={rows?.groups.map((value) => ({ value })) ?? []}
              filterOption={(input, option) =>
                option?.value.toLowerCase().includes(input.toLowerCase()) ?? false
              }
              onChange={(value) => {
                setLoading(true);
                setPage(1);
                setGroup(value);
              }}
            />
          )}
        </Space>
        <Space wrap>
          <Button
            size="small"
            onClick={() => {
              selectRows(
                (rows?.items ?? [])
                  .filter((row) => !row.alreadyAttached && (!frozen || row.id <= frozen.maxId))
                  .map((row) => row.id),
              );
            }}
            disabled={
              !!busy ||
              loading ||
              !targets.length ||
              !rows?.items.some((row) => !row.alreadyAttached)
            }
          >
            {t('pages.inbounds.attachSelectPage')}
          </Button>
          <Button
            size="small"
            onClick={selectAll}
            disabled={!!busy || loading || !targets.length || !rows?.eligible}
            loading={busy === 'freeze'}
            aria-pressed={!!frozen}
          >
            {t('pages.inbounds.attachSelectAllEligible')}
          </Button>
          <Button size="small" onClick={clear} disabled={!!busy || (!choices.size && !frozen)}>
            {t('pages.inbounds.attachClearSelection')}
          </Button>
          <Typography.Text role="status" type="secondary">
            {t('pages.inbounds.attachClientsSelectedCount', {
              selected,
              total: frozen?.eligible ?? rows?.eligible ?? 0,
            })}
          </Typography.Text>
        </Space>
        {loadError && <Alert type="error" title={loadError} />}
        {!sourceId && !loading && !search && !group && rows?.total === 0 && (
          <Alert type="info" title={t('pages.inbounds.attachExistingNoClients')} />
        )}
        <section aria-label={t('pages.inbounds.attachClientsSelectLabel')}>
          <Table<Candidate>
            size="small"
            rowKey="id"
            columns={columns}
            loading={loading}
            dataSource={rows?.items ?? []}
            virtual
            scroll={{ x: 620, y: 280 }}
            pagination={{
              current: page,
              total: rows?.total ?? 0,
              pageSize: PAGE_SIZE,
              showSizeChanger: false,
              onChange: (next) => {
                if (page !== next) {
                  setLoading(true);
                  setPage(next);
                }
              },
            }}
            rowSelection={{
              selectedRowKeys: (rows?.items ?? [])
                .filter(
                  (row) =>
                    !row.alreadyAttached &&
                    (frozen ? row.id <= frozen.maxId && !choices.has(row.id) : choices.has(row.id)),
                )
                .map((row) => row.id),
              onChange: selectRows,
              getCheckboxProps: (row) => ({
                disabled:
                  !!busy ||
                  loading ||
                  !targets.length ||
                  row.alreadyAttached ||
                  (!!frozen && row.id > frozen.maxId),
                'aria-label': row.email,
              }),
            }}
          />
        </section>
        {exclusions.length > 0 && (
          <Space orientation="vertical">
            <Typography.Text>{t('pages.inbounds.attachExcludedClients')}</Typography.Text>
            <Space wrap>
              {exclusions.map(([id, email]) => (
                <Button
                  key={id}
                  size="small"
                  disabled={!!busy}
                  onClick={() =>
                    setChoices((current) => {
                      const next = new Map(current);
                      next.delete(id);
                      return next;
                    })
                  }
                >
                  {t('pages.inbounds.attachRestoreClient', { email })}
                </Button>
              ))}
              <Button size="small" disabled={!!busy} onClick={() => setChoices(new Map())}>
                {t('pages.inbounds.attachClearExclusions')}
              </Button>
            </Space>
            {lastExclusionPage > 1 && (
              <Pagination
                size="small"
                current={currentExclusionPage}
                pageSize={10}
                total={choices.size}
                showSizeChanger={false}
                onChange={setExclusionPage}
              />
            )}
          </Space>
        )}
      </Space>
    </Modal>
  );
}
