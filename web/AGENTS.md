# AGENTS.md (web/)

`web/` is the **Go web panel** (Gin) and includes **embedded** templates/assets for production.

## Templates & assets

- **Templates**: `web/html/**` (server-rendered HTML templates containing Vue/Ant Design UI markup).
- **Static assets**: `web/assets/**` (vendored JS/CSS/libs).
- **Embedding**:
  - Production uses `//go:embed` for `assets`, `html/*`, and `translation/*` (see `web/web.go`).
  - Dev mode (`XUI_DEBUG=true`) loads templates from disk (`web/html`) and serves assets from disk (`web/assets`).

## i18n / translations

- Translation files live in `web/translation/*.toml` and are embedded.
- When adding UI strings, update the relevant TOML(s) and keep keys consistent across languages.

## Common dev pitfalls

- Run from repo root when `XUI_DEBUG=true` so `web/html` and `web/assets` resolve correctly.
- Some functionality depends on an Xray binary in `XUI_BIN_FOLDER` (default `bin/`); the panel can run without Xray but Xray-related features will fail until it’s available.


