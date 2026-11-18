package offline

import (
	"archive/zip"
	"bufio"
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

const (
	MaxPackageSize  = 256 << 20
	maxExpandedSize = 512 << 20
	Format          = "3x-ui-local-package:v1"
)

var geoFiles = []string{"geoip.dat", "geosite.dat", "geoip_IR.dat", "geosite_IR.dat", "geoip_RU.dat", "geosite_RU.dat"}

// Extract validates the archive and stages only the supported replacement files.
func Extract(archive, stage string) ([]string, error) {
	info, err := os.Stat(archive)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxPackageSize {
		return nil, errors.New("update package exceeds 256 MiB")
	}
	z, err := zip.OpenReader(archive)
	if err != nil {
		return nil, fmt.Errorf("open update ZIP: %w", err)
	}
	defer z.Close()
	if len(z.File) < 2 || len(z.File) > len(geoFiles)+3 {
		return nil, errors.New("invalid number of entries in update ZIP")
	}

	seen := make(map[string]bool)
	var files []string
	var total uint64
	for _, entry := range z.File {
		name := entry.Name
		if seen[name] || !entry.Mode().IsRegular() || entry.UncompressedSize64 == 0 ||
			(name != "format" && name != "x-ui" && name != "xray" && !slices.Contains(geoFiles, name)) {
			return nil, fmt.Errorf("unexpected update ZIP entry %q", name)
		}
		seen[name] = true
		if entry.UncompressedSize64 > maxExpandedSize-total {
			return nil, errors.New("expanded update exceeds 512 MiB")
		}
		total += entry.UncompressedSize64
		if name == "format" {
			if entry.UncompressedSize64 != uint64(len(Format)+1) {
				return nil, errors.New("invalid update package format")
			}
			data, err := readEntry(entry)
			if err != nil || string(data) != Format+"\n" {
				return nil, errors.New("invalid update package format")
			}
			continue
		}
		dest := filepath.Join(stage, name)
		if err := copyEntry(entry, dest); err != nil {
			return nil, err
		}
		if name == "x-ui" || name == "xray" {
			if err := checkExecutable(dest, name == "x-ui"); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		}
		files = append(files, name)
	}
	if !seen["format"] {
		return nil, errors.New("update ZIP is missing its format identifier")
	}
	return files, nil
}

func readEntry(entry *zip.File) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
}

func copyEntry(entry *zip.File, dest string) error {
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
	return errors.Join(copyErr, out.Close())
}

func checkExecutable(path string, panel bool) error {
	file, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("expected a Linux ELF executable: %w", err)
	}
	defer file.Close()
	machines := map[string]elf.Machine{
		"386": elf.EM_386, "amd64": elf.EM_X86_64, "arm": elf.EM_ARM,
		"arm64": elf.EM_AARCH64, "s390x": elf.EM_S390, "riscv64": elf.EM_RISCV,
	}
	machine, supported := machines[runtime.GOARCH]
	if !supported || machine != file.Machine || (file.Type != elf.ET_EXEC && file.Type != elf.ET_DYN) {
		return errors.New("executable does not match this CPU architecture")
	}
	if panel {
		// The format string is compiled into the local panel's read-only data.
		// Only trusted uploads can supply code. This marker prevents accidental
		// replacement with an upstream build and is not a signature.
		section := file.Section(".rodata")
		if section == nil || !containsMarker(section.Open()) {
			return errors.New("panel binary is missing the offline build marker")
		}
	}
	return nil
}

// Verify checks an update ZIP without touching the installed application.
func Verify(path string) error {
	stage, err := os.MkdirTemp("", "x-ui-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	_, err = Extract(path, stage)
	return err
}

func containsMarker(r io.Reader) bool {
	const windowSize = 64 << 10
	reader := bufio.NewReaderSize(r, windowSize)
	for {
		window, err := reader.Peek(windowSize)
		if bytes.Contains(window, []byte(Format)) {
			return true
		}
		if err != nil {
			return false
		}
		_, _ = reader.Discard(windowSize - len(Format) + 1)
	}
}
