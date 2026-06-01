import { Tooltip } from 'antd';

import { csv } from './helpers';

export default function CriterionRow({ label, value, title }: { label: string; value?: string; title: string }) {
  const parts = csv(value);
  if (parts.length === 0) return null;
  return (
    <Tooltip title={title}>
      <span className="criterion-row">
        <span className="criterion-label">{label}</span>
        <span className="criterion-value">{parts[0]}</span>
        {parts.length > 1 && <span className="criterion-more">+{parts.length - 1}</span>}
      </span>
    </Tooltip>
  );
}
