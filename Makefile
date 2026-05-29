BINARY := fire
PKG := github.com/stefanpenner/fire.cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X $(PKG)/cmd.Version=$(VERSION)"

.PHONY: build test lint fmt vet install clean

build: ## Build the fire binary
	go build $(LDFLAGS) -o $(BINARY) .

test: ## Run all tests
	go test ./...

lint: vet ## Vet + gofmt check
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)

vet:
	go vet ./...

fmt: ## Format the code
	gofmt -w .

install: ## go install the binary
	go install $(LDFLAGS) .

clean:
	rm -f $(BINARY)
