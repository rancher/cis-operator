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
	curl -sSfL -o $(K3D) "https://github.com/k3d-io/k3d/releases/download/$(K3D_VERSION)/k3d-$(OS_NAME)-$(OS_ARCH)"
	echo "$(K3D_SUM_$(OS_ARCH))  $(K3D)" | shasum -a 256 -c -
	chmod u+x $(K3D)

GOLANGCI = $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_VERSION)
GOLANGCI_VERSION_TRIMMED := $(GOLANGCI_VERSION:v%=%)
$(GOLANGCI):
	rm -rf $(TOOLS_BIN)/golangci*
	curl -sSfL -o $(TOOLS_BIN)/golangci.tar.gz \
		"https://github.com/golangci/golangci-lint/releases/download/$(GOLANGCI_VERSION)/golangci-lint-$(GOLANGCI_VERSION_TRIMMED)-$(OS_NAME)-$(OS_ARCH).tar.gz"
	echo "$(GOLANGCI_SUM_$(OS_ARCH))  $(TOOLS_BIN)/golangci.tar.gz" | shasum -a 256 -c -
	tar -xf $(TOOLS_BIN)/golangci.tar.gz -C $(TOOLS_BIN)
	mv $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_VERSION_TRIMMED)-$(OS_NAME)-$(OS_ARCH)/golangci-lint $(GOLANGCI)
	chmod u+x $(GOLANGCI)
	rm -rf $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_VERSION_TRIMMED)-$(OS_NAME)-$(OS_ARCH)
	rm -f $(TOOLS_BIN)/golangci.tar.gz

KUBECTL = $(TOOLS_BIN)/kubectl-$(KUBECTL_VERSION)
$(KUBECTL):
	rm -f $(TOOLS_BIN)/kubectl*
	curl -sSfL -o $(KUBECTL) \
		"https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OS_NAME)/$(OS_ARCH)/kubectl"
	echo "$(KUBECTL_SUM_$(OS_ARCH))  $(KUBECTL)" | shasum -a 256 -c -
	chmod u+x $(KUBECTL)

HELM = $(TOOLS_BIN)/helm-$(HELM_VERSION)
$(HELM):
	rm -rf $(TOOLS_BIN)/helm*
	mkdir -p $(TOOLS_BIN)/tmp-helm
	curl -sSfL -o $(TOOLS_BIN)/helm.tar.gz \
		"https://get.helm.sh/helm-$(HELM_VERSION)-$(OS_NAME)-$(OS_ARCH).tar.gz"
	echo "$(HELM_SUM_$(OS_ARCH))  $(TOOLS_BIN)/helm.tar.gz" | shasum -a 256 -c -
	tar -xf $(TOOLS_BIN)/helm.tar.gz --strip-components 1 -C $(TOOLS_BIN)/tmp-helm
	mv $(TOOLS_BIN)/tmp-helm/helm $(HELM)
	chmod u+x $(HELM)
	rm -rf $(TOOLS_BIN)/helm.tar.gz $(TOOLS_BIN)/tmp-helm

