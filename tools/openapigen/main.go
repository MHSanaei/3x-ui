package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("root", ".", "repository root containing internal/database/model and internal/web/entity")
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
			Path: resolveRel(root, "internal/database/model"),
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
				"ClientReverse",
				"Client",
				"ClientRecord",
				"ClientInbound",
				"InboundFallback",
				"Host",
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
				"Host": {
					{Field: "MuxParams", Kind: KindAny},
					{Field: "SockoptParams", Kind: KindAny},
				},
			},
		},
		{
			Path: resolveRel(root, "internal/web/entity"),
			StructAllow: setOf(
				"Msg",
				"AllSetting",
				"AllSettingView",
			),
		},
		{
			Path: resolveRel(root, "internal/xray"),
			StructAllow: setOf(
				"ClientTraffic",
			),
		},
		{
			Path: resolveRel(root, "internal/web/service"),
			StructAllow: setOf(
				"InboundOption",
				"ProbeResultUI",
			),
		},
		{
			Path:        resolveRel(root, "internal/web/service/panel"),
			StructAllow: setOf("ApiTokenView"),
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
	examplesBuf := &bytes.Buffer{}
	if err := emitExamples(examplesBuf, schemas, aliases); err != nil {
		return err
	}
	schemasBuf := &bytes.Buffer{}
	if err := emitJSONSchema(schemasBuf, schemas, aliases); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(target, "zod.ts"), zodBuf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "types.ts"), typesBuf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "examples.ts"), examplesBuf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "schemas.ts"), schemasBuf.Bytes(), 0o644); err != nil {
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
