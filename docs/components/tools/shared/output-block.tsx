'use client';

import QRCode from 'react-qr-code';
import { CopyButton } from './copy-button';

export function OutputBlock({
  label,
  value,
  qr = false,
}: {
  label: string;
  value: string;
  qr?: boolean;
}) {
  return (
    <div className="overflow-hidden rounded-xl border">
      <div className="flex items-center justify-between gap-2 border-b bg-fd-muted/40 px-3 py-2">
        <span className="text-xs font-medium text-fd-muted-foreground">{label}</span>
        <CopyButton value={value} />
      </div>
      <pre dir="ltr" className="max-h-80 overflow-auto p-3 text-start text-xs leading-relaxed">
        <code>{value}</code>
      </pre>
      {qr && value ? (
        <div className="flex justify-center border-t bg-white p-4">
          <QRCode value={value} size={180} />
        </div>
      ) : null}
    </div>
  );
}
