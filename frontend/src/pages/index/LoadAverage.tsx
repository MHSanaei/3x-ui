import type { CSSProperties } from 'react';
import { Tooltip, theme } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import type { Status } from '@/models/status';
import { loadAppearance } from './load-average';

export default function LoadAverage({ status }: { status: Status }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const title = t('pages.index.loadAverage');
  const comparable = !status.resourceScope && status.metricAvailable('loadCapacity');
  const windows = [1, 5, 15].map((minutes, index) => {
    const appearance = loadAppearance(
      status.metricAvailable('loads') ? status.loads[index] : null,
      status.logicalPro,
      comparable,
    );
    return {
      ...appearance,
      label: t('pages.index.loadWindow', { minutes }),
      text: appearance.value === null ? '—' : appearance.value.toFixed(2),
      description: t(`pages.index.loadSeverity.${appearance.severity}`),
    };
  });
  const primary = windows[0];
  const context = !comparable
    ? t('pages.index.loadScopeMismatch')
    : windows.some((window) => window.value !== null && window.ratio === null)
      ? t('pages.index.loadCapacityUnknown')
      : '';

  return (
    <div className="ov-load">
      <div className="ov-load-head">
        <span
          className="ov-load-indicator"
          role="img"
          aria-label={`${title} (${primary.label}): ${primary.description}`}
          data-severity={primary.severity}
          style={{ backgroundColor: primary.color ?? token.colorTextSecondary }}
        />
        <Tooltip
          trigger={['hover', 'focus', 'click']}
          title={
            <>
              {t('pages.index.loadHelp')}
              {context && (
                <>
                  <br />
                  {context}
                </>
              )}
            </>
          }
        >
          <button type="button" className="ov-load-help" aria-label={title}>
            {title} <InfoCircleOutlined aria-hidden />
          </button>
        </Tooltip>
      </div>
      <dl className="ov-load-values">
        {windows.map((window) => (
          <div
            key={window.label}
            className="ov-load-reading"
            data-severity={window.severity}
            style={{ '--load-color': window.color ?? token.colorTextSecondary } as CSSProperties}
          >
            <dt>{window.label}</dt>
            <dd
              aria-label={`${window.label}: ${window.text} — ${window.description}`}
              title={window.description}
            >
              {window.text}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
