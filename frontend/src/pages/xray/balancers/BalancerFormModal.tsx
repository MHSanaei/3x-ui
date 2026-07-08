import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Modal, Select, Space, Switch } from 'antd';
import { MinusOutlined, PlusOutlined } from '@ant-design/icons';
import { Controller, FormProvider, useForm, useWatch } from 'react-hook-form';
import type { Path } from 'react-hook-form';

import { InputAddon } from '@/components/ui';
import { FormField } from '@/components/form/rhf';
import {
  BalancerFormSchema,
  type BalancerFormValues,
} from '@/schemas/xray';
import {
  BalancerStrategyTypeSchema,
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

function initialState(balancer: BalancerFormValue | null): BalancerFormValues {
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
  const methods = useForm<BalancerFormValues>({ defaultValues: initialState(balancer) });
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const isEdit = balancer != null;

  useEffect(() => {
    if (open) {
      methods.reset(initialState(balancer));
      setSubmitAttempted(false);
    }
  }, [open, balancer, methods]);

  const strategy = useWatch({ control: methods.control, name: 'strategy' });
  const baselines = useWatch({ control: methods.control, name: 'settings.baselines' }) ?? [];
  const costs = useWatch({ control: methods.control, name: 'settings.costs' }) ?? [];

  function submit() {
    const values = methods.getValues();
    const parsed = BalancerFormSchema.safeParse(values);
    const trimmedTag = (values.tag ?? '').trim();
    const duplicateTag = !!trimmedTag && otherTags.includes(trimmedTag);
    methods.clearErrors();
    if (!parsed.success) {
      const seen = new Set<string>();
      for (const issue of parsed.error.issues) {
        const key = String(issue.path[0] ?? '');
        if (key && !seen.has(key)) {
          seen.add(key);
          methods.setError(key as Path<BalancerFormValues>, { message: issue.message });
        }
      }
    }
    if (!parsed.success || duplicateTag) {
      setSubmitAttempted(true);
      return;
    }
    const result: BalancerFormValues = { ...parsed.data };
    if (result.strategy !== 'leastLoad') delete result.settings;
    onConfirm(result);
  }

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
      <FormProvider {...methods}>
        <Form colon={false} labelCol={{ md: { span: 8 } }} wrapperCol={{ md: { span: 14 } }}>
          <Controller
            control={methods.control}
            name="tag"
            render={({ field, fieldState }) => {
              const trimmed = (field.value ?? '').trim();
              const duplicate = !!trimmed && otherTags.includes(trimmed);
              const errorMessage = fieldState.error?.message
                ? t(fieldState.error.message, { defaultValue: fieldState.error.message })
                : '';
              const showDuplicate = !errorMessage && (submitAttempted || fieldState.isTouched) && duplicate;
              return (
                <Form.Item
                  label={t('pages.xray.balancer.tag')}
                  required
                  validateStatus={errorMessage ? 'error' : showDuplicate ? 'warning' : ''}
                  help={errorMessage || (showDuplicate ? t('pages.xray.balancer.tagDuplicate') : '')}
                  hasFeedback
                >
                  <Input
                    value={field.value}
                    onChange={(e) => field.onChange(e.target.value)}
                    onBlur={field.onBlur}
                    ref={field.ref}
                    placeholder={t('pages.xray.balancer.tagPlaceholder')}
                  />
                </Form.Item>
              );
            }}
          />
          <FormField name="strategy" label={t('pages.xray.balancer.balancerStrategy')}>
            <Select options={STRATEGIES} />
          </FormField>
          <FormField name="selector" label={t('pages.xray.balancer.selector')} required>
            <Select
              mode="tags"
              tokenSeparators={[',']}
              options={outboundTags.map((tg) => ({ value: tg, label: tg }))}
            />
          </FormField>
          <FormField
            name="fallbackTag"
            label={t('pages.xray.balancer.fallback')}
            transform={{ output: (v) => v ?? '' }}
          >
            <Select allowClear options={fallbackOptions} />
          </FormField>

          {strategy === 'leastLoad' && (
            <>
              <FormField
                name={['settings', 'expected']}
                label={t('pages.xray.balancer.expected')}
                transform={{ output: (v) => (typeof v === 'number' ? v : undefined) }}
              >
                <InputNumber
                  min={0}
                  placeholder={t('pages.xray.balancer.expectedPlaceholder')}
                  style={{ width: '100%' }}
                />
              </FormField>
              <FormField
                name={['settings', 'maxRTT']}
                label={t('pages.xray.balancer.maxRtt')}
                transform={{ input: (v) => v ?? '', output: (v) => (typeof v === 'string' && v ? v : undefined) }}
              >
                <Input placeholder="e.g. 1s" />
              </FormField>
              <FormField
                name={['settings', 'tolerance']}
                label={t('pages.xray.balancer.tolerance')}
                transform={{ output: (v) => (typeof v === 'number' ? v : undefined) }}
              >
                <InputNumber min={0} max={1} step={0.01} placeholder="0.01 = 1%" style={{ width: '100%' }} />
              </FormField>
              <Form.Item label={t('pages.xray.balancer.baselines')}>
                <Button
                  size="small"
                  type="primary"
                  icon={<PlusOutlined />}
                  aria-label={t('add')}
                  onClick={() => methods.setValue('settings.baselines', [...baselines, ''])}
                />
                {baselines.map((b, idx) => (
                  <Space.Compact key={idx} block style={{ marginTop: 4 }}>
                    <Input
                      value={b}
                      aria-label={t('pages.xray.balancer.baselines')}
                      placeholder="e.g. 1s"
                      onChange={(e) => methods.setValue('settings.baselines', baselines.map((x, i) => (i === idx ? e.target.value : x)))}
                    />
                    <InputAddon ariaLabel={t('remove')} onClick={() => methods.setValue('settings.baselines', baselines.filter((_, i) => i !== idx))}>
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
                  onClick={() => methods.setValue('settings.costs', [...costs, { regexp: false, match: '', value: 1 }])}
                />
                {costs.map((c, idx) => (
                  <Space.Compact key={idx} block style={{ marginTop: 4 }}>
                    <Switch
                      checked={c.regexp}
                      aria-label={t('pages.xray.balancer.costRegexp')}
                      checkedChildren="re"
                      unCheckedChildren="lit"
                      onChange={(v) => methods.setValue('settings.costs', costs.map((x, i) => (i === idx ? { ...x, regexp: v } : x)))}
                    />
                    <Input
                      value={c.match}
                      aria-label={t('pages.xray.balancer.costMatch')}
                      placeholder="tag pattern"
                      onChange={(e) => methods.setValue('settings.costs', costs.map((x, i) => (i === idx ? { ...x, match: e.target.value } : x)))}
                    />
                    <InputNumber
                      value={c.value}
                      aria-label={t('pages.xray.balancer.costValue')}
                      placeholder="weight"
                      style={{ width: 100 }}
                      onChange={(v) => methods.setValue('settings.costs', costs.map((x, i) => (i === idx ? { ...x, value: typeof v === 'number' ? v : 0 } : x)))}
                    />
                    <InputAddon ariaLabel={t('remove')} onClick={() => methods.setValue('settings.costs', costs.filter((_, i) => i !== idx))}>
                      <MinusOutlined />
                    </InputAddon>
                  </Space.Compact>
                ))}
              </Form.Item>
            </>
          )}
        </Form>
      </FormProvider>
    </Modal>
  );
}
