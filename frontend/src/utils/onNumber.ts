/**
 * Wraps an Ant Design InputNumber change handler with the shared
 * cleared-field semantic: null and undefined change events (a cleared or
 * unparsable field) are ignored, leaving the stored value in place — the
 * input snaps back on blur — while real numbers pass through unchanged.
 * Number-only by design: not for `stringMode` inputs, whose whole point is
 * to avoid the IEEE-754 round trip this signature would force.
 */
export function onNumber(
  apply: (value: number) => void,
): (value: number | null | undefined) => void {
  return (value) => {
    if (typeof value !== 'number' || !Number.isFinite(value)) return;
    apply(value);
  };
}
