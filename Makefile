.PHONY: build test lint fmt fmt-check vet check race clean install docs snapshot

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
