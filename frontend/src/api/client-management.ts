export function clientManagementPath(action: 'get' | 'update' | 'del', email: string): string {
  const base = `/panel/api/clients/${action}`;
  const encoded = encodeURIComponent(email);
  return email === '.' || email === '..' ? `${base}?email=${encoded}` : `${base}/${encoded}`;
}
