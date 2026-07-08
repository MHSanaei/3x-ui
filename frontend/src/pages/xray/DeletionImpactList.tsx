import { useTranslation } from 'react-i18next';

import type { DeletionImpact } from './reference-cleanup';

interface DeletionImpactListProps {
  impact: DeletionImpact;
}

export default function DeletionImpactList({ impact }: DeletionImpactListProps) {
  const { t } = useTranslation();

  const lines: string[] = [];
  for (const rule of impact.rules) {
    lines.push(
      rule.fate === 'removed'
        ? t('pages.xray.refCleanup.ruleRemoved', { label: rule.label })
        : t('pages.xray.refCleanup.ruleModified', { label: rule.label, keeps: rule.keeps ?? '' }),
    );
  }
  for (const balancer of impact.balancers) {
    lines.push(t('pages.xray.refCleanup.balancerRemoved', { tag: balancer.tag }));
  }
  if (impact.observatory) lines.push(t('pages.xray.observatory.deleteAlsoObservatory'));
  if (impact.burst) lines.push(t('pages.xray.observatory.deleteAlsoBurst'));

  if (lines.length === 0) return null;

  return (
    <div>
      <p style={{ marginBottom: 8 }}>{t('pages.xray.refCleanup.header')}</p>
      <ul style={{ margin: 0, paddingInlineStart: 20 }}>
        {lines.map((line, i) => (
          <li key={i}>
            <bdi>{line}</bdi>
          </li>
        ))}
      </ul>
    </div>
  );
}
