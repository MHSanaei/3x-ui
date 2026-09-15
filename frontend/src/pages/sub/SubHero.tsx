import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Progress, Tag, theme } from 'antd';

import { IntlUtil } from '@/utils';
import type { CalendarKind } from '@/utils';
import { usagePercent } from './subPageModel';
import type { SubStatus } from './subPageModel';

interface SubHeroProps {
  status: SubStatus;
  daysLeft: number | null;
  usedByte: number;
  totalByte: number;
  expireMs: number;
  lastOnlineMs: number;
  download: string;
  upload: string;
  used: string;
  total: string;
  remained: string;
  datepicker: CalendarKind;
  lang: string;
}

const STATUS_TAGS: Record<SubStatus, { color: string; label: string }> = {
  active: { color: 'green', label: 'subscription.active' },
  unlimited: { color: 'purple', label: 'subscription.unlimited' },
  expired: { color: 'red', label: 'subscription.expired' },
  depleted: { color: 'red', label: 'subscription.depleted' },
  disabled: { color: 'red', label: 'subscription.inactive' },
};

// FormatTraffic renders "37.60GB"; the amount and unit are sized apart.
function splitSize(label: string): [string, string] {
  const match = /^([\d.,]+)\s*(\D*)$/.exec(label.trim());
  return match ? [match[1], match[2]] : [label, ''];
}

export default function SubHero({
  status,
  daysLeft,
  usedByte,
  totalByte,
  expireMs,
  lastOnlineMs,
  download,
  upload,
  used,
  total,
  remained,
  datepicker,
  lang,
}: SubHeroProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();

  const hasQuota = totalByte > 0;
  const healthy = status === 'active' || status === 'unlimited';
  const pct = usagePercent(usedByte, totalByte);
  const ringColor =
    !healthy || pct >= 90 ? token.colorError : pct >= 75 ? token.colorWarning : token.colorPrimary;
  const [amount, unit] = splitSize(hasQuota ? remained : used);
  const formatDate = (ms: number) => IntlUtil.formatDate(ms, datepicker, lang);
  const statusTag = STATUS_TAGS[status];

  const stats: { key: string; label: string; value: ReactNode }[] = [
    { key: 'days', label: t('subscription.daysLeft'), value: daysLeft ?? '∞' },
    {
      key: 'expiry',
      label: t('subscription.expiry'),
      value: expireMs > 0 ? formatDate(expireMs) : t('subscription.noExpiry'),
    },
    {
      key: 'status',
      label: t('subscription.status'),
      value: <Tag color={statusTag.color}>{t(statusTag.label)}</Tag>,
    },
    { key: 'down', label: t('subscription.downloaded'), value: <bdi>{download}</bdi> },
    { key: 'up', label: t('subscription.uploaded'), value: <bdi>{upload}</bdi> },
    { key: 'total', label: t('subscription.totalQuota'), value: <bdi>{total}</bdi> },
    {
      key: 'lastOnline',
      label: t('lastOnline'),
      value: lastOnlineMs > 0 ? formatDate(lastOnlineMs) : '-',
    },
  ];

  return (
    <section className={healthy ? 'sub-hero' : 'sub-hero is-alert'}>
      <Progress
        type="circle"
        className="sub-ring"
        percent={pct}
        status="normal"
        size={156}
        strokeColor={ringColor}
        format={() => (
          <span className="sub-ring-center">
            <span className="sub-ring-value">{hasQuota ? `${pct.toFixed(1)}%` : '∞'}</span>
            <span className="sub-ring-label">
              {hasQuota ? t('usage') : t('subscription.unlimited')}
            </span>
          </span>
        )}
      />
      <div className="sub-hero-summary">
        <div className="sub-label">{hasQuota ? t('remained') : t('usage')}</div>
        <bdi className="sub-big">
          <span className="sub-big-num">{amount}</span>
          {unit && <span className="sub-big-unit">{unit}</span>}
        </bdi>
        <div className="sub-muted">
          {hasQuota ? t('subscription.ofTotal', { total }) : t('subscription.unlimited')}
        </div>
        <dl className="sub-stats">
          {stats.map((stat) => (
            <div key={stat.key} className="sub-stat">
              <dt className="sub-label">{stat.label}</dt>
              <dd className="sub-stat-value">{stat.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}
