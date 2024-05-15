# renovate: datasource=github-release-attachments depName=golangci/golangci-lint
GOLANGCI_VERSION ?= v1.56.0
# renovate: datasource=github-release-attachments depName=k3d-io/k3d
K3D_VERSION ?= v5.6.3

KUBECTL_VERSION ?= 1.28.0
# curk -L "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/arm64/kubectl.sha256"
KUBECTL_SUM_arm64 ?= f5484bd9cac66b183c653abed30226b561f537d15346c605cc81d98095f1717c
# curk -L "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl.sha256"
KUBECTL_SUM_amd64 ?= 4717660fd1466ec72d59000bb1d9f5cdc91fac31d491043ca62b34398e0799ce
KUBECTL_SUM = KUBECTL_SUM_amd64

# renovate: datasource=github-release-attachments depName=rancher/security-scan
SECURITY_SCAN_VERSION ?= v0.2.15
# renovate: datasource=github-release-attachments depName=vmware-tanzu/sonobuoy
SONOBUOY_VERSION ?= v0.57.1
# renovate: datasource=github-release-attachments depName=coredns/coredns
CORE_DNS_VERSION ?= 1.9.4
# renovate: datasource=github-release-attachments depName=k3s-io/klipper-helm
KLIPPER_HELM_VERSION ?= v0.7.4-build20221121
