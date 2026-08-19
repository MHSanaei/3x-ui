import { useCallback, useEffect, useRef, useState } from 'react';

export function useServerDraft<T>(
  server: T | undefined,
  clone: (value: T) => T,
  equals: (left: T, right: T) => boolean,
) {
  const cloneRef = useRef(clone);
  useEffect(() => {
    cloneRef.current = clone;
  });

  const [draft, setDraft] = useState<T | undefined>();
  const [baseline, setBaseline] = useState<T | undefined>();
  const [syncedServer, setSyncedServer] = useState<T | undefined>();

  const isDirty = draft !== undefined && (baseline === undefined || !equals(draft, baseline));

  // Adopting the server value during render (not in an effect) keeps the
  // returned draft and isDirty consistent within the very first render.
  if (server !== syncedServer) {
    setSyncedServer(server);
    if (server !== undefined) {
      setBaseline(server);
      const keepLocalEdits = isDirty && !equals(draft as T, server);
      if (!keepLocalEdits) setDraft(clone(server));
    }
  }

  const markSaved = useCallback((value: T) => {
    setBaseline(cloneRef.current(value));
  }, []);

  return { draft, setDraft, isDirty, markSaved };
}
