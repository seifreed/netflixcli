# netflixcli — build, quality gate and security gate.

BIN ?= netflix

# Tool versions are pinned so a gate means the same thing on every machine and
# in CI. Bump them deliberately.
GOLANGCI_LINT ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
GOVULNCHECK   ?= golang.org/x/vuln/cmd/govulncheck@v1.8.0
OSV_SCANNER   ?= github.com/google/osv-scanner/v2/cmd/osv-scanner@v2.2.4
GITLEAKS      ?= github.com/zricethezav/gitleaks/v8@v8.29.0

.PHONY: build test cover fmt vet tidy lint race coverage-gate \
        quality security gate check clean

build:
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/netflix

test:
	CGO_ENABLED=0 go test ./...

cover:
	./coverage.sh

## ---------------------------------------------------------------- quality --

fmt:
	@test -z "$$(gofmt -l .)" || { echo "unformatted:"; gofmt -l .; exit 1; }

vet:
	CGO_ENABLED=0 go vet ./...

# staticcheck, revive, gocritic, errcheck, ineffassign, unused, gosec, govet.
lint:
	go run $(GOLANGCI_LINT) run ./cmd/... ./internal/...

# go.mod and go.sum must already be what `go mod tidy` would write, so a
# dependency cannot drift in unnoticed.
tidy-check:
	go mod tidy -diff

race:
	CGO_ENABLED=1 go test -race ./...

coverage-gate:
	./coverage-gate.sh

# The full quality gate. Run it before pushing.
quality: fmt vet lint tidy-check race coverage-gate
	@echo "quality gate: ok"

## --------------------------------------------------------------- security --

# Vulnerabilities on code paths the binary actually reaches.
vulncheck:
	go run $(GOVULNCHECK) ./...

# Vulnerabilities anywhere in the dependency graph, reachable or not.
osv:
	go run $(OSV_SCANNER) scan source -r .

# The CLI handles session cookies; a committed one must never reach the repo.
secrets:
	go run $(GITLEAKS) git --no-banner --redact --verbose .

# Module checksums match the ones recorded in go.sum.
modverify:
	go mod verify

security: vulncheck osv secrets modverify
	@echo "security gate: ok"

## ------------------------------------------------------------------ gates --

# Everything. What CI runs.
gate: quality security build
	@echo "all gates: ok"

# The quick pre-commit gate: formatting, vet, tests, build.
check: fmt vet test build
	@echo "ok"

tidy:
	go mod tidy

clean:
	rm -f $(BIN)
	rm -rf dist
