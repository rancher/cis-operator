# To avoid poluting the Makefile, versions and checksums for tooling and 
# dependencies are defined at hack/make/deps.mk.
include hack/make/deps.mk

# Include logic that can be reused across projects.
include hack/make/build.mk
include hack/make/tools.mk

# Define target platforms, image builder and the fully qualified image name.
TARGET_PLATFORMS ?= linux/amd64,linux/arm64

REPO ?= rancher
IMAGE = $(REPO)/cis-operator:$(TAG)
TARGET_BIN ?= build/bin/cis-operator
ARCH ?= $(shell docker info --format '{{.ClientInfo.Arch}}')

.DEFAULT_GOAL := ci
ci: build test validate e2e ## run the targets needed to validate a PR in CI.

clean: ## clean up project.
	rm -rf bin build

test: ## run unit tests.
	@echo "Running tests"
	$(GO) test -race -cover ./...

.PHONY: build
build: ## build project and output binary to TARGET_BIN.
	CGO_ENABLED=0 $(GO) build -tags "$(GO_TAGS)" -ldflags "$(LINKFLAGS)" -o $(TARGET_BIN)

generate:
	$(GO) generate ./...

validate: validate-lint generate validate-dirty

validate-lint: $(GOLANGCI)
	$(GOLANGCI) run

validate-dirty:
ifdef DIRTY
	@echo Git is dirty
	@git --no-pager status
	@git --no-pager diff
	@exit 1
endif
