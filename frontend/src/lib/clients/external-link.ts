const SCOPE_LABEL_KEY: Record<string, string> = {
  client: 'pages.links.scopeClient',
  group: 'pages.links.scopeGroup',
  inbound: 'pages.links.scopeInbound',
  global: 'pages.links.scopeGlobal',
  new_clients: 'pages.links.scopeNewClients',
};

// Returns an i18n key, not text: the caller owns the translation.
export function externalLinkScopeLabel(scope: string): string {
  return SCOPE_LABEL_KEY[scope] ?? scope;
}

export function resolveExternalLinkExpiry(
  externalExpiry: number | null | undefined,
  clientExpiry: number | null | undefined,
): number {
  const explicitExpiry = Number(externalExpiry) || 0;
  if (explicitExpiry > 0) return explicitExpiry;

  const inheritedExpiry = Number(clientExpiry) || 0;
  return inheritedExpiry > 0 ? inheritedExpiry : 0;
}
