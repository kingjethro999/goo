BINARY    := goo
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE      := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS   := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: build run test test-unit test-cover lint clean install

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./

run:
	go run $(LDFLAGS) . $(ARGS)

install:
	go install $(LDFLAGS) ./

test:
	go test ./...

test-unit:
	go test ./... -short

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html

# Build for multiple platforms
release:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   ./
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  ./
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./

# First time setup
setup:
	go mod tidy
	mkdir -p bin dist
	@echo "Run 'make build' to compile."

# Database migrations
db-migrate:
	sqlite3 ~/.config/goo/history.db < memory/schema.sql
