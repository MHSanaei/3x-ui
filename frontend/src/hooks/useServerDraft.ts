import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export function useServerDraft<T>(server: T | undefined, clone: (value: T) => T, equals: (left: T, right: T) => boolean) {
  const cloneRef = useRef(clone);
  const equalsRef = useRef(equals);
  cloneRef.current = clone;
  equalsRef.current = equals;

  const [draft, setDraft] = useState<T | undefined>();
  const [baseline, setBaseline] = useState<T | undefined>();
  const draftRef = useRef(draft);
  const baselineRef = useRef(baseline);
  const serverRef = useRef(server);
  draftRef.current = draft;
  baselineRef.current = baseline;
  serverRef.current = server;

  useEffect(() => {
    if (server === undefined) return;
    const currentDraft = draftRef.current;
    const currentBaseline = baselineRef.current;
    const isDirty = currentDraft !== undefined
      && currentBaseline !== undefined
      && !equalsRef.current(currentDraft, currentBaseline);
    setBaseline(server);
    if (isDirty && !equalsRef.current(currentDraft, server)) return;
    setDraft(cloneRef.current(server));
  }, [server]);

  const discard = useCallback(() => {
    if (serverRef.current === undefined) return;
    setBaseline(serverRef.current);
    setDraft(cloneRef.current(serverRef.current));
  }, []);

  const isDirty = useMemo(
    () => draft !== undefined && baseline !== undefined && !equalsRef.current(draft, baseline),
    [baseline, draft],
  );

  return { draft, setDraft, isDirty, discard };
}
