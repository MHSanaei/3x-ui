import { Tooltip } from 'antd';

import { csv } from './helpers';

export default function CriterionRow({ label, value, values, title }: { label: string; value?: string; values?: string[]; title: string }) {
  const parts = values ?? csv(value);
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
