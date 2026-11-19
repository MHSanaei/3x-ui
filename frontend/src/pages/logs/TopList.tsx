import type { LogCount } from '@/generated/types';

interface TopListProps {
  items: LogCount[];
  activeValue: string;
  labelFor?: (value: string) => string | undefined;
  selectable: (value: string) => boolean;
  onSelect: (value: string) => void;
}

export default function TopList({
  items,
  activeValue,
  labelFor,
  selectable,
  onSelect,
}: TopListProps) {
  const peak = items[0]?.count ?? 0;
  return (
    <ul className="logs-toplist">
      {items.map((item) => {
        const body = (
          <>
            <span className="logs-toplist-label" title={item.value}>
              <bdi>{labelFor?.(item.value) ?? item.value}</bdi>
            </span>
            <span className="logs-toplist-count">{item.count.toLocaleString()}</span>
            <span className="logs-toplist-track" aria-hidden>
              <span
                className="logs-toplist-fill"
                style={{ width: `${peak ? (item.count / peak) * 100 : 0}%` }}
              />
            </span>
          </>
        );
        return (
          <li key={item.value}>
            {selectable(item.value) ? (
              <button
                type="button"
                className="logs-toplist-row"
                aria-pressed={activeValue === item.value}
                onClick={() => onSelect(item.value)}
              >
                {body}
              </button>
            ) : (
              <div className="logs-toplist-row">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
