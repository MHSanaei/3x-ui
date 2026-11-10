import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Input, Modal, Space, Spin, Typography, message } from 'antd';
import { CheckOutlined } from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import type { DBInbound } from '@/models/dbinbound';
import type { InboundTagChangeCheck } from '@/generated/types';

interface ChangeInboundIdModalProps {
  open: boolean;
  dbInbound: DBInbound | null;
  onClose: () => void;
  onSaved: () => void | Promise<void>;
}

type CheckResult = InboundTagChangeCheck;

// ChangeInboundIdModal is the "safe change ID" flow: it shows the current tag,
// takes a new one, runs the backend pre-flight check (uniqueness, node
// conflicts, how many routing rules a rename would rewrite) and only then
// lets the user commit through the proven rename path.
export default function ChangeInboundIdModal({
  open,
  dbInbound,
  onClose,
  onSaved,
}: ChangeInboundIdModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [newTag, setNewTag] = useState('');
  const [checking, setChecking] = useState(false);
  const [check, setCheck] = useState<CheckResult | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Reset when the dialog opens for a different inbound.
  useEffect(() => {
    if (!open || !dbInbound) return;
    const t = setTimeout(() => {
      setNewTag(dbInbound.tag);
      setCheck(null);
    }, 0);
    return () => clearTimeout(t);
  }, [open, dbInbound]);

  // Live pre-flight check, debounced so typing stays responsive.
  useEffect(() => {
    if (!open || !dbInbound) return;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const value = newTag.trim();
    if (value === '' || value === dbInbound.tag) {
      const t = setTimeout(() => {
        setCheck({
          unchanged: value === dbInbound.tag && value !== '',
          valid: value !== '',
          reason: '',
          routingRefs: 0,
          nodeTagConflict: false,
        });
      }, 0);
      return () => clearTimeout(t);
    }
    const t = setTimeout(() => setChecking(true), 0);
    debounceRef.current = setTimeout(async () => {
      const msg = await HttpUtil.post(
        '/panel/api/inbounds/checkTag',
        { inboundId: dbInbound.id, newTag: value },
        { silent: true },
      );
      setChecking(false);
      if (msg?.success && msg.obj) {
        setCheck(msg.obj as CheckResult);
      }
    }, 400);
    return () => {
      clearTimeout(t);
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [newTag, open, dbInbound]);

  const canApply =
    !!dbInbound && newTag.trim() !== '' && check?.valid === true && check.unchanged === false;

  async function apply() {
    if (!dbInbound || !canApply) return;
    setSubmitting(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/inbounds/changeTag/${dbInbound.id}`,
        { newTag: newTag.trim() },
        { silent: true },
      );
      if (msg?.success) {
        messageApi.success(t('pages.inbounds.toasts.changeIdSuccess'));
        await onSaved();
        onClose();
      } else {
        messageApi.error(t('pages.inbounds.toasts.changeIdFailed', { reason: msg?.msg || '' }));
      }
    } finally {
      setSubmitting(false);
    }
  }

  let alert: ReactNode = null;
  if (check && !check.unchanged && !check.valid) {
    alert = (
      <Alert
        type="error"
        showIcon
        message={t('pages.inbounds.changeIdInvalid', { reason: check.reason })}
      />
    );
  } else if (check && check.unchanged) {
    alert = <Alert type="info" showIcon message={t('pages.inbounds.changeIdUnchanged')} />;
  } else if (check && check.valid) {
    const impacts: string[] = [];
    if (check.routingRefs > 0) {
      impacts.push(t('pages.inbounds.changeIdRoutingRefs', { count: check.routingRefs }));
    }
    if (check.nodeTagConflict) {
      impacts.push(t('pages.inbounds.changeIdNodeConflict'));
    }
    alert = (
      <Alert
        type={check.nodeTagConflict ? 'warning' : 'success'}
        showIcon
        message={
          impacts.length > 0 ? (
            <Space direction="vertical" size={2}>
              {impacts.map((line, i) => (
                <Typography.Text key={i}>{line}</Typography.Text>
              ))}
            </Space>
          ) : (
            t('pages.inbounds.changeIdValid')
          )
        }
      />
    );
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.inbounds.changeIdTitle', { tag: dbInbound?.tag ?? '' })}
        okText={t('pages.inbounds.changeIdApply')}
        cancelText={t('cancel')}
        okButtonProps={{
          disabled: !canApply,
          loading: submitting,
          icon: <CheckOutlined aria-hidden />,
        }}
        onCancel={onClose}
        onOk={apply}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t('pages.inbounds.changeIdDesc')}
        </Typography.Paragraph>
        <Input
          aria-label={t('pages.inbounds.changeIdLabel')}
          value={newTag}
          onChange={(e) => setNewTag(e.target.value)}
          autoFocus
          onPressEnter={() => canApply && apply()}
          suffix={checking ? <Spin size="small" /> : undefined}
        />
        {alert}
      </Modal>
    </>
  );
}
