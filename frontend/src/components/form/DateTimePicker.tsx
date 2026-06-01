import { useMemo } from 'react';
import { DatePicker } from 'antd';
import dayjs from 'dayjs';
import type { Dayjs } from 'dayjs';
import { PersianDateTimePicker } from 'persian-calendar-suite';

import { useDatepicker } from '@/hooks/useDatepicker';
import { useTheme } from '@/hooks/useTheme';
import './DateTimePicker.css';

interface DateTimePickerProps {
  value: Dayjs | null;
  onChange: (next: Dayjs | null) => void;
  showTime?: boolean;
  format?: string;
  placeholder?: string;
  disabled?: boolean;
}

const LIGHT_THEME = {
  primaryColor: '#1677ff',
  backgroundColor: '#ffffff',
  borderColor: '#d9d9d9',
  hoverColor: 'rgba(22, 119, 255, 0.10)',
  selectedTextColor: '#ffffff',
  textColor: 'rgba(0, 0, 0, 0.88)',
};

const DARK_THEME = {
  primaryColor: '#1677ff',
  backgroundColor: '#23252b',
  borderColor: 'rgba(255, 255, 255, 0.12)',
  hoverColor: 'rgba(22, 119, 255, 0.18)',
  selectedTextColor: '#ffffff',
  textColor: 'rgba(255, 255, 255, 0.88)',
};

const ULTRA_DARK_THEME = {
  primaryColor: '#1677ff',
  backgroundColor: '#101013',
  borderColor: 'rgba(255, 255, 255, 0.08)',
  hoverColor: 'rgba(22, 119, 255, 0.16)',
  selectedTextColor: '#ffffff',
  textColor: 'rgba(255, 255, 255, 0.88)',
};

export default function DateTimePicker({
  value,
  onChange,
  showTime = true,
  format = 'YYYY-MM-DD HH:mm:ss',
  placeholder = '',
  disabled = false,
}: DateTimePickerProps) {
  const { datepicker } = useDatepicker();
  const { isDark, isUltra } = useTheme();

  const persianTheme = useMemo(() => {
    if (isUltra) return ULTRA_DARK_THEME;
    if (isDark) return DARK_THEME;
    return LIGHT_THEME;
  }, [isDark, isUltra]);

  if (datepicker === 'jalalian') {
    return (
      <div className={`jdp-wrap${isDark ? ' jdp-dark' : ''}${isUltra ? ' jdp-ultra' : ''}${disabled ? ' jdp-disabled' : ''}`}>
        <PersianDateTimePicker
          value={value ? value.valueOf() : null}
          onChange={(next: number | string | null) => {
            if (next == null || next === '') {
              onChange(null);
              return;
            }
            const ms = typeof next === 'number' ? next : Number(next);
            if (Number.isFinite(ms)) onChange(dayjs(ms));
          }}
          showTime={showTime}
          outputFormat="timestamp"
          persianNumbers
          rtlCalendar
          theme={persianTheme}
        />
      </div>
    );
  }

  return (
    <DatePicker
      value={value}
      onChange={(next) => onChange(next || null)}
      showTime={showTime ? { format: 'HH:mm:ss' } : false}
      format={format}
      placeholder={placeholder}
      disabled={disabled}
      style={{ width: '100%' }}
    />
  );
}
