import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export function useServerDraft<T>(server: T | undefined, clone: (value: T) => T, equals: (left: T, right: T) => boolean) {
  const cloneRef = useRef(clone);
  const equalsRef = useRef(equals);
  cloneRef.current = clone;
  equalsRef.current = equals;

  const [draft, setDraft] = useState<T | undefined>();
  const baselineRef = useRef<T | undefined>(undefined);
  const serverRef = useRef(server);
  serverRef.current = server;

  useEffect(() => {
    if (server === undefined) return;
    setDraft((current) => {
      if (
        baselineRef.current !== undefined
        && current !== undefined
        && !equalsRef.current(current, baselineRef.current)
        && !equalsRef.current(current, server)
      ) {
        return current;
      }
      baselineRef.current = server;
      return cloneRef.current(server);
    });
  }, [server]);

  const discard = useCallback(() => {
    if (serverRef.current === undefined) return;
    baselineRef.current = serverRef.current;
    setDraft(cloneRef.current(serverRef.current));
  }, []);

  const isDirty = useMemo(
    () => draft !== undefined && baselineRef.current !== undefined && !equalsRef.current(draft, baselineRef.current),
    [draft],
  );

  return { draft, setDraft, isDirty, discard };
}
