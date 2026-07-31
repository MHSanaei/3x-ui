import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Tooltip, Typography } from 'antd';
import { DatabaseOutlined } from '@ant-design/icons';

import { useValidateGeoTokens, type GeoTokenKind } from '@/api/queries/useGeodata';
import { parseTokens } from '@/lib/xray/geoTokens';
import type { GeodataTokenIssue, GeoKind } from '@/generated/types';

import GeoBrowserModal from './GeoBrowserModal';

const VALIDATION_DELAY = 600;

// Each reason needs its own wording: a missing database is fixed under Geodata,
// a missing category by picking another one, and a bad token by editing it.
const REASON_KEYS: Record<string, string> = {
  fileMissing: 'pages.xray.geoBrowser.missingDatabase',
  categoryMissing: 'pages.xray.geoBrowser.unknownCategories',
  attributeMissing: 'pages.xray.geoBrowser.unknownAttribute',
  syntax: 'pages.xray.geoBrowser.invalidToken',
};

export interface GeoTokenInputProps {
  value?: string;
  onChange?: (value: string) => void;
  onBlur?: () => void;
  kind: GeoTokenKind;
  placeholder?: string;
  id?: string;
}

export default function GeoTokenInput({ value = '', onChange, onBlur, kind, placeholder, id }: GeoTokenInputProps) {
  const { t } = useTranslation();
  const [browsing, setBrowsing] = useState(false);
  const [issues, setIssues] = useState<GeodataTokenIssue[]>([]);
  const validate = useValidateGeoTokens();
  const { mutateAsync } = validate;

  useEffect(() => {
    const tokens = parseTokens(value);
    if (tokens.length === 0) {
      setIssues([]);
      return;
    }
    let cancelled = false;
    const timer = setTimeout(() => {
      mutateAsync({ tokens, kind })
        .then((found) => {
          if (!cancelled) setIssues(found);
        })
        .catch(() => {
          if (!cancelled) setIssues([]);
        });
    }, VALIDATION_DELAY);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [value, kind, mutateAsync]);

  return (
    <>
      <Input
        id={id}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange?.(event.target.value)}
        onBlur={onBlur}
        addonAfter={
          <Tooltip title={t('pages.xray.geoBrowser.openTooltip')}>
            <Button
              type="text"
              size="small"
              icon={<DatabaseOutlined />}
              aria-label={t('pages.xray.geoBrowser.openTooltip')}
              onClick={() => setBrowsing(true)}
            />
          </Tooltip>
        }
      />
      {groupByReason(issues).map(([reason, tokens]) => (
        <Typography.Text key={reason} type="warning" className="geo-unknown-hint">
          {t(REASON_KEYS[reason] ?? REASON_KEYS.categoryMissing, { tokens: tokens.join(', ') })}
        </Typography.Text>
      ))}
      <GeoBrowserModal
        open={browsing}
        kind={(kind === 'ip' ? 'ip' : 'site') as GeoKind}
        value={value}
        onApply={(next) => {
          onChange?.(next);
          setBrowsing(false);
        }}
        onClose={() => setBrowsing(false)}
      />
    </>
  );
}

function groupByReason(issues: GeodataTokenIssue[]): Array<[string, string[]]> {
  const grouped = new Map<string, string[]>();
  for (const issue of issues) {
    const tokens = grouped.get(issue.reason) ?? [];
    tokens.push(issue.token);
    grouped.set(issue.reason, tokens);
  }
  return [...grouped];
}
