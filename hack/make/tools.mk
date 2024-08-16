TOOLS_BIN := $(shell mkdir -p build/tools && realpath build/tools)
OS_NAME = $(shell uname -s | tr A-Z a-z)
OS_ARCH = $(shell uname -m)

ifeq ($(OS_ARCH),x86_64)
	OS_ARCH = amd64
endif
ifeq ($(OS_ARCH),aarch64)
	OS_ARCH = arm64
endif

K3D = $(TOOLS_BIN)/k3d-$(K3D_VERSION)
$(K3D):
	rm -f $(TOOLS_BIN)/k3d*
	curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | \
		PATH=$(PATH):$(TOOLS_BIN) K3D_INSTALL_DIR="$(TOOLS_BIN)" TAG="$(K3D_VERSION)" USE_SUDO=false bash
	mv $(TOOLS_BIN)/k3d $(TOOLS_BIN)/k3d-$(K3D_VERSION)

GOLANGCI = $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_VERSION)
$(GOLANGCI):
	rm -f $(TOOLS_BIN)/golangci-lint*
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_BIN) $(GOLANGCI_VERSION)
	mv $(TOOLS_BIN)/golangci-lint $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_VERSION)

KUBECTL = $(TOOLS_BIN)/kubectl-$(KUBECTL_VERSION)
$(KUBECTL):
	rm -f $(TOOLS_BIN)/kubectl*
	curl --output $(KUBECTL) -sSfL "https://dl.k8s.io/release/v$(KUBECTL_VERSION)/bin/$(OS_NAME)/$(OS_ARCH)/kubectl"
	$(call indirect-value,KUBECTL_SUM)
	echo "$(RESULT)  $(KUBECTL)" | sha256sum -c -
	chmod u+x $(KUBECTL)

HELM = $(TOOLS_BIN)/helm-$(HELM_VERSION)
$(HELM):
	rm -rf $(TOOLS_BIN)/helm*
	mkdir -p $(TOOLS_BIN)/tmp-helm
	curl --output $(TOOLS_BIN)/helm.tar.gz -sSfL "https://get.helm.sh/helm-$(HELM_VERSION)-$(OS_NAME)-$(OS_ARCH).tar.gz"
	$(call indirect-value,HELM_SUM)
	echo "$(RESULT)  $(TOOLS_BIN)/helm.tar.gz" | sha256sum -c -
	tar -xf $(TOOLS_BIN)/helm.tar.gz --strip-components 1 -C $(TOOLS_BIN)/tmp-helm
	mv $(TOOLS_BIN)/tmp-helm/helm $(HELM)
	chmod u+x $(HELM)
	rm -rf $(TOOLS_BIN)/helm.tar.gz $(TOOLS_BIN)/tmp-helm

# indirect-value gets the value of a Makefile var from a var that contains its name.
# This is equivalent to ${!var} in bash.
define indirect-value
    $(eval RESULT := $$($$($1)))
endef

# go-install-tool will 'go install' any package $2 and install it as $1.
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
echo "Downloading $(2)" ;\
GOBIN=$(TOOLS_BIN) go install $(2) ;\
}
endef
