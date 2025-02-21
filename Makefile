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

# TARGET_ARCHS defines all GOARCH used for releasing binaries.
TARGET_ARCHS = arm64 amd64
BUILD_ACTION = --load

.DEFAULT_GOAL := ci
ci: build test validate e2e ## run the targets needed to validate a PR in CI.

clean: ## clean up project.
	rm -rf bin build

test: ## run unit tests.
	@echo "Running tests"
	$(GO) test -race -cover ./...

.PHONY: build
build: ## build project and output binary to TARGET_BIN.
	CGO_ENABLED=0 $(GO) build -trimpath -tags "$(GO_TAGS)" -ldflags "$(LINKFLAGS)" -o $(TARGET_BIN)

test-image:
	# Instead of loading image, target all platforms, effectivelly testing
	# the build for the target architectures.
	$(MAKE) build-image BUILD_ACTION="--platform=$(TARGET_PLATFORMS)"

build-image: buildx-machine ## build (and load) the container image targeting the current platform.
	$(IMAGE_BUILDER) build -f package/Dockerfile \
		--builder $(MACHINE) $(IMAGE_ARGS) \
		--build-arg VERSION=$(VERSION) -t "$(IMAGE)" $(BUILD_ACTION) .
	@echo "Built $(IMAGE)"

push-image: buildx-machine ## build the container image targeting all platforms defined by TARGET_PLATFORMS and push to a registry.
	$(IMAGE_BUILDER) build -f package/Dockerfile \
		--builder $(MACHINE) $(IMAGE_ARGS) $(IID_FILE_FLAG) $(BUILDX_ARGS) \
		--build-arg VERSION=$(VERSION) --platform=$(TARGET_PLATFORMS) -t "$(IMAGE)" --push .
	@echo "Pushed $(IMAGE)"

e2e: $(K3D) $(KUBECTL) $(HELM) build-image ## Run E2E tests.
	K3D=$(K3D) KUBECTL=$(KUBECTL) HELM=$(HELM) VERSION=$(VERSION) \
	IMAGE=$(IMAGE) \
		./hack/e2e

generate: ## Run code generation logic.
	$(GO) generate ./...

validate: validate-lint generate validate-dirty ## Run validation checks.

validate-lint: $(GOLANGCI)
	$(GOLANGCI) run --timeout=2m

validate-dirty:
ifdef DIRTY
	@echo Git is dirty
	@git --no-pager status
	@git --no-pager diff
	@exit 1
endif

upload: clean ## Build and upload artefacts to the GitHub release.
	$(MAKE) $(addsuffix -upload, $(TARGET_ARCHS))

%-upload:
	TARGET_BIN=build/bin/cis-operator-$(subst :,/,$*) \
	GOARCH=$(subst :,/,$*) GOOS=linux \
		$(MAKE) build

	TAG=$(TAG) \
		./hack/upload-gh $(subst :,/,$*)
