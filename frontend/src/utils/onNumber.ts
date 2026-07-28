/**
 * Wraps an Ant Design InputNumber change handler so a cleared field never
 * writes a synthetic value: null, empty-string, and NaN change events are
 * ignored, leaving the stored value in place (the input snaps back on blur).
 * This is the port-field semantic from the sub-port fix, shared so new
 * numeric settings cannot reintroduce the `Number(v) || 0` clamp bug.
 */
export function onNumber(
  apply: (value: number) => void,
): (value: number | string | null | undefined) => void {
  return (value) => {
    if (value == null || value === '') return;
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) return;
    apply(parsed);
  };
}
