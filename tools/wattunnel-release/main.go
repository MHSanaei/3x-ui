package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/wattunnel"
)

func main() {
	target := flag.String("target", "", "linux-amd64, linux-arm64, or windows-amd64")
	out := flag.String("out", "", "binary output directory")
	flag.Parse()
	if err := run(*target, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(target, out string) error {
	if target != "linux-amd64" && target != "linux-arm64" && target != "windows-amd64" {
		return errors.New("unsupported WatTunnel release target")
	}
	if out == "" {
		return errors.New("output directory required")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(out, ".wattunnel-download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	name := "wattun-" + target
	if target == "windows-amd64" {
		name += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Minute}
	for _, asset := range []string{name, "SHA256SUMS", "SHA256SUMS.sig"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/JohnWatsoon/WatTunnel/releases/download/v"+wattunnel.Version+"/"+asset, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("download %s: HTTP %d", asset, resp.StatusCode)
		}
		f, err := os.OpenFile(filepath.Join(dir, asset), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			resp.Body.Close()
			return err
		}
		n, copyErr := io.Copy(f, io.LimitReader(resp.Body, (128<<20)+1))
		closeErr := f.Close()
		resp.Body.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if n > 128<<20 {
			return errors.New("release asset too large")
		}
	}
	if err = wattunnel.VerifyArtifacts(filepath.Join(dir, name), filepath.Join(dir, "SHA256SUMS"), filepath.Join(dir, "SHA256SUMS.sig")); err != nil {
		return err
	}
	if err = os.Chmod(filepath.Join(dir, name), 0o755); err != nil {
		return err
	}
	for _, asset := range []string{"SHA256SUMS", "SHA256SUMS.sig", name} {
		dest := asset
		if asset != name {
			if err = os.Chmod(filepath.Join(dir, asset), 0o644); err != nil {
				return err
			}
			dest = "wattunnel-" + asset
		}
		if err = os.Rename(filepath.Join(dir, asset), filepath.Join(out, dest)); err != nil {
			return err
		}
	}
	if err = os.WriteFile(filepath.Join(out, "wattunnel-LICENSE"), wattunnel.License, 0o644); err != nil {
		return err
	}
	fmt.Println("verified WatTunnel", wattunnel.Version, target)
	return nil
}
