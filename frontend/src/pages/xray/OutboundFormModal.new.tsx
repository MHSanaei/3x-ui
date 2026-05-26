import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Select, Space, Tabs, message } from 'antd';

import JsonEditor from '@/components/JsonEditor';
import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';
import { OutboundFormBaseSchema, type OutboundFormValues } from '@/schemas/forms/outbound-form';
import { OutboundProtocols as Protocols } from '@/schemas/primitives';
import { antdRule } from '@/utils/zodForm';
import './OutboundFormModal.css';

// Pattern A rewrite of OutboundFormModal. Built as a sibling `.new.tsx`
// file so the build stays green section-by-section. The atomic swap at
// the end of the rewrite replaces the legacy file in one commit
// (per Core Decision 7 in the migration spec).

interface OutboundFormModalProps {
  open: boolean;
  outbound: Record<string, unknown> | null;
  existingTags: string[];
  onClose: () => void;
  onConfirm: (outbound: Record<string, unknown>) => void;
}

const PROTOCOL_OPTIONS = Object.values(Protocols).map((p) => ({ value: p, label: p }));

function buildAddModeValues(): OutboundFormValues {
  return rawOutboundToFormValues({});
}

export default function OutboundFormModalNew({
  open,
  outbound: outboundProp,
  existingTags,
  onClose,
  onConfirm,
}: OutboundFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<OutboundFormValues>();
  const [activeKey, setActiveKey] = useState('1');
  const [jsonText, setJsonText] = useState('');
  const [jsonDirty, setJsonDirty] = useState(false);

  const isEdit = outboundProp != null;
  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Outbounds')}`
    : `+ ${t('pages.xray.Outbounds')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  useEffect(() => {
    if (!open) return;
    const initial = outboundProp
      ? rawOutboundToFormValues(outboundProp)
      : buildAddModeValues();
    form.resetFields();
    form.setFieldsValue(initial);
    setActiveKey('1');
    setJsonText(JSON.stringify(formValuesToWirePayload(initial), null, 2));
    setJsonDirty(false);
  }, [open, outboundProp, form]);

  const tag = Form.useWatch('tag', form) ?? '';
  const protocol = (Form.useWatch('protocol', form) ?? 'vless') as string;

  const duplicateTag = useMemo(() => {
    const myTag = tag.trim();
    if (!myTag) return false;
    if (isEdit && (outboundProp?.tag as string | undefined) === myTag) return false;
    return (existingTags || []).includes(myTag);
  }, [tag, existingTags, isEdit, outboundProp]);

  // Bridge form ↔ JSON tab: when leaving the JSON tab back to Basic, push
  // any edits into form state. When entering JSON tab, snapshot current
  // form values so the user sees the live shape.
  function applyJsonToForm(): boolean {
    if (!jsonDirty) return true;
    const raw = jsonText.trim();
    if (!raw) return true;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(raw) as Record<string, unknown>;
    } catch (e) {
      messageApi.error(`JSON: ${(e as Error).message}`);
      return false;
    }
    const next = rawOutboundToFormValues(parsed);
    form.resetFields();
    form.setFieldsValue(next);
    setJsonDirty(false);
    return true;
  }

  function onTabChange(key: string) {
    if (document.activeElement instanceof HTMLElement) {
      document.activeElement.blur();
    }
    if (key === '2') {
      const values = form.getFieldsValue(true) as OutboundFormValues;
      setJsonText(JSON.stringify(formValuesToWirePayload(values), null, 2));
      setJsonDirty(false);
      setActiveKey(key);
      return;
    }
    if (key === '1' && activeKey === '2') {
      if (!applyJsonToForm()) return;
    }
    setActiveKey(key);
  }

  async function onOk() {
    if (activeKey === '2' && !applyJsonToForm()) return;
    let values: OutboundFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    if (duplicateTag) {
      messageApi.error('Tag already used by another outbound');
      return;
    }
    onConfirm(formValuesToWirePayload(values));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        okText={okText}
        cancelText={t('close')}
        mask={{ closable: false }}
        width={780}
        onOk={onOk}
        onCancel={onClose}
        destroyOnHidden
      >
        <Form
          form={form}
          colon={false}
          labelCol={{ md: { span: 8 } }}
          wrapperCol={{ md: { span: 14 } }}
        >
          <Tabs
            activeKey={activeKey}
            onChange={onTabChange}
            items={[
              {
                key: '1',
                label: t('pages.xray.basicTemplate'),
                children: (
                  <>
                    <Form.Item
                      label={t('protocol')}
                      name="protocol"
                      rules={[antdRule(OutboundFormBaseSchema.shape.tag, t)]}
                    >
                      <Select options={PROTOCOL_OPTIONS} />
                    </Form.Item>

                    <Form.Item
                      label="Tag"
                      name="tag"
                      validateStatus={duplicateTag ? 'warning' : undefined}
                      help={duplicateTag ? 'Tag already used by another outbound' : undefined}
                      rules={[
                        { required: true, message: 'Tag is required' },
                      ]}
                    >
                      <Input placeholder="unique-tag" />
                    </Form.Item>

                    <Form.Item label="Send through" name="sendThrough">
                      <Input placeholder="local IP" />
                    </Form.Item>

                    {/* Protocol-specific sub-forms come in subsequent commits. */}
                    <div style={{ marginTop: 12, opacity: 0.6, fontStyle: 'italic' }}>
                      Protocol-specific fields for {protocol} are still being
                      migrated. Use the JSON tab to edit settings until the
                      relevant section lands.
                    </div>
                  </>
                ),
              },
              {
                key: '2',
                label: 'JSON',
                children: (
                  <Space orientation="vertical" size={10} style={{ width: '100%', marginTop: 10 }}>
                    <JsonEditor
                      value={jsonText}
                      onChange={(next) => {
                        setJsonText(next);
                        setJsonDirty(true);
                      }}
                      minHeight="360px"
                      maxHeight="600px"
                    />
                  </Space>
                ),
              },
            ]}
          />
        </Form>
      </Modal>
    </>
  );
}
