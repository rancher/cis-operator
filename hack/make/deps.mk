# renovate: datasource=github-release-attachments depName=golangci/golangci-lint
GOLANGCI_VERSION = v1.59.1
# renovate: datasource=github-release-attachments depName=k3d-io/k3d
K3D_VERSION = v5.7.3

KUBECTL_VERSION = 1.28.0
# curk -L "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/arm64/kubectl.sha256"
KUBECTL_SUM_arm64 = f5484bd9cac66b183c653abed30226b561f537d15346c605cc81d98095f1717c
# curk -L "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl.sha256"
KUBECTL_SUM_amd64 = 4717660fd1466ec72d59000bb1d9f5cdc91fac31d491043ca62b34398e0799ce
KUBECTL_SUM = KUBECTL_SUM_amd64

HELM_VERSION = v3.15.3
HELM_SUM_amd64 = ad871aecb0c9fd96aa6702f6b79e87556c8998c2e714a4959bf71ee31282ac9c
HELM_SUM_arm64 = bd57697305ba46fef3299b50168a34faa777dd2cf5b43b50df92cca7ed118cce
HELM_SUM = HELM_SUM_amd64
