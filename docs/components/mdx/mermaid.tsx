'use client';

import { useEffect, useId, useState } from 'react';
import { useTheme } from 'next-themes';

// Client-side, theme-aware Mermaid renderer. Mermaid is imported dynamically so
// it stays out of the initial bundle and only loads on pages that use a diagram.
export function Mermaid({ chart }: { chart: string }) {
  const rawId = useId();
  const id = `mmd-${rawId.replace(/[^a-zA-Z0-9]/g, '')}`;
  const { resolvedTheme } = useTheme();
  const [svg, setSvg] = useState('');

  useEffect(() => {
    let active = true;
    void (async () => {
      const mermaid = (await import('mermaid')).default;
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        theme: resolvedTheme === 'dark' ? 'dark' : 'default',
        fontFamily: 'inherit',
      });
      try {
        const { svg } = await mermaid.render(id, chart.trim());
        if (active) setSvg(svg);
      } catch {
        if (active) setSvg('');
      }
    })();
    return () => {
      active = false;
    };
  }, [chart, resolvedTheme, id]);

  return (
    <div
      className="my-6 flex justify-center overflow-x-auto rounded-xl border bg-fd-card p-4 [&_svg]:max-w-full"
      role="img"
      aria-label="Architecture diagram"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
