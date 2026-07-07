'use client';

import { useEffect, useState } from 'react';
import { GitFork, Star, Tag } from 'lucide-react';
import { fetchGitHubStats, formatCount, type GitHubStats } from '@/lib/github-stats';

/**
 * Stars / forks / latest-release row. Renders the build-time numbers
 * immediately (no layout shift, works without JS), then swaps in live ones
 * from the GitHub API after hydration.
 */
export function GitHubStatsRow({
  initial,
  labels,
}: {
  initial: GitHubStats;
  labels: { stars: string; forks: string; latest: string };
}) {
  const [stats, setStats] = useState(initial);

  useEffect(() => {
    let cancelled = false;
    // Plain fetch, no custom headers — keeps the request preflight-free.
    void fetchGitHubStats().then((live) => {
      if (cancelled || !live) return;
      setStats((prev) => ({ ...live, latestVersion: live.latestVersion || prev.latestVersion }));
    });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <dl className="mt-8 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-sm">
      <Stat icon={<Star className="size-4" aria-hidden />} label={labels.stars}>
        {formatCount(stats.stars)}
      </Stat>
      <Stat icon={<GitFork className="size-4" aria-hidden />} label={labels.forks}>
        {formatCount(stats.forks)}
      </Stat>
      <Stat icon={<Tag className="size-4" aria-hidden />} label={labels.latest}>
        {stats.latestVersion}
      </Stat>
    </dl>
  );
}

function Stat({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="inline-flex items-center gap-2">
      <span className="text-brand">{icon}</span>
      <dt className="sr-only">{label}</dt>
      <dd>
        <span className="font-semibold">{children}</span>{' '}
        <span className="text-fd-muted-foreground">{label}</span>
      </dd>
    </div>
  );
}
