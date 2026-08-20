import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, InputNumber, Modal, Select, Switch, message } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { FormField, rhfZodValidate } from '@/components/form/rhf';
import SelectAllClearButtons from '@/components/form/SelectAllClearButtons';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import { formatInboundLabel } from '@/lib/inbounds/label';
import {
  SubBalancerFormSchema,
  SubBalancerStrategySchema,
  type SubBalancer,
  type SubBalancerFormValues,
  type SubBalancerStrategy,
} from '@/schemas/subBalancer';

// The JSON subscription only builds proxy outbounds for these protocols;
// mtproto has no proxy-outbound case, so it is excluded from balancer members.
const MULTI_CLIENT_PROTOCOLS = new Set([
  'shadowsocks',
  'vless',
  'vmess',
  'trojan',
  'hysteria',
  'wireguard',
]);

const STRATEGY_LABEL_KEYS: Record<SubBalancerStrategy, string> = {
  leastLoad: 'pages.settings.subBalancers.strategyLeastLoad',
  leastPing: 'pages.settings.subBalancers.strategyLeastPing',
  random: 'pages.settings.subBalancers.strategyRandom',
  roundRobin: 'pages.settings.subBalancers.strategyRoundRobin',
};

function initialState(balancer: SubBalancer | null): SubBalancerFormValues {
  return {
    remark: balancer?.remark ?? '',
    strategy: balancer?.strategy ?? 'random',
    inboundIds: [...(balancer?.inboundIds ?? [])],
    sortOrder: balancer?.sortOrder ?? 1,
    enabled: balancer?.enabled ?? true,
  };
}

interface SubBalancerFormModalProps {
  open: boolean;
  balancer: SubBalancer | null;
  onClose: () => void;
  onConfirm: (values: SubBalancerFormValues) => void;
}

export default function SubBalancerFormModal({
  open,
  balancer,
  onClose,
  onConfirm,
}: SubBalancerFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<SubBalancerFormValues>({ defaultValues: initialState(balancer) });
  const isEdit = balancer != null;

  useEffect(() => {
    if (open) methods.reset(initialState(balancer));
  }, [open, balancer, methods]);

  const inboundIds = useWatch({ control: methods.control, name: 'inboundIds' }) ?? [];

  const { data: inboundOptionsRaw } = useInboundOptions();
  const inboundOptions = useMemo(
    () =>
      (inboundOptionsRaw ?? [])
        .filter((ib) => MULTI_CLIENT_PROTOCOLS.has(ib.protocol || ''))
        .map((ib) => ({
          label: formatInboundLabel(ib.tag, ib.remark),
          value: ib.id,
          title: formatInboundLabel(ib.tag, ib.remark),
        })),
    [inboundOptionsRaw],
  );

  function onFinish(values: SubBalancerFormValues) {
    const parsed = SubBalancerFormSchema.safeParse(values);
    if (!parsed.success) {
      messageApi.error(
        t(parsed.error.issues[0]?.message ?? 'pages.settings.subBalancers.errRemarkRequired'),
      );
      return;
    }
    onConfirm(parsed.data);
  }

  const strategies = SubBalancerStrategySchema.options.map((value) => ({
    value,
    label: t(STRATEGY_LABEL_KEYS[value]),
  }));

  return (
    <Modal
      open={open}
      title={
        isEdit
          ? `${t('edit')} ${t('pages.settings.subBalancers.title')}`
          : `+ ${t('pages.settings.subBalancers.add')}`
      }
      okText={isEdit ? t('pages.clients.submitEdit') : t('create')}
      cancelText={t('close')}
      mask={{ closable: false }}
      width="640px"
      onOk={methods.handleSubmit(onFinish)}
      onCancel={onClose}
    >
      {messageContextHolder}
      <FormProvider {...methods}>
        <Form layout="vertical">
          <FormField
            label={t('pages.settings.subBalancers.remark')}
            name="remark"
            required
            rules={{ validate: rhfZodValidate(SubBalancerFormSchema.shape.remark) }}
          >
            <Input placeholder={t('pages.settings.subBalancers.remarkPlaceholder')} />
          </FormField>

          <FormField label={t('pages.settings.subBalancers.strategy')} name="strategy" required>
            <Select options={strategies} />
          </FormField>

          <FormField
            label={t('pages.settings.subBalancers.sortOrder')}
            name="sortOrder"
            required
            tooltip={t('pages.settings.subBalancers.sortOrderHelp')}
            rules={{ validate: rhfZodValidate(SubBalancerFormSchema.shape.sortOrder) }}
          >
            <InputNumber min={1} precision={0} style={{ width: '100%' }} />
          </FormField>

          <FormField
            label={t('pages.settings.subBalancers.inbounds')}
            name="inboundIds"
            required
            rules={{ validate: rhfZodValidate(SubBalancerFormSchema.shape.inboundIds) }}
          >
            <Select
              mode="multiple"
              options={inboundOptions}
              maxTagCount="responsive"
              listHeight={220}
              showSearch={{ optionFilterProp: 'label' }}
            />
          </FormField>
          <SelectAllClearButtons
            options={inboundOptions}
            value={inboundIds}
            onChange={(v) => methods.setValue('inboundIds', v, { shouldDirty: true })}
          />

          <FormField
            label={t('pages.settings.subBalancers.enabled')}
            name="enabled"
            valueProp="checked"
          >
            <Switch />
          </FormField>
        </Form>
      </FormProvider>
    </Modal>
  );
}
