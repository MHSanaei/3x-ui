#!/usr/bin/env bash
#
# build.sh — cross-platform build helper for 3x-ui
# ----------------------------------------------------------------------------
# 3x-ui needs CGO for the SQLite driver (github.com/mattn/go-sqlite3), so a C
# toolchain for the *target* platform must be available. This script handles
# that for you:
#
#   * Linux targets  -> built inside Docker (clean musl/CGO toolchain, also
#                       bundles the matching xray binary + geo data; matches
#                       the production image).
#   * Host target    -> built natively (needs a local C compiler, e.g. gcc).
#   * CGO=0 builds    -> pure-Go cross-compile to any OS/arch with no toolchain,
#                       but support Postgres only (no SQLite) and do not bundle
#                       xray. Handy for quick Windows/macOS/Linux binaries.
#
# Usage:
#   ./build.sh frontend                 Build the web UI -> web/dist
#   ./build.sh native                   Build for the host OS/arch (CGO)
#   ./build.sh linux-amd64              Linux x86-64 binary  (Docker, full)
#   ./build.sh linux-arm64              Linux arm64  binary  (Docker, full)
#   ./build.sh windows-amd64            Windows x86-64 .exe
#   ./build.sh darwin-arm64             macOS Apple-Silicon binary
#   ./build.sh all                      frontend + linux-amd64 + linux-arm64
#   ./build.sh clean                    Remove build/ and vendor/
#   ./build.sh native linux-amd64 ...   Run several targets in one go
#
# Env vars:
#   CGO=0                Pure-Go build (Postgres-only, no SQLite, no Docker,
#                        no C toolchain). Works for any target.
#   VENDOR=auto|on|off   Vendor modules before Docker builds so the in-container
#                        build is offline (also dodges flaky module-proxy zips).
#                        Default: auto (on when a host `go` toolchain exists).
#   SKIP_FRONTEND=1      Don't (re)build web/dist for native / pure-Go targets.
#   LDFLAGS="..."        Extra Go linker flags. Default: "-w -s".
#   CC="..."             Override the C compiler for CGO cross-compiles.
# ----------------------------------------------------------------------------
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

OUT="build"
LDFLAGS="${LDFLAGS:--w -s}"
CGO="${CGO:-1}"
IMAGE_PREFIX="3x-ui-builder"
VENDORED_BY_US=0

log()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m warn:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Frontend (Vite -> web/dist). Required before any CGO/native Go build because
# the binary embeds web/dist via go:embed. The Docker path builds it itself.
# ---------------------------------------------------------------------------
build_frontend() {
  command -v npm >/dev/null 2>&1 || die "npm not found — install Node.js 20+."
  log "Building web UI -> web/dist"
  ( cd frontend && { [ -d node_modules ] || npm ci; } && npm run build )
}

ensure_frontend() { [ "${SKIP_FRONTEND:-0}" = 1 ] || build_frontend; }

# ---------------------------------------------------------------------------
# Generic Go build for a single target. ext is the output suffix (".exe" / "").
# cc (optional) is the C cross-compiler used only when CGO=1.
# ---------------------------------------------------------------------------
go_build() {
  local goos="$1" goarch="$2" ext="$3" cc="${4:-}"
  mkdir -p "$OUT"
  local out="$OUT/x-ui-$goos-$goarch$ext"
  if [ "$CGO" = 1 ]; then
    [ -n "$cc" ] || cc="${CC:-gcc}"
    command -v "$cc" >/dev/null 2>&1 || die \
      "CGO build for $goos/$goarch needs C compiler '$cc'. Install it, use the Docker path (for Linux), or set CGO=0 for a pure-Go Postgres-only build."
    log "Building $goos/$goarch (CGO=1, CC=$cc) -> $out"
    CGO_ENABLED=1 CC="$cc" GOOS="$goos" GOARCH="$goarch" \
      CGO_CFLAGS="-D_LARGEFILE64_SOURCE" \
      go build -ldflags "$LDFLAGS" -o "$out" main.go
  else
    log "Building $goos/$goarch (CGO=0, pure-Go) -> $out"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -ldflags "$LDFLAGS" -o "$out" main.go
    warn "CGO=0: SQLite is unavailable — run with XUI_DB_TYPE=postgres. Xray binary is NOT bundled."
  fi
  log "done: $out"
}

build_native() {
  ensure_frontend
  go_build "$(go env GOOS)" "$(go env GOARCH)" "$([ "$(go env GOOS)" = windows ] && echo .exe || echo)"
}

# ---------------------------------------------------------------------------
# Vendor modules so the Docker build needs no network (and won't trip over a
# flaky module proxy). Only vendors if we created it; cleaned up on exit.
# ---------------------------------------------------------------------------
maybe_vendor() {
  local mode="${VENDOR:-auto}"
  [ "$mode" = off ] && return 0
  [ "$mode" = auto ] && ! command -v go >/dev/null 2>&1 && return 0
  [ -d vendor ] && return 0   # respect a pre-existing vendor tree, leave as-is
  command -v go >/dev/null 2>&1 || return 0
  log "Vendoring Go modules (offline Docker build)"
  go mod vendor && VENDORED_BY_US=1
}
cleanup_vendor() { [ "$VENDORED_BY_US" = 1 ] && rm -rf vendor && log "removed transient vendor/"; return 0; }
trap cleanup_vendor EXIT

# ---------------------------------------------------------------------------
# Linux build. CGO=1 -> Docker (full SQLite build + bundled xray/geo).
#               CGO=0 -> direct pure-Go cross-compile (Postgres-only).
# ---------------------------------------------------------------------------
build_linux() {
  local arch="$1"
  if [ "$CGO" = 0 ]; then ensure_frontend; go_build linux "$arch" ""; return; fi

  command -v docker >/dev/null 2>&1 || die "docker not found (needed for the full Linux build). Or set CGO=0 for a pure-Go Postgres-only binary."
  docker buildx version >/dev/null 2>&1 || die "docker buildx not available."
  docker info >/dev/null 2>&1 || die "Docker daemon is not running — start Docker Desktop / the engine."

  maybe_vendor
  local img="$IMAGE_PREFIX:linux-$arch"
  log "Building Linux/$arch via Docker (CGO, musl) — this compiles xray-core, be patient"
  docker buildx build --platform "linux/$arch" --target builder \
    --provenance=false --sbom=false -t "$img" --load .

  log "Extracting binary + xray/geo assets"
  mkdir -p "$OUT"
  local cid; cid="$(docker create --platform "linux/$arch" "$img")"
  docker cp "$cid:/app/build/x-ui" "$OUT/x-ui-linux-$arch"
  rm -rf "$OUT/bin-linux-$arch"
  docker cp "$cid:/app/build/bin" "$OUT/bin-linux-$arch"
  docker rm "$cid" >/dev/null
  log "done: $OUT/x-ui-linux-$arch  (+ $OUT/bin-linux-$arch/ : xray + geo data)"
}

usage() { sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//; /^set -euo/d'; }

run_target() {
  case "$1" in
    frontend)      build_frontend ;;
    native)        build_native ;;
    linux-amd64)   build_linux amd64 ;;
    linux-arm64)   build_linux arm64 ;;
    windows-amd64) ensure_frontend; go_build windows amd64 .exe "${CC:-x86_64-w64-mingw32-gcc}" ;;
    windows-arm64) ensure_frontend; go_build windows arm64 .exe "${CC:-aarch64-w64-mingw32-gcc}" ;;
    darwin-amd64)  ensure_frontend; go_build darwin amd64 "" "${CC:-o64-clang}" ;;
    darwin-arm64)  ensure_frontend; go_build darwin arm64 "" "${CC:-oa64-clang}" ;;
    all)           build_frontend; build_linux amd64; build_linux arm64 ;;
    clean)         rm -rf "$OUT" vendor; VENDORED_BY_US=0; log "cleaned build/ and vendor/" ;;
    -h|--help|help) usage ;;
    *)             die "unknown target '$1' (run: ./build.sh --help)" ;;
  esac
}

[ $# -gt 0 ] || { usage; exit 1; }
for target in "$@"; do run_target "$target"; done
