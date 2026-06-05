import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';

export interface MeInfo {
  id: number;
  username: string;
  email: string;
  role: string;
  isAdmin: boolean;
  balance: number;
  clientCost: number;
  clientCostPerGB: number;
  zarinpalEnable: boolean;
  currency: string;
}

export const ME_QUERY_KEY = ['me'] as const;

async function fetchMe(): Promise<MeInfo> {
  const msg = await HttpUtil.get('/panel/api/me', undefined, { silent: true });
  if (!msg?.success || !msg.obj) {
    throw new Error(msg?.msg || 'Failed to load profile');
  }
  const o = msg.obj as Partial<MeInfo>;
  return {
    id: Number(o.id) || 0,
    username: String(o.username ?? ''),
    email: String(o.email ?? ''),
    role: String(o.role ?? 'user'),
    isAdmin: Boolean(o.isAdmin),
    balance: Number(o.balance) || 0,
    clientCost: Number(o.clientCost) || 0,
    clientCostPerGB: Number(o.clientCostPerGB) || 0,
    zarinpalEnable: Boolean(o.zarinpalEnable),
    currency: String(o.currency ?? 'IRT'),
  };
}

/**
 * useMe loads the current session's identity, role, wallet balance and the
 * per-client cost. It is the single source of truth the SPA uses to gate
 * navigation, hide admin UI and preview purchases. The backend independently
 * enforces every one of these — the hook only drives presentation.
 */
export function useMe() {
  const query = useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: fetchMe,
    staleTime: 15_000,
  });
  return {
    me: query.data,
    isAdmin: query.data?.isAdmin,
    balance: query.data?.balance ?? 0,
    clientCost: query.data?.clientCost ?? 0,
    loading: query.isLoading,
    refetch: query.refetch,
  };
}
