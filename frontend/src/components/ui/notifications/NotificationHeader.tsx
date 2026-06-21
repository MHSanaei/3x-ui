import { useRef, useEffect } from 'react';
import { Tag } from 'antd';

interface Props {
  count: number;
  total: number;
  allSelected: boolean;
  indeterminate: boolean;
  onToggleAll: () => void;
}

function MasterCheckbox({ checked, indeterminate, onChange }: { checked: boolean; indeterminate: boolean; onChange: () => void }) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return <input ref={ref} type="checkbox" checked={checked} onChange={onChange} style={{ cursor: 'pointer' }} />;
}

export function NotificationHeader({ count, total, allSelected, indeterminate, onToggleAll }: Props) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      <Tag>{count}/{total}</Tag>
      <MasterCheckbox checked={allSelected} indeterminate={indeterminate} onChange={onToggleAll} />
    </span>
  );
}
