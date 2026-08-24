package frontproxy

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Limits on an uploaded decoy archive. A decoy is a small static site, so
// these are generous while still bounding a zip bomb.
const (
	maxDecoyBytes   = 64 << 20
	maxDecoyEntries = 2000
)

// DecoyInstalled reports whether an uploaded site is present and servable,
// which is what the settings UI shows next to the upload button.
func DecoyInstalled(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && info.Mode().IsRegular()
}

// InstallDecoyArchive unpacks a zip into dir, replacing whatever was there.
// It stages into a sibling directory so a bad upload leaves the old site up.
func InstallDecoyArchive(dir string, r io.ReaderAt, size int64) error {
	if dir == "" {
		return fmt.Errorf("no decoy directory configured")
	}
	if size > maxDecoyBytes {
		return fmt.Errorf("archive is larger than the %d MiB limit", maxDecoyBytes>>20)
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("not a readable zip archive: %w", err)
	}
	if len(zr.File) > maxDecoyEntries {
		return fmt.Errorf("archive has %d entries, more than the %d allowed", len(zr.File), maxDecoyEntries)
	}
	prefix, err := decoyRootPrefix(zr.File)
	if err != nil {
		return err
	}

	staging := dir + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("cannot clear staging directory: %w", err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("cannot create staging directory: %w", err)
	}
	if err := extractDecoy(zr, staging, prefix); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	return swapDecoyDir(dir, staging)
}

// decoyRootPrefix locates index.html, tolerating the common case of a zip
// that wraps the whole site in one folder.
func decoyRootPrefix(files []*zip.File) (string, error) {
	tops := map[string]bool{}
	for _, f := range files {
		name, err := decoyEntryName(f)
		if err != nil {
			return "", err
		}
		if name == "" {
			continue
		}
		if name == "index.html" {
			return "", nil
		}
		if i := strings.Index(name, "/"); i > 0 {
			tops[name[:i]] = true
		}
	}
	if len(tops) == 1 {
		for top := range tops {
			for _, f := range files {
				if name, _ := decoyEntryName(f); name == top+"/index.html" {
					return top + "/", nil
				}
			}
		}
	}
	return "", fmt.Errorf("archive has no index.html at its root")
}

// decoyEntryName normalizes one archive path and refuses anything that could
// escape the destination directory once joined to it.
func decoyEntryName(f *zip.File) (string, error) {
	name := strings.ReplaceAll(f.Name, `\`, "/")
	if strings.HasPrefix(name, "/") || filepath.VolumeName(name) != "" {
		return "", fmt.Errorf("archive entry %q has an absolute path", f.Name)
	}
	clean := path.Clean(name)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive entry %q points outside the archive", f.Name)
	}
	if strings.HasSuffix(name, "/") {
		return clean + "/", nil
	}
	return clean, nil
}

// extractDecoy writes every regular file under prefix into dst, stopping if
// the archive expands past the size cap.
func extractDecoy(zr *zip.Reader, dst, prefix string) error {
	var written int64
	for _, f := range zr.File {
		name, err := decoyEntryName(f)
		if err != nil {
			return err
		}
		if name == "" || !strings.HasPrefix(name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		// Symlinks and devices have no place in a static site, and following
		// one during extraction is exactly how a zip escapes its directory.
		if !f.Mode().IsRegular() {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("cannot create %s: %w", filepath.Dir(rel), err)
		}
		n, err := copyDecoyFile(target, f, maxDecoyBytes-written)
		if err != nil {
			return err
		}
		written += n
	}
	if written == 0 {
		return fmt.Errorf("archive contains no files")
	}
	return nil
}

// copyDecoyFile writes one archive entry, refusing to write more than the
// budget left so a lying uncompressed size cannot blow past the cap.
func copyDecoyFile(target string, f *zip.File, budget int64) (int64, error) {
	if budget <= 0 {
		return 0, fmt.Errorf("archive expands past the %d MiB limit", maxDecoyBytes>>20)
	}
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("cannot read %s from archive: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("cannot write %s: %w", f.Name, err)
	}
	n, err := io.Copy(out, io.LimitReader(rc, budget+1))
	closeErr := out.Close()
	if err != nil {
		return 0, fmt.Errorf("cannot write %s: %w", f.Name, err)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("cannot write %s: %w", f.Name, closeErr)
	}
	if n > budget {
		return 0, fmt.Errorf("archive expands past the %d MiB limit", maxDecoyBytes>>20)
	}
	return n, nil
}

// swapDecoyDir moves staging into place, keeping the previous site until the
// new one is live so a failure mid-swap never leaves no decoy at all.
func swapDecoyDir(dir, staging string) error {
	old := dir + ".old"
	if err := os.RemoveAll(old); err != nil {
		return fmt.Errorf("cannot clear the previous decoy: %w", err)
	}
	if _, err := os.Stat(dir); err == nil {
		if err := os.Rename(dir, old); err != nil {
			return fmt.Errorf("cannot move the previous decoy aside: %w", err)
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		if _, statErr := os.Stat(old); statErr == nil {
			_ = os.Rename(old, dir)
		}
		return fmt.Errorf("cannot put the new decoy in place: %w", err)
	}
	_ = os.RemoveAll(old)
	return nil
}

// RemoveDecoy deletes the uploaded site, returning the front door to whatever
// template mode would show.
func RemoveDecoy(dir string) error {
	if dir == "" {
		return fmt.Errorf("no decoy directory configured")
	}
	return os.RemoveAll(dir)
}
