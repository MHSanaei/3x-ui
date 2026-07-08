import { beforeEach, describe, expect, it, vi } from 'vitest';

const toast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  loading: vi.fn(),
}));

vi.mock('@/api/http-init', () => ({
  httpRequest: vi.fn(),
  HttpError: class HttpError extends Error {
    status: number;
    response: { status: number; statusText: string; data: unknown };
    constructor(status: number, statusText: string, data: unknown) {
      super(`Request failed with status ${status}`);
      this.status = status;
      this.response = { status, statusText, data };
    }
  },
}));

vi.mock('@/utils/messageBus', () => ({
  getMessage: () => toast,
}));

import { HttpUtil } from '@/utils';
import { HttpError, httpRequest } from '@/api/http-init';
import type { HttpResponse } from '@/api/http-init';

const mockRequest = vi.mocked(httpRequest);
const envelope = (data: unknown): HttpResponse => ({ ok: true, status: 200, statusText: 'OK', data });

describe('HttpUtil', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('unwraps a success envelope and shows a success toast', async () => {
    mockRequest.mockResolvedValue(envelope({ success: true, msg: 'done', obj: { id: 1 } }));

    const msg = await HttpUtil.post('/x', { a: 1 });

    expect(msg.success).toBe(true);
    expect(msg.obj).toEqual({ id: 1 });
    expect(toast.success).toHaveBeenCalledWith('done');
  });

  it('suppresses the success toast with silentSuccess but still warns on nodePending', async () => {
    mockRequest.mockResolvedValue(envelope({ success: true, msg: 'saved', obj: { nodePending: true } }));

    await HttpUtil.post('/x', { a: 1 }, { silentSuccess: true });

    expect(toast.success).not.toHaveBeenCalled();
    expect(toast.warning).toHaveBeenCalled();
  });

  it('shows an error toast for a failure envelope', async () => {
    mockRequest.mockResolvedValue(envelope({ success: false, msg: 'nope', obj: null }));

    const msg = await HttpUtil.post('/x');

    expect(msg.success).toBe(false);
    expect(toast.error).toHaveBeenCalledWith('nope');
  });

  it('suppresses all toasts with silent', async () => {
    mockRequest.mockResolvedValue(envelope({ success: false, msg: 'nope', obj: null }));

    await HttpUtil.post('/x', undefined, { silent: true });

    expect(toast.error).not.toHaveBeenCalled();
  });

  it('maps a thrown HttpError to a failure Msg via response.data.message', async () => {
    mockRequest.mockRejectedValue(new HttpError(400, 'Bad Request', { message: 'bad input' }));

    const msg = await HttpUtil.post('/x', undefined, { silent: true });

    expect(msg.success).toBe(false);
    expect(msg.msg).toBe('bad input');
  });

  it('maps a thrown native error to a failure Msg via its message', async () => {
    mockRequest.mockRejectedValue(new Error('Network down'));

    const msg = await HttpUtil.get('/x', undefined, { silent: true });

    expect(msg.msg).toBe('Network down');
  });

  it('returns "No response data" for an empty body', async () => {
    mockRequest.mockResolvedValue(envelope(''));

    const msg = await HttpUtil.get('/x', undefined, { silent: true });

    expect(msg.success).toBe(false);
    expect(msg.msg).toBe('No response data');
  });
});
