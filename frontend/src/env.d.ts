/// <reference types="vite/client" />

interface SubPageData {
  sId?: string;
  enabled?: boolean;
  download?: string;
  upload?: string;
  total?: string;
  used?: string;
  remained?: string;
  totalByte?: string | number;
  expire?: string | number;
  lastOnline?: string | number;
  subUrl?: string;
  subJsonUrl?: string;
  subClashUrl?: string;
  subTitle?: string;
  links?: string[];
  emails?: string[];
  datepicker?: 'gregorian' | 'jalalian';
  announce?: string;
  downloadByte?: string | number;
  uploadByte?: string | number;
  usedByte?: string | number;
}

interface Window {
  X_UI_BASE_PATH?: string;
  X_UI_CUR_VER?: string;
  X_UI_DB_TYPE?: string;
  __SUB_PAGE_DATA__?: SubPageData;
}

declare module 'qs' {
  interface StringifyOptions {
    arrayFormat?: 'indices' | 'brackets' | 'repeat' | 'comma';
    encode?: boolean;
    encoder?: (str: unknown, defaultEncoder: (s: unknown) => string, charset: string, type: 'key' | 'value') => string;
    allowDots?: boolean;
    skipNulls?: boolean;
    addQueryPrefix?: boolean;
  }
  interface ParseOptions {
    depth?: number;
    arrayLimit?: number;
    allowDots?: boolean;
    parseArrays?: boolean;
    ignoreQueryPrefix?: boolean;
  }
  export function stringify(obj: unknown, options?: StringifyOptions): string;
  export function parse(str: string, options?: ParseOptions): Record<string, unknown>;
  const qs: { stringify: typeof stringify; parse: typeof parse };
  export default qs;
}

declare module 'persian-calendar-suite' {
  import type { ComponentType, ReactNode } from 'react';

  type DateInput = string | number | null;
  type OutputFormat = 'iso' | 'shamsi' | 'gregorian' | 'hijri' | 'timestamp';

  interface PersianDateTimePickerProps {
    value?: DateInput;
    onChange?: (value: number | string | null) => void;
    defaultValue?: string | number | 'now' | null;
    showTime?: boolean;
    minuteStep?: number;
    outputFormat?: OutputFormat;
    showFooter?: boolean;
    theme?: Record<string, unknown>;
    disabledHours?: number[];
    minDate?: string | Date | null;
    maxDate?: string | Date | null;
    enabledDates?: string[] | null;
    disabledDates?: string[] | null;
    disabledWeekDays?: number[];
    persianNumbers?: boolean;
    rtlCalendar?: boolean;
    placeholder?: string;
    disabled?: boolean;
    className?: string;
    children?: ReactNode;
  }

  export const PersianDateTimePicker: ComponentType<PersianDateTimePickerProps>;
  export const PersianCalendar: ComponentType<Record<string, unknown>>;
  export const PersianDateRangePicker: ComponentType<Record<string, unknown>>;
  export const PersianTimePicker: ComponentType<Record<string, unknown>>;
  export const PersianTimeline: ComponentType<Record<string, unknown>>;
}
