// Pure builders for 3x-ui install commands (script + Docker). No React/DOM.

export type InstallMethod = 'script' | 'docker';

export interface InstallOptions {
  method: InstallMethod;
  /** A release tag like `v3.4.1`, or empty/`latest` for the latest release. */
  version: string;
  enableFail2ban: boolean;
  panelPort: string;
  webBasePath: string;
}

const REPO_RAW = 'https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh';
const IMAGE = 'ghcr.io/mhsanaei/3x-ui:latest';

function isLatest(version: string): boolean {
  const v = version.trim().toLowerCase();
  return v === '' || v === 'latest';
}

/**
 * The one-line script install command. The master install script reads the
 * version as its first argument: empty = latest stable release, a tag like
 * `v3.4.0` = that release, and `dev-latest` = the rolling per-commit dev build.
 */
export function buildScriptCommand(options: InstallOptions): string {
  if (isLatest(options.version)) {
    return `bash <(curl -Ls ${REPO_RAW})`;
  }
  return `bash <(curl -Ls ${REPO_RAW}) ${options.version.trim()}`;
}

/** A `docker run` command reflecting the chosen options. */
export function buildDockerRun(options: InstallOptions): string {
  const lines = ['docker run -itd'];
  lines.push(`  -e XRAY_VMESS_AEAD_FORCED=false`);
  lines.push(`  -e XUI_ENABLE_FAIL2BAN=${options.enableFail2ban ? 'true' : 'false'}`);
  if (options.panelPort.trim()) lines.push(`  -e XUI_PORT=${options.panelPort.trim()}`);
  if (options.webBasePath.trim())
    lines.push(`  -e XUI_INIT_WEB_BASE_PATH=${options.webBasePath.trim()}`);
  lines.push(`  -v $PWD/db/:/etc/x-ui/`);
  lines.push(`  -v $PWD/cert/:/root/cert/`);
  lines.push(`  --network=host`);
  lines.push(`  --restart=unless-stopped`);
  lines.push(`  --name 3x-ui`);
  lines.push(`  ${IMAGE}`);
  return lines.join(' \\\n');
}

/** A `docker-compose.yml` reflecting the chosen options. */
export function buildDockerCompose(options: InstallOptions): string {
  const env: string[] = [
    `      XRAY_VMESS_AEAD_FORCED: 'false'`,
    `      XUI_ENABLE_FAIL2BAN: '${options.enableFail2ban ? 'true' : 'false'}'`,
  ];
  if (options.panelPort.trim()) env.push(`      XUI_PORT: '${options.panelPort.trim()}'`);
  if (options.webBasePath.trim())
    env.push(`      XUI_INIT_WEB_BASE_PATH: '${options.webBasePath.trim()}'`);

  return [
    `services:`,
    `  3x-ui:`,
    `    image: ${IMAGE}`,
    `    container_name: 3x-ui`,
    `    volumes:`,
    `      - ./db/:/etc/x-ui/`,
    `      - ./cert/:/root/cert/`,
    `    environment:`,
    ...env,
    `    network_mode: host`,
    `    restart: unless-stopped`,
  ].join('\n');
}
