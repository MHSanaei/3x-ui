import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Controller, FormProvider, useWatch } from 'react-hook-form';
import { Button, Divider, Empty, Form, Modal, Select, Space, Tag, message } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';

import { FormField, useZodForm } from '@/components/form/rhf';
import { externalLinkScopeLabel } from '@/lib/clients/external-link';
import { useClientOptions } from '@/api/queries/useClientOptions';
import { useGroupOptions } from '@/api/queries/useGroupOptions';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import {
  LinkAssignSchema,
  type LinkAssignValues,
  type LinkRecord,
  type LinkScope,
  type LinkTarget,
} from '@/schemas/api/link';

interface LinkTargetsModalProps {
  open: boolean;
  link: LinkRecord | null;
  targets: LinkTarget[];
  loading: boolean;
  assign: (
    payload: LinkAssignValues,
  ) => Promise<{ success?: boolean; msg?: string; obj?: unknown } | undefined>;
  unassign: (
    payload: LinkAssignValues,
  ) => Promise<{ success?: boolean; msg?: string; obj?: unknown } | undefined>;
  onOpenChange: (open: boolean) => void;
}

const SCOPES: LinkScope[] = ['client', 'group', 'inbound', 'global', 'new_clients'];

const EMPTY_ASSIGN: LinkAssignValues = {
  scope: 'client',
  emails: [],
  group: '',
  inboundId: 0,
};

function affectedCount(msg: { obj?: unknown } | undefined): number {
  const obj = msg?.obj as { affected?: number } | null | undefined;
  return typeof obj?.affected === 'number' ? obj.affected : 0;
}

export default function LinkTargetsModal({
  open,
  link,
  targets,
  loading,
  assign,
  unassign,
  onOpenChange,
}: LinkTargetsModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [busy, setBusy] = useState(false);
  const methods = useZodForm(LinkAssignSchema, { defaultValues: EMPTY_ASSIGN });
  const scope = useWatch({ control: methods.control, name: 'scope' }) ?? 'client';
  const { data: emails = [] } = useClientOptions(open);
  const { data: groups = [] } = useGroupOptions(open);
  const { data: inbounds = [] } = useInboundOptions();

  useEffect(() => {
    if (open) methods.reset(EMPTY_ASSIGN);
  }, [open, methods]);

  const inboundOptions = useMemo(
    () => inbounds.map((inbound) => ({ value: inbound.id, label: inbound.remark || inbound.tag })),
    [inbounds],
  );

  const onAssign = async (values: LinkAssignValues) => {
    if (!link || busy) return;
    setBusy(true);
    try {
      const res = await assign(values);
      if (res?.success) {
        messageApi.success(t('pages.links.affected', { count: affectedCount(res) }));
        methods.reset(EMPTY_ASSIGN);
      } else if (res?.msg) {
        messageApi.error(res.msg);
      }
    } finally {
      setBusy(false);
    }
  };

  const onUnassign = async (target: LinkTarget) => {
    if (!link || busy) return;
    setBusy(true);
    try {
      const res = await unassign({
        scope: target.targetType,
        emails: target.targetType === 'client' ? [target.name] : [],
        group: target.targetType === 'group' ? target.name : '',
        inboundId: target.targetType === 'inbound' ? target.targetId : 0,
      });
      if (res?.success) {
        messageApi.success(t('pages.links.unassigned'));
      } else if (res?.msg) {
        messageApi.error(res.msg);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      open={open}
      title={t('pages.links.targets')}
      onCancel={() => onOpenChange(false)}
      footer={null}
      destroyOnHidden
      width={640}
    >
      {messageContextHolder}
      {targets.length === 0 ? (
        <Empty description={t('pages.links.targetsEmpty')} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <Space size={[8, 8]} wrap>
          {targets.map((target) => (
            <Tag
              key={`${target.targetType}:${target.targetId}`}
              closable={!busy}
              closeIcon={<DeleteOutlined />}
              onClose={(e) => {
                e.preventDefault();
                void onUnassign(target);
              }}
            >
              {t(externalLinkScopeLabel(target.targetType))}: {target.name || target.targetId}
            </Tag>
          ))}
        </Space>
      )}
      <Divider />
      <FormProvider {...methods}>
        <Form colon={false} labelCol={{ sm: { span: 6 } }} wrapperCol={{ sm: { span: 18 } }}>
          <FormField name="scope" label={t('pages.links.scope')}>
            <Select
              options={SCOPES.map((value) => ({ value, label: t(externalLinkScopeLabel(value)) }))}
              loading={loading}
            />
          </FormField>
          {scope === 'client' && (
            <Controller
              control={methods.control}
              name="emails"
              render={({ field }) => (
                <Form.Item label={t('pages.links.scopeClient')}>
                  <Select
                    mode="multiple"
                    value={field.value}
                    onChange={field.onChange}
                    options={emails.map((email) => ({ value: email, label: email }))}
                    showSearch={{ optionFilterProp: 'label' }}
                    placeholder={t('pages.links.scopeClient')}
                  />
                </Form.Item>
              )}
            />
          )}
          {scope === 'group' && (
            <FormField name="group" label={t('pages.links.scopeGroup')}>
              <Select
                options={groups.map((name) => ({ value: name, label: name }))}
                showSearch={{ optionFilterProp: 'label' }}
                placeholder={t('pages.links.scopeGroup')}
              />
            </FormField>
          )}
          {scope === 'inbound' && (
            <FormField name="inboundId" label={t('pages.links.scopeInbound')}>
              <Select
                options={inboundOptions}
                showSearch={{ optionFilterProp: 'label' }}
                placeholder={t('pages.links.scopeInbound')}
              />
            </FormField>
          )}
          <Form.Item wrapperCol={{ sm: { offset: 6, span: 18 } }}>
            <Button type="primary" loading={busy} onClick={methods.handleSubmit(onAssign)}>
              {t('pages.links.assign')}
            </Button>
          </Form.Item>
        </Form>
      </FormProvider>
    </Modal>
  );
}
