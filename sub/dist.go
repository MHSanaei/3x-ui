package sub

import "embed"

// distFS holds the Vite-built frontend filesystem, injected from main at
// startup. The `web` package owns the //go:embed directive (because dist/
// is at web/dist/), and hands the FS over via SetDistFS so the sub package
// doesn't import web — that would create an import cycle once any
// web/controller handler reuses sub's link-building service.
var distFS embed.FS

// SetDistFS installs the embedded frontend filesystem the sub server uses
// for its info page assets. Must be called before NewServer().Start().
func SetDistFS(fs embed.FS) {
	distFS = fs
}
