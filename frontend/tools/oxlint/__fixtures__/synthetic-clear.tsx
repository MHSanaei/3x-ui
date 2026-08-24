// Fixtures for input-number-guard. Each handler below is a shape the guard must
// reject; ok.tsx holds the onNumber() form it must accept.
import { InputNumber } from 'antd';

declare function setPort(value: number): void;

export const NumberOr = () => <InputNumber onChange={(v) => setPort(Number(v) || 443)} />;
export const TypeofTernary = () => (
  <InputNumber onChange={(v) => setPort(typeof v === 'number' ? v : 443)} />
);
export const NullishLiteral = () => <InputNumber onChange={(v) => setPort(v ?? 443)} />;
