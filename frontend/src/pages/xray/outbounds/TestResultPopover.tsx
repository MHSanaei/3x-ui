import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Popover } from 'antd';
import { CheckCircleFilled, CloseCircleFilled } from '@ant-design/icons';

import type { OutboundTestResult } from '@/hooks/useXraySetting';

interface TestResultPopoverProps {
  result: OutboundTestResult;
  // Custom trigger element; defaults to the ok/fail latency pill.
  children?: ReactNode;
}

// Latency pill + detail popover for an outbound test result: per-endpoint
// dial outcomes for TCP probes, HTTP status and the timing breakdown for
// HTTP probes.
export default function TestResultPopover({ result: r, children }: TestResultPopoverProps) {
  const { t } = useTranslation();

  const breakdown: Array<{ key: string; label: string; value: string }> = [];
  if (typeof r.httpStatus === 'number') {
    breakdown.push({ key: 'status', label: t('pages.xray.outbound.httpStatus'), value: String(r.httpStatus) });
  }
  if (typeof r.connectMs === 'number') {
    breakdown.push({ key: 'connect', label: t('pages.xray.outbound.breakdownConnect'), value: `${r.connectMs} ms` });
  }
  if (typeof r.tlsMs === 'number') {
    breakdown.push({ key: 'tls', label: t('pages.xray.outbound.breakdownTls'), value: `${r.tlsMs} ms` });
  }
  if (typeof r.ttfbMs === 'number') {
    breakdown.push({ key: 'ttfb', label: t('pages.xray.outbound.breakdownTtfb'), value: `${r.ttfbMs} ms` });
  }

  return (
    <Popover
      placement="topLeft"
      rootClassName="outbound-test-popover"
      content={
        <div className="timing-breakdown">
          <div className={`td-head ${r.success ? 'ok' : 'fail'}`}>
            {r.success ? <span>{r.delay} ms</span> : <span>{r.error || 'failed'}</span>}
            {r.mode && <span className="mode-badge">{String(r.mode).toUpperCase()}</span>}
          </div>
          {(r.endpoints || []).map((ep) => (
            <div key={ep.address} className="endpoint-row">
              <span className={ep.success ? 'dot-ok' : 'dot-fail'}>●</span>
              <span className="ep-addr">{ep.address}</span>
              <span className="ep-meta">{ep.success ? `${ep.delay} ms` : ep.error || 'failed'}</span>
            </div>
          ))}
          {breakdown.map((row) => (
            <div key={row.key} className="breakdown-row">
              <span className="bd-label">{row.label}</span>
              <span className="bd-value">{row.value}</span>
            </div>
          ))}
        </div>
      }
    >
      {children ?? (
        <span className={r.success ? 'pill-ok' : 'pill-fail'}>
          {r.success ? <CheckCircleFilled /> : <CloseCircleFilled />}
          {r.success ? <span>{r.delay}&nbsp;ms</span> : <span>failed</span>}
        </span>
      )}
    </Popover>
  );
}
