## githand Makefile
##
## Common targets:
##   make            — build for current OS/arch
##   make test       — run all tests
##   make lint       — vet + staticcheck
##   make cross      — cross-compile all platforms
##   make clean      — remove build artifacts

BINARY    := githand
CMD       := ./cmd/githand/
BINDIR    := bin
GO        := go
GOFLAGS   := -trimpath
LDFLAGS   := -s -w

## Version injection from git (falls back to "dev")
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_LDF := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

## Cross-compilation targets
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

## ── Default ──────────────────────────────────────────────

.PHONY: all
all: build

## ── Build ────────────────────────────────────────────────

.PHONY: build
build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(BUILD_LDF)" -o $(BINDIR)/$(BINARY) $(CMD)

.PHONY: install
install: build
	cp $(BINDIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

## ── Test ─────────────────────────────────────────────────

.PHONY: test
test:
	$(GO) test ./internal/... -count=1 -race

.PHONY: coverage
coverage:
	$(GO) test ./internal/... -count=1 -race -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

## ── Lint ─────────────────────────────────────────────────

.PHONY: lint
lint: vet staticcheck

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: staticcheck
staticcheck:
	@which staticcheck > /dev/null 2>&1 || (echo "installing staticcheck..." && $(GO) install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...

## ── Cross-compilation ───────────────────────────────────

define cross_template
.PHONY: cross-$(1)-$(2)
cross-$(1)-$(2):
	CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) $$(GO) build $$(GOFLAGS) -ldflags "$$(LDFLAGS) $$(BUILD_LDF)" -o $$(BINDIR)/$(BINARY)-$(1)-$(2)$(if $(filter windows,$(1)),.exe,) $$(CMD)
endef

$(foreach plat,$(PLATFORMS),$(eval $(call cross_template,$(word 1,$(subst /, ,$(plat))),$(word 2,$(subst /, ,$(plat))))))

.PHONY: cross
cross: $(foreach plat,$(PLATFORMS),cross-$(subst /,-,$(plat)))

## ── GoReleaser (local snapshot) ─────────────────────────

.PHONY: snapshot
snapshot:
	goreleaser build --snapshot --clean

## ── Clean ────────────────────────────────────────────────

.PHONY: clean
clean:
	rm -rf $(BINDIR)/
	rm -f coverage.out
	rm -f dist/

## ── Help ─────────────────────────────────────────────────

.PHONY: help
help:
	@echo "githand build targets:"
	@echo "  make          — build for current platform"
	@echo "  make test     — run tests (race detector on)"
	@echo "  make lint     — go vet + staticcheck"
	@echo "  make cross    — cross-compile all platforms"
	@echo "  make snapshot — goreleaser local snapshot build"
	@echo "  make clean    — remove bin/ and dist/"
	@echo ""
	@echo "Cross-compile individual targets:"
	@$(foreach plat,$(PLATFORMS),echo "  make cross-$(subst /,-,$(plat))";)
