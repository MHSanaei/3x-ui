'use client';

import { useState } from 'react';
import Link from 'next/link';
import { Sparkles } from 'lucide-react';
import {
  recommend,
  type UseCase,
  type CensorshipLevel,
  type ClientSupport,
} from '@/lib/xray/protocols';
import { ToolFrame } from './tool-frame';
import { SelectField } from './shared/fields';

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

export function ProtocolWizard() {
  const [useCase, setUseCase] = useState<UseCase>('general');
  const [censorship, setCensorship] = useState<CensorshipLevel>('medium');
  const [clientSupport, setClientSupport] = useState<ClientSupport>('modern');

  const result = recommend({ useCase, censorship, clientSupport });

  return (
    <ToolFrame
      title="Protocol wizard"
      description="Answer a few questions to get a recommended protocol and transport."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <SelectField
          label="Primary goal"
          value={cap(useCase)}
          onChange={(v) => setUseCase(v.toLowerCase() as UseCase)}
          options={['Censorship', 'General', 'Speed']}
        />
        <SelectField
          label="Censorship level"
          value={cap(censorship)}
          onChange={(v) => setCensorship(v.toLowerCase() as CensorshipLevel)}
          options={['High', 'Medium', 'Low']}
        />
        <SelectField
          label="Client support"
          value={cap(clientSupport)}
          onChange={(v) => setClientSupport(v.toLowerCase() as ClientSupport)}
          options={['Modern', 'Broad']}
        />
      </div>

      <div className="mt-4 rounded-xl border bg-fd-background p-4">
        <div className="flex items-center gap-2 text-brand">
          <Sparkles className="size-4" aria-hidden />
          <span className="text-sm font-medium">Recommended</span>
        </div>
        <div className="mt-2 flex flex-wrap gap-2">
          <Badge>{result.protocol}</Badge>
          <Badge>{result.transport}</Badge>
          <Badge>{result.security}</Badge>
        </div>
        <p className="mt-3 text-sm text-fd-muted-foreground">{result.rationale}</p>
        <div className="mt-3 flex flex-wrap gap-3 text-sm">
          {result.links.map((link) => (
            <Link key={link.href} href={link.href} className="text-fd-primary hover:underline">
              {link.title} →
            </Link>
          ))}
        </div>
      </div>
    </ToolFrame>
  );
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-lg bg-brand/10 px-2.5 py-1 text-sm font-medium text-brand">
      {children}
    </span>
  );
}
