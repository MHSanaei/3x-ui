package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "repository root containing database/model and web/entity")
	outDir := flag.String("out", "frontend/src/generated", "output directory relative to root")
	flag.Parse()

	if err := run(*root, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}

func run(root, outDir string) error {
	requests := []packageRequest{
		{
			Path: resolveRel(root, "database/model"),
			StructAllow: setOf(
				"User",
				"Inbound",
				"FallbackParentInfo",
				"OutboundTraffics",
				"InboundClientIps",
				"ApiToken",
				"HistoryOfSeeders",
				"Setting",
				"Node",
				"CustomGeoResource",
				"ClientReverse",
				"Client",
				"ClientRecord",
				"ClientInbound",
				"InboundFallback",
			),
			AliasAllow: setOf("Protocol"),
			Overrides: map[string][]walkOverride{
				"Inbound": {
					{Field: "Settings", Kind: KindAny},
					{Field: "StreamSettings", Kind: KindAny},
					{Field: "Sniffing", Kind: KindAny},
				},
				"ClientRecord": {
					{Field: "Reverse", Kind: KindAny},
				},
				"InboundClientIps": {
					{Field: "Ips", Kind: KindAny},
				},
			},
		},
		{
			Path: resolveRel(root, "web/entity"),
			StructAllow: setOf(
				"Msg",
				"AllSetting",
				"AllSettingView",
			),
		},
		{
			Path: resolveRel(root, "xray"),
			StructAllow: setOf(
				"ClientTraffic",
			),
		},
	}

	schemas, aliases, err := walkPackages(requests)
	if err != nil {
		return err
	}
	schemas = flattenEmbedded(schemas)

	if len(schemas) == 0 {
		return fmt.Errorf("no schemas produced; nothing to write")
	}

	target := filepath.Join(root, outDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	zodBuf := &bytes.Buffer{}
	if err := emitZod(zodBuf, schemas, aliases); err != nil {
		return err
	}
	typesBuf := &bytes.Buffer{}
	if err := emitTypes(typesBuf, schemas, aliases); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(target, "zod.ts"), zodBuf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "types.ts"), typesBuf.Bytes(), 0o644); err != nil {
		return err
	}

	fmt.Printf("openapigen: wrote %d schemas to %s\n", len(schemas), target)
	return nil
}

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
