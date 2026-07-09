export type FieldName = string | ReadonlyArray<string | number>;

export function toDotted(name: FieldName): string {
  return typeof name === 'string' ? name : name.join('.');
}
