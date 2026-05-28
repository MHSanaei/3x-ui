import { lazy, useEffect, useMemo, useState } from 'react';

import { HttpUtil } from '@/utils';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';

const BulkAddToGroupModal = lazy(() => import('@/pages/clients/BulkAddToGroupModal'));

interface AddClientsToGroupModalProps {
  open: boolean;
  source: DBInbound | null;
  onClose: () => void;
  onAdded?: () => void;
}

function readClientEmails(settings: unknown): string[] {
  const parsed = coerceInboundJsonField(settings) as { clients?: Array<{ email?: string }> };
  const clients = Array.isArray(parsed?.clients) ? parsed.clients : [];
  return clients.map((c) => (c?.email || '').trim()).filter(Boolean);
}

export default function AddClientsToGroupModal({
  open,
  source,
  onClose,
  onAdded,
}: AddClientsToGroupModalProps) {
  const [groups, setGroups] = useState<string[]>([]);

  const emails = useMemo(() => (source ? readClientEmails(source.settings) : []), [source]);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.get('/panel/api/clients/groups', undefined, { silent: true });
      if (cancelled) return;
      const list = Array.isArray(msg?.obj) ? (msg.obj as Array<{ name?: string }>) : [];
      setGroups(list.map((g) => g?.name || '').filter(Boolean));
    })();
    return () => { cancelled = true; };
  }, [open]);

  return (
    <BulkAddToGroupModal
      open={open}
      count={emails.length}
      groups={groups}
      onOpenChange={(o) => { if (!o) onClose(); }}
      onSubmit={async (group) => {
        const msg = await HttpUtil.post(
          '/panel/api/clients/groups/bulkAdd',
          { emails, group },
          { headers: { 'Content-Type': 'application/json' } },
        );
        if (!msg?.success) return null;
        onAdded?.();
        return (msg.obj as { affected?: number } | undefined) ?? { affected: 0 };
      }}
    />
  );
}
