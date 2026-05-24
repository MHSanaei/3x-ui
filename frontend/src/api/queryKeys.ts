export const keys = {
  server: {
    status: () => ['server', 'status'] as const,
  },
  nodes: {
    root: () => ['nodes'] as const,
    list: () => ['nodes', 'list'] as const,
  },
  settings: {
    root: () => ['settings'] as const,
    all: () => ['settings', 'all'] as const,
    defaults: () => ['settings', 'defaults'] as const,
  },
  inbounds: {
    root: () => ['inbounds'] as const,
    slim: () => ['inbounds', 'slim'] as const,
  },
  clients: {
    root: () => ['clients'] as const,
    onlines: () => ['clients', 'onlines'] as const,
    lastOnline: () => ['clients', 'lastOnline'] as const,
  },
} as const;
