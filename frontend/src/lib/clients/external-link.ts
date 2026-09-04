export function resolveExternalLinkExpiry(
  externalExpiry: number | null | undefined,
  clientExpiry: number | null | undefined,
): number {
  const explicitExpiry = Number(externalExpiry) || 0;
  if (explicitExpiry > 0) return explicitExpiry;

  const inheritedExpiry = Number(clientExpiry) || 0;
  return inheritedExpiry > 0 ? inheritedExpiry : 0;
}
