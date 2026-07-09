import { http, HttpResponse } from 'msw';

export const CSRF_TOKEN = 'test-csrf-token';

export const handlers = [
  http.get('*/csrf-token', () => HttpResponse.json({ success: true, obj: CSRF_TOKEN })),
];
