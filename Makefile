.PHONY: build test lint fmt vet clean install

EXT       :=
ifeq ($(OS),Windows_NT)
  EXT     := .exe
endif

VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o wayback-dl$(EXT) .

# Cross-compile all release targets into per-platform subdirs.
# Binary is always named wayback-dl (wayback-dl.exe on Windows).
build-all:
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/linux_amd64/wayback-dl .
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/linux_arm64/wayback-dl .
	GOOS=linux   GOARCH=arm   GOARM=7 go build -ldflags "$(LDFLAGS)" -o dist/linux_armv7/wayback-dl .
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/darwin_arm64/wayback-dl .
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/darwin_amd64/wayback-dl .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/windows_amd64/wayback-dl.exe .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w -s .

lint:
	golangci-lint run

clean:
	rm -f wayback-dl wayback-dl.exe
	rm -rf dist/
