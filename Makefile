VERSION ?= dev
LDFLAGS := -s -w -X github.com/alexandrealan/devnat/internal/buildinfo.Version=$(VERSION)

.PHONY: build tidy vet test run-relay-dev clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o devnat ./cmd/devnat

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

run-relay-dev: build
	./devnat relay --dev --domain localhost --addr :8000

clean:
	rm -f devnat
	rm -rf dist
