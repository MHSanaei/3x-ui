import type { ReactNode } from 'react';
import { Statistic } from 'antd';
import './CustomStatistic.css';

interface CustomStatisticProps {
  title?: string;
  value?: string | number;
  prefix?: ReactNode;
  suffix?: ReactNode;
}

export default function CustomStatistic({ title = '', value = '', prefix, suffix }: CustomStatisticProps) {
  return <Statistic title={title} value={value} prefix={prefix} suffix={suffix} />;
}
