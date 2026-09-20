# netflixcli — build & maintenance.

BIN ?= netflix

.PHONY: build test cover fmt vet tidy check clean

build:
	CGO_ENABLED=0 go build -o $(BIN) ./cmd/netflix

test:
	CGO_ENABLED=0 go test ./...

cover:
	./coverage.sh

fmt:
	@test -z "$$(gofmt -l .)" || { echo "unformatted:"; gofmt -l .; exit 1; }

vet:
	CGO_ENABLED=0 go vet ./...

tidy:
	go mod tidy

# Pre-release gate: formatting, vet, tests, build.
check: fmt vet test build
	@echo "ok"

clean:
	rm -f $(BIN)
	rm -rf dist
