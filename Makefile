MODULE := github.com/dashfactory/go-factory-io
GO := go
BINARY := secsgem

# Cross-compile targets
PLATFORMS := linux/amd64 linux/arm64 darwin/arm64

.PHONY: all build test lint clean cross-compile

all: test build

build:
	$(GO) build -o bin/$(BINARY) ./cmd/secsgem/

test:
	$(GO) test -v -race -count=1 ./...

test-cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

bench:
	$(GO) test -bench=. -benchmem ./pkg/message/secs2/

lint:
	$(GO) vet ./...

cross-compile:
	@for platform in $(PLATFORMS); do \
		os=$${platform%%/*}; \
		arch=$${platform##*/}; \
		output=bin/$(BINARY)-$$os-$$arch; \
		echo "Building $$output..."; \
		GOOS=$$os GOARCH=$$arch $(GO) build -o $$output ./cmd/secsgem/; \
	done

clean:
	rm -rf bin/ coverage.out coverage.html
