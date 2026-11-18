import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, QRCode, Segmented, Tag } from 'antd';
import { CopyOutlined, DownloadOutlined } from '@ant-design/icons';

import SubQrButton from './SubQrButton';

type SubLinkKind = 'SUB' | 'JSON' | 'CLASH';

interface SubLinkRow {
  kind: SubLinkKind;
  color: string;
  url: string;
  title: string;
  downloadable: boolean;
}

interface SubLinksTabProps {
  subUrl: string;
  subJsonUrl: string;
  subClashUrl: string;
  onCopy: (value: string) => void;
}

const appendRawView = (url: string) => `${url}${url.includes('?') ? '&' : '?'}view=raw`;

const trimTrailingSlash = (path: string) => (path.length > 1 ? path.replace(/\/+$/, '') : path);

function parseRowUrl(raw: string, baseOrigin: string): URL | null {
  try {
    return new URL(raw, baseOrigin);
  } catch {
    return null;
  }
}

function defaultQrKind(rows: SubLinkRow[]): SubLinkKind | undefined {
  if (rows.length === 0 || typeof window === 'undefined') return rows[0]?.kind;
  const { origin, pathname } = window.location;
  const currentPath = trimTrailingSlash(pathname);
  const parsed = rows.map((row) => ({ kind: row.kind, url: parseRowUrl(row.url, origin) }));
  // Browser visits to the JSON or Clash URL render this same HTML page.
  const exact = parsed.find(
    (entry) =>
      entry.url?.origin === origin && trimTrailingSlash(entry.url.pathname) === currentPath,
  );
  if (exact) return exact.kind;
  if (currentPath !== '/') {
    const byPath = parsed.find(
      (entry) => entry.url !== null && trimTrailingSlash(entry.url.pathname) === currentPath,
    );
    if (byPath) return byPath.kind;
  }
  return rows[0].kind;
}

export default function SubLinksTab({ subUrl, subJsonUrl, subClashUrl, onCopy }: SubLinksTabProps) {
  const { t } = useTranslation();
  const subLabel = t('pages.settings.subSettings');
  const rows: SubLinkRow[] = [
    { kind: 'SUB' as const, color: 'green', url: subUrl, title: subLabel, downloadable: false },
    {
      kind: 'JSON' as const,
      color: 'purple',
      url: subJsonUrl,
      title: `${subLabel} JSON`,
      downloadable: true,
    },
    {
      kind: 'CLASH' as const,
      color: 'gold',
      url: subClashUrl,
      title: 'Clash / Mihomo',
      downloadable: true,
    },
  ].filter((row) => row.url);

  const [selectedKind, setSelectedKind] = useState<SubLinkKind | undefined>(() =>
    defaultQrKind(rows),
  );
  const activeRow = rows.find((row) => row.kind === selectedKind) ?? rows[0];

  return (
    <div className="sub-rows">
      {rows.map((row) => (
        <div key={row.kind} className="sub-row">
          <Tag color={row.color} className="sub-row-tag">
            {row.kind}
          </Tag>
          <div className="sub-row-main">
            <a href={row.url} target="_blank" rel="noopener noreferrer" className="sub-row-title">
              {row.title}
            </a>
            <div className="sub-row-url" dir="ltr" title={row.url}>
              {row.url}
            </div>
          </div>
          <div className="sub-row-actions">
            {row.downloadable && (
              <Button
                href={appendRawView(row.url)}
                target="_blank"
                rel="noopener noreferrer"
                icon={<DownloadOutlined aria-hidden />}
                aria-label={t('download')}
                title={t('download')}
              />
            )}
            <Button
              icon={<CopyOutlined aria-hidden />}
              onClick={() => onCopy(row.url)}
              aria-label={t('copy')}
              title={t('copy')}
            />
            <SubQrButton value={row.url} label={row.title} onCopy={onCopy} />
          </div>
        </div>
      ))}
      {activeRow && (
        <div className="sub-qr-card">
          <div className="sub-qr-code">
            <QRCode
              value={activeRow.url}
              size={112}
              type="svg"
              bordered={false}
              color="#000000"
              bgColor="#ffffff"
            />
          </div>
          <div className="sub-qr-body">
            <div className="sub-qr-title">{t('subscription.scanTitle')}</div>
            <div className="sub-muted">{t('subscription.scanHint')}</div>
            {rows.length > 1 && (
              <Segmented<SubLinkKind>
                size="small"
                className="sub-qr-segmented"
                value={activeRow.kind}
                onChange={setSelectedKind}
                options={rows.map((row) => ({ value: row.kind, label: row.kind }))}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}
