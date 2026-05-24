import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Select } from 'antd';

export interface BalancerFormValue {
  tag: string;
  strategy: string;
  selector: string[];
  fallbackTag: string;
}

interface BalancerFormModalProps {
  open: boolean;
  balancer: BalancerFormValue | null;
  outboundTags: string[];
  otherTags: string[];
  onClose: () => void;
  onConfirm: (value: BalancerFormValue) => void;
}

const STRATEGIES = [
  { value: 'random', label: 'Random' },
  { value: 'roundRobin', label: 'Round robin' },
  { value: 'leastLoad', label: 'Least load' },
  { value: 'leastPing', label: 'Least ping' },
];

export default function BalancerFormModal({
  open,
  balancer,
  outboundTags,
  otherTags,
  onClose,
  onConfirm,
}: BalancerFormModalProps) {
  const { t } = useTranslation();
  const [tag, setTag] = useState(() => balancer?.tag || '');
  const [strategy, setStrategy] = useState(() => balancer?.strategy || 'random');
  const [selector, setSelector] = useState<string[]>(() => [...(balancer?.selector || [])]);
  const [fallbackTag, setFallbackTag] = useState(() => balancer?.fallbackTag || '');

  const isEdit = balancer != null;

  useEffect(() => {
    if (!open) return;
    if (balancer) {
      setTag(balancer.tag || '');
      setStrategy(balancer.strategy || 'random');
      setSelector([...(balancer.selector || [])]);
      setFallbackTag(balancer.fallbackTag || '');
    } else {
      setTag('');
      setStrategy('random');
      setSelector([]);
      setFallbackTag('');
    }
  }, [open, balancer]);

  const tagEmpty = !tag.trim();
  const duplicateTag = !!tag && otherTags.includes(tag.trim());
  const emptySelector = selector.length === 0;
  const isValid = !tagEmpty && !duplicateTag && !emptySelector;

  const tagValidateStatus: 'error' | 'warning' | 'success' = tagEmpty
    ? 'error'
    : duplicateTag
      ? 'warning'
      : 'success';
  const tagHelp = tagEmpty
    ? 'Tag is required'
    : duplicateTag
      ? 'Tag already used by another balancer'
      : '';

  const selectorValidateStatus: 'error' | 'success' = emptySelector ? 'error' : 'success';
  const selectorHelp = emptySelector ? 'Pick at least one outbound' : '';

  function submit() {
    if (!isValid) return;
    onConfirm({ tag, strategy, selector, fallbackTag });
  }

  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Balancers')}`
    : `+ ${t('pages.xray.Balancers')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  const fallbackOptions = useMemo(
    () => ['', ...outboundTags].map((tg) => ({ value: tg, label: tg || `(${t('none')})` })),
    [outboundTags, t],
  );

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('close')}
      okButtonProps={{ disabled: !isValid }}
      mask={{ closable: false }}
      destroyOnHidden
      onOk={submit}
      onCancel={onClose}
    >
      <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
        <Form.Item label="Tag" validateStatus={tagValidateStatus} help={tagHelp} hasFeedback>
          <Input value={tag} onChange={(e) => setTag(e.target.value)} placeholder="unique balancer tag" />
        </Form.Item>
        <Form.Item label="Strategy">
          <Select value={strategy} onChange={setStrategy} options={STRATEGIES} />
        </Form.Item>
        <Form.Item
          label="Selector"
          validateStatus={selectorValidateStatus}
          help={selectorHelp}
          hasFeedback
        >
          <Select
            mode="tags"
            value={selector}
            onChange={setSelector}
            tokenSeparators={[',']}
            options={outboundTags.map((tg) => ({ value: tg, label: tg }))}
          />
        </Form.Item>
        <Form.Item label="Fallback">
          <Select value={fallbackTag} onChange={setFallbackTag} allowClear options={fallbackOptions} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
