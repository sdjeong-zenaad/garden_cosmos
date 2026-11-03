.DEFAULT_GOAL := build

# GO selects the launcher. GOTOOLCHAIN (environment or make command line)
# selects its toolchain; when omitted, Go's own saved/default setting applies.
GO ?= go
PREFIX ?= $(HOME)/go
BINDIR ?= $(PREFIX)/bin
DESTDIR ?=
TESTFLAGS ?= -timeout 30s
VERSION := $(strip $(shell cat VERSION))

.PHONY: build install toolchain test vet help

build: toolchain
	@mkdir -p bin
	"$(GO)" build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/gcosm ./cmd/gcosm

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 0755 bin/gcosm "$(DESTDIR)$(BINDIR)/gcosm"
	@printf 'Installed: %s\n' "$(DESTDIR)$(BINDIR)/gcosm"

toolchain:
	@command -v "$(GO)" >/dev/null 2>&1 || { printf 'Go 실행 파일을 찾을 수 없습니다: %s\nGO=/path/to/go 로 지정하세요.\n' "$(GO)" >&2; exit 1; }
	@printf 'Go launcher: %s\n' "$(GO)"
	@"$(GO)" version
	@printf 'GOTOOLCHAIN: '
	@"$(GO)" env GOTOOLCHAIN
	@printf 'Required by go.mod: '
	@awk '$$1 == "go" { print $$2 }' go.mod

test: toolchain
	"$(GO)" test $(TESTFLAGS) ./...

vet: toolchain
	"$(GO)" vet ./...

help:
	@printf '%s\n' \
	  'make                         Build bin/gcosm' \
	  'make install                 Install to ~/go/bin/gcosm' \
	  'make install PREFIX=/path    Install to /path/bin/gcosm' \
	  'make GO=/path/to/go          Select a Go executable' \
	  'make GOTOOLCHAIN=go1.x.y      Select an exact Go toolchain' \
	  'make GOTOOLCHAIN=local       Use only the launcher toolchain' \
	  'make toolchain               Show selected and required Go versions' \
	  'make test / make vet         Run checks with the same Go selection'
