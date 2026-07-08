export type ValueProp = 'value' | 'checked';

export function normalizeAntdOnChange(args: unknown[], valueProp: ValueProp): unknown {
  const first = args[0];
  if (first !== null && typeof first === 'object' && 'target' in first) {
    const target = (first as { target: { value?: unknown; checked?: unknown } }).target;
    return valueProp === 'checked' ? target.checked : target.value;
  }
  return first;
}
