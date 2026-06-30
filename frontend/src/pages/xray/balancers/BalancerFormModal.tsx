import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';

import { InputAddon } from '@/components/ui';
import {
  BalancerFormSchema,
  type BalancerFormValues,
} from '@/schemas/xray';
import {
  BalancerStrategyTypeSchema,
  type BalancerStrategySettings,
  type BalancerStrategyType,
} from '@/schemas/routing';

export type BalancerFormValue = BalancerFormValues;

interface BalancerFormModalProps {
  open: boolean;
  balancer: BalancerFormValue | null;
  outboundTags: string[];
  otherTags: string[];
  onClose: () => void;
  onConfirm: (value: BalancerFormValue) => void;
}

const STRATEGY_LABELS: Record<string, string> = {
  random: 'Random',
  roundRobin: 'Round robin',
  leastLoad: 'Least load',
  leastPing: 'Least ping',
};

const STRATEGIES = BalancerStrategyTypeSchema.options.map((value) => ({
  value,
  label: STRATEGY_LABELS[value] ?? value,
}));

interface FormState {
  tag: string;
  strategy: BalancerStrategyType;
  selector: string[];
  fallbackTag: string;
  settings?: BalancerStrategySettings;
}

function initialState(balancer: BalancerFormValue | null): FormState {
  if (!balancer) {
    return { tag: '', strategy: 'random', selector: [], fallbackTag: '' };
  }
  return {
    tag: balancer.tag ?? '',
    strategy: (balancer.strategy ?? 'random') as BalancerStrategyType,
    selector: [...(balancer.selector ?? [])],
    fallbackTag: balancer.fallbackTag ?? '',
    settings: balancer.settings,
  };
}

export default function BalancerFormModal({
  open,
  balancer,
  outboundTags,
  otherTags,
  onClose,
  onConfirm,
}: BalancerFormModalProps) {
  const { t } = useTranslation();
  const [state, setState] = useState<FormState>(() => initialState(balancer));
  const [touched, setTouched] = useState<Partial<Record<keyof FormState, boolean>>>({});
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const isEdit = balancer != null;

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setTouched((prev) => (prev[key] ? prev : { ...prev, [key]: true }));
    setState((prev) => ({ ...prev, [key]: value }));
  };

  const parsed = useMemo(
    () => BalancerFormSchema.safeParse(state),
    [state],
  );
  const duplicateTag = !!state.tag.trim() && otherTags.includes(state.tag.trim());
  const issues = useMemo(() => {
    const map: Record<string, string> = {};
    if (!parsed.success) {
      for (const issue of parsed.error.issues) {
        const key = String(issue.path[0] ?? '');
        if (!map[key]) map[key] = t(issue.message, { defaultValue: issue.message });
      }
    }
    return map;
  }, [parsed, t]);

  const showTagIssue = submitAttempted || !!touched.tag;
  const showSelectorIssue = submitAttempted || !!touched.selector;
  const tagError = showTagIssue ? issues.tag : '';
  const selectorError = showSelectorIssue ? issues.selector : '';
  const showDuplicate = showTagIssue && duplicateTag;

  function submit() {
    if (!parsed.success || duplicateTag) {
      setSubmitAttempted(true);
      return;
    }
    const values = { ...parsed.data };
    if (values.strategy !== 'leastLoad') delete values.settings;
    onConfirm(values);
  }

  const settings = state.settings;
  const updateSetting = <K extends keyof BalancerStrategySettings>(
    key: K,
    value: BalancerStrategySettings[K],
  ) => {
    setState((prev) => ({
      ...prev,
      settings: { ...(prev.settings ?? {}), [key]: value },
    }));
  };
  const updateBaselines = (next: string[]) => updateSetting('baselines', next);
  const updateCosts = (next: NonNullable<BalancerStrategySettings['costs']>) => updateSetting('costs', next);

  const baselines = settings?.baselines ?? [];
  const costs = settings?.costs ?? [];

  const fallbackOptions = useMemo(
    () => ['', ...outboundTags].map((tg) => ({ value: tg, label: tg || `(${t('none')})` })),
    [outboundTags, t],
  );

  const title = isEdit
    ? `${t('edit')} ${t('pages.xray.Balancers')}`
    : `+ ${t('pages.xray.Balancers')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('close')}
      mask={{ closable: false }}
      onOk={submit}
      onCancel={onClose}
    >
      <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
        <Form.Item
          label={t('pages.xray.balancer.tag')}
          required
          validateStatus={tagError ? 'error' : showDuplicate ? 'warning' : ''}
          help={tagError || (showDuplicate ? t('pages.xray.balancer.tagDuplicate') : '')}
          hasFeedback
        >
          <Input
            value={state.tag}
            onChange={(e) => update('tag', e.target.value)}
            placeholder={t('pages.xray.balancer.tagPlaceholder')}
          />
        </Form.Item>
        <Form.Item label={t('pages.xray.balancer.balancerStrategy')}>
          <Select
            value={state.strategy}
            onChange={(v) => update('strategy', v)}
            options={STRATEGIES}
          />
        </Form.Item>
        <Form.Item
          label={t('pages.xray.balancer.selector')}
          required
          validateStatus={selectorError ? 'error' : ''}
          help={selectorError || ''}
          hasFeedback
        >
          <Select
            mode="tags"
            value={state.selector}
            onChange={(v) => update('selector', v)}
            tokenSeparators={[',']}
            options={outboundTags.map((tg) => ({ value: tg, label: tg }))}
          />
        </Form.Item>
        <Form.Item label={t('pages.xray.balancer.fallback')}>
          <Select
            value={state.fallbackTag}
            onChange={(v) => update('fallbackTag', v ?? '')}
            allowClear
            options={fallbackOptions}
          />
        </Form.Item>

        {state.strategy === 'leastLoad' && (
          <>
            <Form.Item label={t('pages.xray.balancer.expected')}>
              <InputNumber
                value={settings?.expected}
                onChange={(v) => updateSetting('expected', typeof v === 'number' ? v : undefined)}
                min={0}
                placeholder={t('pages.xray.balancer.expectedPlaceholder')}
                style={{ width: '100%' }}
              />
            </Form.Item>
            <Form.Item label={t('pages.xray.balancer.maxRtt')}>
              <Input
                value={settings?.maxRTT ?? ''}
                onChange={(e) => updateSetting('maxRTT', e.target.value || undefined)}
                placeholder="e.g. 1s"
              />
            </Form.Item>
            <Form.Item label={t('pages.xray.balancer.tolerance')}>
              <InputNumber
                value={settings?.tolerance}
                onChange={(v) => updateSetting('tolerance', typeof v === 'number' ? v : undefined)}
                min={0}
                max={1}
                step={0.01}
                placeholder="0.01 = 1%"
                style={{ width: '100%' }}
              />
            </Form.Item>
            <Form.Item label={t('pages.xray.balancer.baselines')}>
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                aria-label={t('add')}
                onClick={() => updateBaselines([...baselines, ''])}
              />
              {baselines.map((b, idx) => (
                <Space.Compact key={idx} block style={{ marginTop: 4 }}>
                  <Input
                    value={b}
                    aria-label={t('pages.xray.balancer.baselines')}
                    placeholder="e.g. 1s"
                    onChange={(e) => updateBaselines(baselines.map((x, i) => (i === idx ? e.target.value : x)))}
                  />
                  <InputAddon ariaLabel={t('remove')} onClick={() => updateBaselines(baselines.filter((_, i) => i !== idx))}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
            <Form.Item label={t('pages.xray.balancer.costs')}>
              <Button
                size="small"
                type="primary"
                icon={<PlusOutlined />}
                aria-label={t('add')}
                onClick={() => updateCosts([...costs, { regexp: false, match: '', value: 1 }])}
              />
              {costs.map((c, idx) => (
                <Space.Compact key={idx} block style={{ marginTop: 4 }}>
                  <Switch
                    checked={c.regexp}
                    aria-label={t('pages.xray.balancer.costRegexp')}
                    checkedChildren="re"
                    unCheckedChildren="lit"
                    onChange={(v) => updateCosts(costs.map((x, i) => (i === idx ? { ...x, regexp: v } : x)))}
                  />
                  <Input
                    value={c.match}
                    aria-label={t('pages.xray.balancer.costMatch')}
                    placeholder="tag pattern"
                    onChange={(e) => updateCosts(costs.map((x, i) => (i === idx ? { ...x, match: e.target.value } : x)))}
                  />
                  <InputNumber
                    value={c.value}
                    aria-label={t('pages.xray.balancer.costValue')}
                    placeholder="weight"
                    style={{ width: 100 }}
                    onChange={(v) => updateCosts(costs.map((x, i) => (i === idx ? { ...x, value: typeof v === 'number' ? v : 0 } : x)))}
                  />
                  <InputAddon ariaLabel={t('remove')} onClick={() => updateCosts(costs.filter((_, i) => i !== idx))}>
                    <MinusOutlined />
                  </InputAddon>
                </Space.Compact>
              ))}
            </Form.Item>
          </>
        )}
      </Form>
    </Modal>
  );
}
