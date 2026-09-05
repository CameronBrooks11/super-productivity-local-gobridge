.PHONY: build test test-live lint fmt fmt-check vet check race clean install docs snapshot

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
  -X github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version.Version=$(VERSION) \
  -X github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version.Commit=$(COMMIT) \
  -X github.com/CameronBrooks11/super-productivity-local-gobridge/internal/version.Date=$(DATE)

BINARY = sp-local-bridge

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/sp-local-bridge

test:
	go test ./... -count=1

# Verifies the client against a running Super Productivity: that the fields the
# bridge depends on are still present with the types it expects, and that the
# committed fixtures do not claim fields SP never returns. Excluded from `test`
# and from CI, which has no SP to talk to. Read-only (GET) — see AGENTS.md.
test-live:
	@command -v curl >/dev/null 2>&1 || \
	  { echo "curl is required for the preflight check."; exit 1; }
	@url="$${SP_BASE_URL:-http://127.0.0.1:3876}"; \
	  curl -sf "$$url/health" >/dev/null 2>&1 || \
	  { echo "Super Productivity is not reachable at $$url."; \
	    echo "Start it and enable Settings -> Sync & Export -> Local REST API,"; \
	    echo "or set SP_BASE_URL to point elsewhere."; exit 1; }
	go test -tags live ./... -count=1 -run TestLive -v

test-cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	golangci-lint run ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "Files need formatting:"; gofmt -l .; exit 1; }

vet:
	go vet ./...
	@# The live suite is excluded from ./... by its build tag, so nothing else
	@# type-checks it. Without this, renaming a client method leaves CI green and
	@# breaks the suite at the moment it is most needed: just before a release.
	go vet -tags live ./...

race:
	go test -race ./... -count=1

scripts-check:
	@bash -n scripts/install.sh
	@bash -n scripts/uninstall.sh
	@echo "Scripts syntax OK."

docs:
	npm run docs:build

check: fmt-check vet test scripts-check
	@echo "All checks passed."

snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { echo "goreleaser not installed"; exit 1; }
	goreleaser release --snapshot --clean

doctor: build
	./$(BINARY) doctor

clean:
	rm -f $(BINARY) coverage.out

install: build
	install -m 755 $(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || \
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
