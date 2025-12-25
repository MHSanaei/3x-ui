# AGENTS.md (xray/)

`xray/` wraps **Xray-core process management** (start/stop/restart, API traffic reads) and defines where runtime files live.

## Runtime file paths (via `config/`)

- **Binary**: `XUI_BIN_FOLDER/xray-<goos>-<goarch>`
  - Example on Linux amd64: `bin/xray-linux-amd64`
- **Config**: `XUI_BIN_FOLDER/config.json`
- **Geo files**: `XUI_BIN_FOLDER/{geoip.dat,geosite.dat,...}`
- **Logs**: `XUI_LOG_FOLDER/*` (default `/var/log` on non-Windows)

## Notes for changes

- Keep OS/arch naming consistent with `GetBinaryName()` (`xray-<GOOS>-<GOARCH>`).
- The web panel may attempt to restart Xray periodically; if the binary is missing, Xray-related operations will fail but the panel can still run.
- Be careful with process/exec changes: they are security- and stability-sensitive.


