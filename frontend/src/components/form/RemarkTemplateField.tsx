import { useRef } from 'react';
import { Button, Input, Popover, Tooltip } from 'antd';
import type { InputRef } from 'antd';
import { CodeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import { hasRemarkTokens, previewRemark, wrapToken } from '@/lib/remark/remarkVariables';
import RemarkVarPicker from './RemarkVarPicker';

interface RemarkTemplateFieldProps {
  // Injected by antd Form.Item:
  value?: string;
  onChange?: (value: string) => void;
  maxLength?: number;
  placeholder?: string;
}

/**
 * RemarkTemplateField is a text input augmented with a {{VAR}} template picker
 * (insert-at-caret) and a live, sample-based preview of the expanded result.
 * Used for the global subscription Remark Template.
 */
export default function RemarkTemplateField({ value = '', onChange, maxLength, placeholder }: RemarkTemplateFieldProps) {
  const { t } = useTranslation();
  const inputRef = useRef<InputRef>(null);

  function insertToken(token: string) {
    const el = inputRef.current?.input;
    const start = el?.selectionStart ?? value.length;
    const end = el?.selectionEnd ?? value.length;
    const insert = wrapToken(token);
    const next = value.slice(0, start) + insert + value.slice(end);
    onChange?.(maxLength ? next.slice(0, maxLength) : next);
    const caret = start + insert.length;
    // The controlled value updates next render; restore the caret after it.
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(caret, caret);
    });
  }

  return (
    <div>
      <Input
        ref={inputRef}
        value={value}
        maxLength={maxLength}
        placeholder={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        suffix={
          <Popover
            content={<RemarkVarPicker onPick={insertToken} />}
            trigger="click"
            placement="bottomRight"
            title={t('pages.hosts.remarkVars.title')}
          >
            <Tooltip title={t('pages.hosts.remarkVars.title')}>
              <Button type="text" size="small" icon={<CodeOutlined />} aria-label={t('pages.hosts.remarkVars.title')} style={{ marginInlineEnd: -7 }} />
            </Tooltip>
          </Popover>
        }
      />
      {hasRemarkTokens(value) && (
        <div style={{ fontSize: 12, marginTop: 4, opacity: 0.7 }}>
          {t('pages.hosts.remarkVars.preview')}:{' '}
          <span style={{ fontFamily: 'monospace' }}>{previewRemark(value) || '—'}</span>
        </div>
      )}
    </div>
  );
}
