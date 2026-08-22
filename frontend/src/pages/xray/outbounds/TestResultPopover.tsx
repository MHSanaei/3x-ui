import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Popover } from 'antd';
import { CheckCircleFilled, CloseCircleFilled, ExclamationCircleFilled } from '@ant-design/icons';

import type { OutboundTestResult } from '@/hooks/useXraySetting';

import { testModeLabel } from './outbounds-tab-helpers';

interface TestResultPopoverProps {
  result: OutboundTestResult;
  // Custom trigger element; defaults to the ok/fail latency pill.
  children?: ReactNode;
}

function fmtMbps(v?: number): string {
  return typeof v === 'number' ? v.toFixed(1) : '—';
}

// Latency pill + detail popover for an outbound test result: per-endpoint
// dial outcomes for TCP probes, HTTP status and the timing breakdown for
// HTTP probes.
export default function TestResultPopover({ result: r, children }: TestResultPopoverProps) {
  const { t } = useTranslation();
  const isSpeed = r.mode === 'speed';
  // probeSpeedThroughSocks sets Success as soon as either direction measures
  // something, so a one-sided failure still reports success -- this flag
  // keeps that failure from being silently swallowed.
  const speedPartialFailure = isSpeed && r.success && !!r.error;
  const speedSummary = `↓${fmtMbps(r.downloadMbps)} / ↑${fmtMbps(r.uploadMbps)} ${t('pages.xray.outbound.mbpsUnit')}`;

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
  if (typeof r.downloadMbps === 'number') {
    breakdown.push({ key: 'download', label: t('pages.xray.outbound.breakdownDownload'), value: `${fmtMbps(r.downloadMbps)} ${t('pages.xray.outbound.mbpsUnit')}` });
  }
  if (typeof r.uploadMbps === 'number') {
    breakdown.push({ key: 'upload', label: t('pages.xray.outbound.breakdownUpload'), value: `${fmtMbps(r.uploadMbps)} ${t('pages.xray.outbound.mbpsUnit')}` });
  }

  return (
    <Popover
      placement="topLeft"
      rootClassName="outbound-test-popover"
      content={
        <div className="timing-breakdown">
          <div className={`td-head ${r.success ? 'ok' : 'fail'}`}>
            {r.success
              ? <span>{isSpeed ? speedSummary : `${r.delay} ms`}</span>
              : <span>{r.error || 'failed'}</span>}
            {r.mode && <span className="mode-badge">{testModeLabel(String(r.mode), t)}</span>}
          </div>
          {speedPartialFailure && <div className="td-head-partial">{r.error}</div>}
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
        <span className={r.success ? (speedPartialFailure ? 'pill-warn' : 'pill-ok') : 'pill-fail'}>
          {r.success ? (speedPartialFailure ? <ExclamationCircleFilled /> : <CheckCircleFilled />) : <CloseCircleFilled />}
          {r.success
            ? (isSpeed
              ? <span>{speedSummary}</span>
              : <span>{r.delay}&nbsp;ms</span>)
            : <span>failed</span>}
        </span>
      )}
    </Popover>
  );
}
