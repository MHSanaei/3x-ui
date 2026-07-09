import { useEffect, useRef, useState } from 'react';
import { useFormContext, useWatch } from 'react-hook-form';

import { JsonEditor } from '@/components/form';
import {
  pruneEmpty,
  normalizeSniffing,
  normalizeClients,
  dropLegacyOptionalEmpties,
} from '@/lib/xray/inbound-form-adapter';

/*
 * Sub-editor for one slice of the form (settings, streamSettings, sniffing).
 * Holds a local text buffer so the user can type freely; on every keystroke
 * we try to JSON.parse and forward the result to form state. Invalid JSON
 * is held in the buffer until the next valid moment — no panic on partial
 * input. The buffer seeds once on mount; the modal's destroyOnHidden makes
 * each open a fresh editor instance, so we don't need to re-sync on outer
 * form changes.
 */
export function AdvancedSliceEditor({
  path,
  wrapKey,
  minHeight,
  maxHeight,
}: {
  path: string;
  /*
   * When set, the editor wraps the inner value with `{ [wrapKey]: ... }` so
   * the JSON the user sees matches the wire shape's slice envelope (e.g.
   * `{ "settings": { ... } }`). Edits unwrap the outer key before writing
   * back to the form. Mirrors the legacy modal's wrappedConfigValue.
   */
  wrapKey?: string;
  minHeight?: string;
  maxHeight?: string;
}) {
  const { control, getValues, setValue } = useFormContext();

  const serialize = (value: unknown): string => {
    const inner = value ?? {};
    return JSON.stringify(wrapKey ? { [wrapKey]: inner } : inner, null, 2);
  };

  const watched = useWatch({ control, name: path });
  const lastEmitRef = useRef<string>('');
  const [text, setText] = useState(() => {
    const initial = serialize(getValues(path));
    lastEmitRef.current = initial;
    return initial;
  });

  useEffect(() => {
    const formStr = serialize(watched);
    if (formStr === lastEmitRef.current) return;
    setText(formStr);
    lastEmitRef.current = formStr;
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [watched, wrapKey]);

  return (
    <JsonEditor
      value={text}
      minHeight={minHeight}
      maxHeight={maxHeight}
      onChange={(next) => {
        setText(next);
        try {
          const parsed = JSON.parse(next);
          const toWrite = wrapKey && parsed && typeof parsed === 'object' && !Array.isArray(parsed)
            ? (parsed as Record<string, unknown>)[wrapKey] ?? {}
            : parsed;
          setValue(path, toWrite);
          lastEmitRef.current = JSON.stringify(wrapKey ? { [wrapKey]: toWrite } : toWrite, null, 2);
        } catch {
          /* invalid JSON; keep buffer, don't push to form */
        }
      }}
    />
  );
}

/*
 * The "All" editor shows the full inbound JSON in one editor: top-level
 * connection fields plus the three nested sub-objects (settings,
 * streamSettings, sniffing). Edits round-trip back to the form's slices,
 * mirroring the legacy modal's setAdvancedAllValue behavior. Reactivity
 * works the same way as AdvancedSliceEditor: useWatch on the slices we
 * care about, lastEmitRef as the "we wrote this" guard.
 */
export function AdvancedAllEditor({
  streamEnabled,
  sniffingEnabled,
}: {
  streamEnabled: boolean;
  sniffingEnabled: boolean;
}) {
  const { control, setValue } = useFormContext();
  const wListen = useWatch({ control, name: 'listen' });
  const wPort = useWatch({ control, name: 'port' });
  const wProtocol = useWatch({ control, name: 'protocol' });
  const wTag = useWatch({ control, name: 'tag' });
  const wSettings = useWatch({ control, name: 'settings' });
  const wSniffing = useWatch({ control, name: 'sniffing' });
  const wStream = useWatch({ control, name: 'streamSettings' });

  const serialize = () => {
    /*
     * Apply the same prune/normalize as the wire payload so the JSON
     * shown here is what the panel actually POSTs (no empty defaults,
     * disabled sniffing as { enabled: false }, finalmask dropped when
     * there are no masks).
     */
    const settingsView = (pruneEmpty(wSettings ?? {}) ?? {}) as Record<string, unknown>;
    if (typeof wProtocol === 'string' && Array.isArray(settingsView.clients)) {
      settingsView.clients = normalizeClients(wProtocol, settingsView.clients);
    }
    const streamView = streamEnabled
      ? ((pruneEmpty(wStream ?? {}) ?? {}) as Record<string, unknown>)
      : undefined;
    dropLegacyOptionalEmpties(settingsView, streamView);
    const out: Record<string, unknown> = {
      listen: wListen ?? '',
      port: wPort ?? 0,
      protocol: wProtocol ?? '',
      tag: wTag ?? '',
      settings: settingsView,
    };
    if (sniffingEnabled) {
      out.sniffing = normalizeSniffing(wSniffing as Parameters<typeof normalizeSniffing>[0]);
    }
    if (streamView) out.streamSettings = streamView;
    return JSON.stringify(out, null, 2);
  };

  const lastEmitRef = useRef<string>('');
  const [text, setText] = useState(() => {
    const initial = serialize();
    lastEmitRef.current = initial;
    return initial;
  });

  useEffect(() => {
    const formStr = serialize();
    if (formStr === lastEmitRef.current) return;
    setText(formStr);
    lastEmitRef.current = formStr;
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [wListen, wPort, wProtocol, wTag, wSettings, wSniffing, wStream, streamEnabled, sniffingEnabled]);

  return (
    <JsonEditor
      value={text}
      minHeight="340px"
      maxHeight="560px"
      onChange={(next) => {
        setText(next);
        let parsed: Record<string, unknown>;
        try {
          parsed = JSON.parse(next) as Record<string, unknown>;
        } catch {
          return;
        }
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return;
        if (typeof parsed.listen === 'string') setValue('listen', parsed.listen);
        if (typeof parsed.port === 'number' && Number.isFinite(parsed.port)) {
          setValue('port', parsed.port);
        }
        if (typeof parsed.protocol === 'string') setValue('protocol', parsed.protocol);
        if (typeof parsed.tag === 'string') setValue('tag', parsed.tag);
        if (parsed.settings && typeof parsed.settings === 'object') {
          setValue('settings', parsed.settings);
        }
        if (sniffingEnabled && parsed.sniffing && typeof parsed.sniffing === 'object') {
          setValue('sniffing', parsed.sniffing);
        }
        if (streamEnabled && parsed.streamSettings && typeof parsed.streamSettings === 'object') {
          setValue('streamSettings', parsed.streamSettings);
        }
        lastEmitRef.current = next;
      }}
    />
  );
}
