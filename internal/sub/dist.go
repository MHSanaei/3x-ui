package sub

import "io/fs"

// distFS holds the Vite-built frontend filesystem, injected from main at
// startup. The `web` package owns the //go:embed directive (because dist/
// is at internal/web/dist/), and hands the FS over via SetDistFS so the sub package
// doesn't import web — that would create an import cycle once any
// internal/web/controller handler reuses sub's link-building service.
var distFS fs.FS

// SetDistFS installs the embedded frontend filesystem the sub server uses
// for its info page assets. Must be called before NewServer().Start().
func SetDistFS(frontendFS fs.FS) {
	distFS = frontendFS
}
