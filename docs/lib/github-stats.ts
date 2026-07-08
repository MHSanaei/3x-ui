import { productRepo } from './shared';

export interface GitHubStats {
  stars: number;
  forks: number;
  latestVersion: string;
}

// Real, recent numbers used as a fallback when the GitHub API is unavailable
// at build time (offline CI, rate limit). Update periodically.
const FALLBACK: GitHubStats = {
  stars: 41500,
  forks: 7700,
  latestVersion: 'v3.x',
};

const API_BASE = `https://api.github.com/repos/${productRepo.user}/${productRepo.repo}`;

/**
 * Fetch live repo stats. Runs both at build time (Node) and in the browser —
 * api.github.com sends CORS headers, and the unauthenticated limit (60 req/h
 * per client IP) is plenty for one call per landing-page visit.
 * Returns null on any failure so callers keep the numbers they already have;
 * `latestVersion` is '' when the release lookup alone fails.
 */
export async function fetchGitHubStats(init?: RequestInit): Promise<GitHubStats | null> {
  try {
    const [repoRes, releaseRes] = await Promise.all([
      fetch(API_BASE, init),
      fetch(`${API_BASE}/releases/latest`, init),
    ]);

    if (!repoRes.ok) return null;
    const repo = (await repoRes.json()) as { stargazers_count?: number; forks_count?: number };
    if (typeof repo.stargazers_count !== 'number' || typeof repo.forks_count !== 'number') {
      return null;
    }

    let latestVersion = '';
    if (releaseRes.ok) {
      const release = (await releaseRes.json()) as { tag_name?: string };
      if (release.tag_name) latestVersion = release.tag_name;
    }

    return { stars: repo.stargazers_count, forks: repo.forks_count, latestVersion };
  } catch {
    return null;
  }
}

/**
 * Build-time stats used as the initial render (no layout shift, works without
 * JS). The client refreshes them via fetchGitHubStats() after hydration.
 * Always resolves; on any error it returns the hardcoded fallback.
 */
export async function getGitHubStats(): Promise<GitHubStats> {
  const headers: Record<string, string> = {
    'User-Agent': '3x-ui-docs',
    Accept: 'application/vnd.github+json',
  };
  if (process.env.GITHUB_TOKEN) headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;

  const live = await fetchGitHubStats({ headers, next: { revalidate: 3600 } });
  if (!live) return FALLBACK;
  return { ...live, latestVersion: live.latestVersion || FALLBACK.latestVersion };
}

/** Compact display, e.g. 41523 -> "41.5k". */
export function formatCount(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}
