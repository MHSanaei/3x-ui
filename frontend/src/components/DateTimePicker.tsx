// React port of DateTimePicker.vue. For now this delegates to AntD's
// <DatePicker>; the Jalali calendar UI from vue3-persian-datetime-picker
// has no clean React equivalent and is tracked as a follow-up for when
// the inbounds entry migrates. Read-only Jalali display still works via
// IntlUtil.formatDate, which uses Intl.DateTimeFormat with the persian
// calendar extension.

import { DatePicker } from 'antd';
import type { Dayjs } from 'dayjs';

interface DateTimePickerProps {
  value: Dayjs | null;
  onChange: (next: Dayjs | null) => void;
  showTime?: boolean;
  format?: string;
  placeholder?: string;
  disabled?: boolean;
}

export default function DateTimePicker({
  value,
  onChange,
  showTime = true,
  format = 'YYYY-MM-DD HH:mm:ss',
  placeholder = '',
  disabled = false,
}: DateTimePickerProps) {
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
