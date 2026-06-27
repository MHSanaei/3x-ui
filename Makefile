# Canonical task runner. Mirrors .github/workflows/ci.yml so `make verify`
# reproduces the PR gate locally. Run `make help` for the list.

SHELL := bash
GO_PKGS = $(shell go list ./... | grep -v '/frontend/node_modules/')
FRONTEND = frontend

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

# go:embed of internal/web/dist needs the dir to exist even when the
# frontend bundle has not been built. CI stubs it the same way.
.PHONY: dist-stub
dist-stub:
	@mkdir -p internal/web/dist && touch internal/web/dist/.gitkeep

.PHONY: gen
gen: ## Regenerate Zod schemas + OpenAPI from Go sources
	cd $(FRONTEND) && npm run gen

.PHONY: gen-check
gen-check: gen ## Fail if generated files are stale
	git diff --exit-code -- frontend/src/generated frontend/public/openapi.json

.PHONY: lint-go
lint-go: dist-stub ## golangci-lint on Go sources
	golangci-lint run

.PHONY: lint-fe
lint-fe: ## ESLint on frontend sources
	cd $(FRONTEND) && npm run lint

.PHONY: lint
lint: lint-go lint-fe ## All linters

.PHONY: typecheck
typecheck: ## tsc --noEmit
	cd $(FRONTEND) && npm run typecheck

.PHONY: test-go
test-go: dist-stub ## Go tests (shuffle, no cache)
	go test -shuffle=on -count=1 $(GO_PKGS)

.PHONY: race
race: dist-stub ## Go tests with the race detector (needs a C compiler)
	go test -race -shuffle=on -count=1 $(GO_PKGS)

.PHONY: test-fe
test-fe: ## Frontend tests (vitest)
	cd $(FRONTEND) && npm test

.PHONY: test
test: test-go test-fe ## All tests

.PHONY: vulncheck
vulncheck: dist-stub ## govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: build-fe
build-fe: ## Build the Vite bundles into internal/web/dist
	cd $(FRONTEND) && npm run build

.PHONY: build
build: build-fe ## Build the frontend then the Go binary
	go build ./...

# The PR gate. Matches ci.yml: codegen freshness, both linters, typecheck,
# both test suites, and a full build.
.PHONY: verify
verify: gen-check lint typecheck test build ## Full local gate (mirrors CI)
	@echo "verify: OK"
