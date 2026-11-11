import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Form, Select } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { useChainProxiesQuery, useChainSelectorsQuery } from '@/api/queries/useChainProxyQueries';

// Mirrors internal/sub's chainableProtocol: only these protocols build a
// streamSettings-bearing client outbound a hop can ride on.
export const CHAINABLE_PROTOCOLS = new Set(['vmess', 'vless', 'trojan', 'shadowsocks']);

const CHAIN_NONE = '';
const SELECTOR_PREFIX = 'selector:';

interface ChainAssignmentFieldsProps {
  protocol: string;
}

// The wire carries the assignment as two mutually exclusive ids, but one select
// is what an operator understands, so it edits both and clears the other.
export default function ChainAssignmentFields({ protocol }: ChainAssignmentFieldsProps) {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const { proxies } = useChainProxiesQuery();
  const { selectors } = useChainSelectorsQuery();
  const chainProxyId = useWatch({ control, name: 'chainProxyId' }) as number | undefined;
  const chainSelectorId = useWatch({ control, name: 'chainSelectorId' }) as number | undefined;

  const options = useMemo(() => {
    const items: { label: string; value: string }[] = [
      { label: t('pages.inbounds.form.chainNone'), value: CHAIN_NONE },
    ];
    for (const proxy of proxies) {
      items.push({
        label: `${proxy.remark} · ${proxy.type}://${proxy.server}:${proxy.port}`,
        value: String(proxy.id),
      });
    }
    for (const selector of selectors) {
      items.push({
        label: `${selector.remark} · ${t('pages.inbounds.form.chainSelectorSuffix')}`,
        value: `${SELECTOR_PREFIX}${selector.id}`,
      });
    }
    return items;
  }, [proxies, selectors, t]);

  if (!CHAINABLE_PROTOCOLS.has(protocol)) return null;

  const value =
    chainSelectorId && chainSelectorId > 0
      ? `${SELECTOR_PREFIX}${chainSelectorId}`
      : chainProxyId && chainProxyId > 0
        ? String(chainProxyId)
        : CHAIN_NONE;

  function onChange(next: string) {
    const isSelector = next.startsWith(SELECTOR_PREFIX);
    setValue('chainProxyId', !isSelector && next !== CHAIN_NONE ? Number(next) : 0, {
      shouldDirty: true,
    });
    setValue('chainSelectorId', isSelector ? Number(next.slice(SELECTOR_PREFIX.length)) : 0, {
      shouldDirty: true,
    });
  }

  return (
    <Form.Item
      htmlFor="inbound-chain"
      label={t('pages.inbounds.form.chain')}
      tooltip={t('pages.inbounds.form.chainHelp')}
    >
      <Select
        id="inbound-chain"
        value={value}
        options={options}
        onChange={onChange}
        showSearch={{ optionFilterProp: 'label' }}
      />
    </Form.Item>
  );
}
