import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Input, Modal, Select, Switch, message } from 'antd';
import { FormProvider, useForm, useWatch } from 'react-hook-form';

import { selectTagRender } from '@/components/form';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import SelectAllClearButtons from '@/components/form/SelectAllClearButtons';
import {
  ChainSelectorFormSchema,
  ChainSelectorStrategySchema,
  type ChainProxy,
  type ChainSelector,
  type ChainSelectorFormValues,
  type ChainSelectorStrategy,
} from '@/schemas/chainProxy';

const STRATEGY_LABEL_KEYS: Record<ChainSelectorStrategy, string> = {
  leastLoad: 'pages.settings.chainSelectors.strategyLeastLoad',
  leastPing: 'pages.settings.chainSelectors.strategyLeastPing',
  random: 'pages.settings.chainSelectors.strategyRandom',
  roundRobin: 'pages.settings.chainSelectors.strategyRoundRobin',
};

function initialState(selector: ChainSelector | null): ChainSelectorFormValues {
  return {
    remark: selector?.remark ?? '',
    strategy: selector?.strategy ?? 'random',
    proxyIds: [...(selector?.proxyIds ?? [])],
    enabled: selector?.enabled ?? true,
  };
}

interface ChainSelectorFormModalProps {
  open: boolean;
  selector: ChainSelector | null;
  proxies: ChainProxy[];
  onClose: () => void;
  onConfirm: (values: ChainSelectorFormValues) => void;
}

export default function ChainSelectorFormModal({
  open,
  selector,
  proxies,
  onClose,
  onConfirm,
}: ChainSelectorFormModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<ChainSelectorFormValues>({ defaultValues: initialState(selector) });
  const isEdit = selector != null;

  useEffect(() => {
    if (open) methods.reset(initialState(selector));
  }, [open, selector, methods]);

  const proxyIds = useWatch({ control: methods.control, name: 'proxyIds' });

  const proxyOptions = useMemo(
    () =>
      proxies
        .filter((proxy) => proxy.enabled || (proxyIds ?? []).includes(proxy.id))
        .map((proxy) => ({
          label: `${proxy.remark} · ${proxy.type}://${proxy.server}:${proxy.port}`,
          value: proxy.id,
        })),
    [proxies, proxyIds],
  );

  function onFinish(values: ChainSelectorFormValues) {
    const parsed = ChainSelectorFormSchema.safeParse(values);
    if (!parsed.success) {
      messageApi.error(
        t(parsed.error.issues[0]?.message ?? 'pages.settings.chainSelectors.errRemarkRequired'),
      );
      return;
    }
    onConfirm(parsed.data);
  }

  const strategies = ChainSelectorStrategySchema.options.map((value) => ({
    value,
    label: t(STRATEGY_LABEL_KEYS[value]),
  }));

  return (
    <Modal
      open={open}
      title={
        isEdit
          ? `${t('edit')} ${t('pages.settings.chainSelectors.title')}`
          : `+ ${t('pages.settings.chainSelectors.add')}`
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
            label={t('pages.settings.chainSelectors.remark')}
            name="remark"
            required
            rules={{ validate: rhfZodValidate(ChainSelectorFormSchema.shape.remark) }}
          >
            <Input placeholder={t('pages.settings.chainSelectors.remarkPlaceholder')} />
          </FormField>

          <FormField
            label={t('pages.settings.chainSelectors.strategy')}
            name="strategy"
            required
            tooltip={t('pages.settings.chainSelectors.strategyHelp')}
          >
            <Select options={strategies} />
          </FormField>

          <FormField
            label={t('pages.settings.chainSelectors.proxies')}
            name="proxyIds"
            required
            rules={{ validate: rhfZodValidate(ChainSelectorFormSchema.shape.proxyIds) }}
          >
            <Select
              mode="multiple"
              tagRender={selectTagRender}
              options={proxyOptions}
              listHeight={220}
              showSearch={{ optionFilterProp: 'label' }}
            />
          </FormField>
          <SelectAllClearButtons
            options={proxyOptions}
            value={proxyIds || []}
            onChange={(v) => methods.setValue('proxyIds', v, { shouldDirty: true })}
          />

          <FormField
            label={t('pages.settings.chainSelectors.enabled')}
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
