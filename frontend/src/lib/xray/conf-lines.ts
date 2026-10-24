// One entry per .conf line, or '' when any entry would split: a line break can
// re-open [Interface], whose PostUp runs as root wherever wg-quick applies it.
export function joinConfLines(lines: readonly string[]): string {
  return lines.some(hasControlChar) ? '' : lines.join('\n');
}

function hasControlChar(line: string): boolean {
  for (let i = 0; i < line.length; i++) {
    const code = line.charCodeAt(i);
    if (code < 0x20 || code === 0x7f) return true;
  }
  return false;
}

// wg-quick matches keys case-insensitively; no panel builder emits these hooks.
const WG_QUICK_HOOK = /^\s*(?:pre|post)(?:up|down)\s*=/im;

export function hasWgQuickHook(conf: string): boolean {
  return WG_QUICK_HOOK.test(conf);
}
