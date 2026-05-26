import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Modal, Typography, message } from 'antd';

import { HttpUtil, RandomUtil } from '@/utils';
import {
  rawInboundToFormValues,
  formValuesToWirePayload,
} from '@/lib/xray/inbound-form-adapter';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { InboundFormSchema, type InboundFormValues } from '@/schemas/forms/inbound-form';
import type { DBInbound } from '@/models/dbinbound';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

// Pattern A rewrite of InboundFormModal. Built as a sibling file so the
// build stays green while the rewrite progresses section by section. The
// old InboundFormModal.tsx continues to be the one InboundsPage renders
// until the atomic swap at the end of the rewrite (per Core Decision 7 in
// the architecture spec).
//
// Current state: skeleton only. The form holds the full InboundFormValues
// shape via setFieldsValue on open; validateFields + safeParse + adapter
// produce the wire payload on submit. Tabs are not yet wired — the modal
// body shows a WIP placeholder.

const { Text } = Typography;

interface InboundFormModalProps {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  mode: 'add' | 'edit';
  dbInbound: DBInbound | null;
  dbInbounds: DBInbound[];
  availableNodes?: NodeRecord[];
}

function buildAddModeValues(): InboundFormValues {
  const settings = createDefaultInboundSettings('vless') ?? undefined;
  return rawInboundToFormValues({
    protocol: 'vless',
    settings,
    streamSettings: { network: 'tcp', security: 'none' },
    sniffing: {},
    port: RandomUtil.randomInteger(10000, 60000),
    listen: '',
    tag: '',
    enable: true,
    trafficReset: 'never',
  });
}

export default function InboundFormModalNew({
  open,
  onClose,
  onSaved,
  mode,
  dbInbound,
}: InboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<InboundFormValues>();
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    const initial = mode === 'edit' && dbInbound
      ? rawInboundToFormValues(dbInbound)
      : buildAddModeValues();
    form.setFieldsValue(initial);
  }, [open, mode, dbInbound, form]);

  const submit = async () => {
    let values: InboundFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    const parsed = InboundFormSchema.safeParse(values);
    if (!parsed.success) {
      const issue = parsed.error.issues[0];
      messageApi.error(
        t(issue?.message ?? 'somethingWentWrong', {
          defaultValue: issue?.message ?? 'invalid',
        }),
      );
      return;
    }
    setSaving(true);
    try {
      const payload = formValuesToWirePayload(parsed.data);
      const url = mode === 'edit' && dbInbound
        ? `/panel/api/inbounds/update/${dbInbound.id}`
        : '/panel/api/inbounds/add';
      const msg = await HttpUtil.post(url, payload);
      if (msg?.success) {
        onSaved();
        onClose();
      }
    } finally {
      setSaving(false);
    }
  };

  const title = mode === 'edit'
    ? t('pages.inbounds.modifyInbound')
    : t('pages.inbounds.addInbound');

  const okText = mode === 'edit'
    ? t('pages.clients.submitEdit')
    : t('create');

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        okText={okText}
        cancelText={t('close')}
        confirmLoading={saving}
        mask={{ closable: false }}
        width={780}
        onOk={submit}
        onCancel={onClose}
        destroyOnHidden
      >
        <Form
          form={form}
          colon={false}
          labelCol={{ sm: { span: 8 } }}
          wrapperCol={{ sm: { span: 14 } }}
        >
          <Text type="secondary">
            WIP — Pattern A rewrite. Tabs are not yet wired into this skeleton.
          </Text>
        </Form>
      </Modal>
    </>
  );
}
