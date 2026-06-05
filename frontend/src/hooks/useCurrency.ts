import { useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import { useMe } from '@/hooks/useMe';

/**
 * useCurrency centralizes how wallet amounts are rendered. The balance is held
 * as integer credits where 1 credit == 1 currency unit; this formats it with
 * thousand separators and the localized unit word (Toman / Rial) so users
 * actually understand what the number means.
 */
export function useCurrency() {
  const { me } = useMe();
  const { t } = useTranslation();

  const code = me?.currency || 'IRT';
  const unit = useMemo(() => t(`currency.${code}`, { defaultValue: code }), [t, code]);

  const formatNumber = useCallback((amount: number) => new Intl.NumberFormat().format(Math.round(amount || 0)), []);

  // "135,000 Toman"
  const format = useCallback(
    (amount: number) => `${formatNumber(amount)} ${unit}`,
    [formatNumber, unit],
  );

  return { format, formatNumber, unit, code, clientCost: me?.clientCost ?? 0, clientCostPerGB: me?.clientCostPerGB ?? 0 };
}

const BYTES_PER_GB = 1024 * 1024 * 1024;

/**
 * computeClientCost mirrors the server formula in web/service/cost.go.
 * totalBytes is the client's traffic quota in bytes (0 = unlimited → base only).
 */
export function computeClientCost(base: number, perGB: number, totalBytes: number): number {
  let cost = base || 0;
  if (perGB > 0 && totalBytes > 0) {
    cost += Math.round((totalBytes / BYTES_PER_GB) * perGB);
  }
  return cost < 0 ? 0 : cost;
}
