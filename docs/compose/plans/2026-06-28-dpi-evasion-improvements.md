# DPI Evasion Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden existing protocols against Iran's DPI by improving XHTTP defaults, Reality targets, and enabling obfuscation by default.

**Architecture:** Three targeted changes across frontend schemas and Go backend seed list. No new features — just better defaults that make traffic harder to fingerprint.

**Tech Stack:** TypeScript (Zod schemas), Go (seed target list)

---

## Task 1: Enable XHTTP Padding Obfuscation by Default

**Covers:** XHTTP padding obfuscation and SSE header removal

**Files:**
- Modify: `frontend/src/schemas/protocols/stream/xhttp.ts:60,80`

**Interfaces:**
- Consumes: `XHttpStreamSettingsSchema` (existing)
- Produces: Updated defaults for `xPaddingObfsMode` and `noSSEHeader`

- [ ] **Step 1: Update XHTTP schema defaults**

In `frontend/src/schemas/protocols/stream/xhttp.ts`, change:

```typescript
// Line 60: change default from false to true
xPaddingObfsMode: z.boolean().default(true),

// Line 80: change default from false to true
noSSEHeader: z.boolean().default(true),
```

- [ ] **Step 2: Verify tests still pass**

Run: `cd frontend && npm test`
Expected: All tests pass (existing tests that explicitly set these values to `true` will still work; tests that rely on default `false` need updating if they assert the default).

- [ ] **Step 3: Run typecheck**

Run: `cd frontend && npm run typecheck`
Expected: No type errors.

---

## Task 2: Add Iran-Optimized Reality Targets to Backend Seed List

**Covers:** Better Reality target discovery for Iran DPI evasion

**Files:**
- Modify: `internal/web/service/reality_scan.go:27-38`

**Interfaces:**
- Consumes: `defaultRealityScanCandidates` (existing seed list)
- Produces: Expanded seed list with Iran-friendly targets

- [ ] **Step 1: Expand the defaultRealityScanCandidates list**

In `internal/web/service/reality_scan.go`, replace the existing list with:

```go
var defaultRealityScanCandidates = []string{
	"www.samsung.com:443",
	"www.microsoft.com:443",
	"www.nvidia.com:443",
	"www.amd.com:443",
	"www.intel.com:443",
	"www.sony.com:443",
	"dl.google.com:443",
	"www.amazon.com:443",
	"aws.amazon.com:443",
	"www.cloudflare.com:443",
	"www.mozilla.org:443",
	"www.yahoo.com:443",
	"www.fujitsu.com:443",
	"www.ibm.com:443",
	"support.lenovo.com:443",
}
```

Rationale: Samsung, Microsoft, Nvidia, AMD, Intel, Sony, and Google are commonly visited sites in Iran — their domains are less likely to be blocked or scrutinized by DPI. Moved `www.samsung.com` to top since it's a reliable Reality target with TLS 1.3 + X25519 support.

- [ ] **Step 2: Verify Go code compiles**

Run: `go build ./...`
Expected: No compilation errors.

- [ ] **Step 3: Run Go tests**

Run: `go test ./internal/web/service/... -run Reality -v`
Expected: Tests pass.

---

## Task 3: Change Default Reality Target for New Inbounds

**Covers:** Better default Reality target when creating a new inbound

**Files:**
- Modify: `frontend/src/schemas/protocols/security/reality.ts:58-59`
- Modify: `frontend/src/pages/inbounds/form/useSecurityActions.ts:266-267`

**Interfaces:**
- Consumes: `RealityStreamSettingsSchema` (existing)
- Produces: Updated default target and serverNames

- [ ] **Step 1: Update Reality schema defaults**

In `frontend/src/schemas/protocols/security/reality.ts`, change:

```typescript
// Line 58: change default target
target: z.string().default('www.samsung.com:443'),

// Line 59: change default serverNames
serverNames: z.array(z.string()).default(['www.samsung.com']),
```

- [ ] **Step 2: Update useSecurityActions.ts fallback**

In `frontend/src/pages/inbounds/form/useSecurityActions.ts`, change:

```typescript
// Line 266-267: change the fallback target when switching to reality
reality.target = 'www.samsung.com:443';
reality.serverNames = ['www.samsung.com'];
```

- [ ] **Step 3: Run tests**

Run: `cd frontend && npm test`
Expected: All tests pass.

- [ ] **Step 4: Run typecheck**

Run: `cd frontend && npm run typecheck`
Expected: No type errors.

---

## Task 4: Run Full Verification

- [ ] **Step 1: Run full verification**

Run: `make verify`
Expected: All checks pass (Go tests, frontend tests, lint, typecheck).

- [ ] **Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

---

## Summary of Changes

| Change | File | Before | After |
|--------|------|--------|-------|
| XHTTP padding obfuscation | `xhttp.ts` | `default(false)` | `default(true)` |
| XHTTP no SSE header | `xhttp.ts` | `default(false)` | `default(true)` |
| Reality default target | `reality.ts` | `images.apple.com:443` | `www.samsung.com:443` |
| Reality scan seed list | `reality_scan.go` | 10 targets | 15 targets (Iran-optimized) |

**Impact:** New inbounds created after this change will:
1. Use XHTTP with padding obfuscation enabled by default (harder to fingerprint)
2. Remove SSE headers by default (removes a DPI fingerprint)
3. Use `www.samsung.com:443` as Reality target (commonly visited in Iran, reliable TLS 1.3)
4. Benefit from expanded Reality target scanning with Iran-friendly domains
