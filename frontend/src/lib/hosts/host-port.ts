function addressOf(entry: string): string {
  const value = entry.trim();
  if (value.startsWith('[')) {
    const close = value.indexOf(']');
    return close === -1 ? value : value.slice(1, close);
  }
  if (value.split(':').length > 2) return value;
  const colon = value.lastIndexOf(':');
  return colon !== -1 && /^\d+$/.test(value.slice(colon + 1)) ? value.slice(0, colon) : value;
}

// Port 0 means "inherit the inbound port", so it leaves the addresses bare.
export function withHostPort(entries: string[], port: number): string[] {
  return entries.map((entry) => {
    const address = addressOf(entry);
    if (!port || port <= 0) return address;
    return address.includes(':') ? `[${address}]:${port}` : `${address}:${port}`;
  });
}
