# renovate: datasource=github-release-attachments depName=golangci/golangci-lint
GOLANGCI_VERSION = v1.60.1
# renovate: datasource=github-release-attachments depName=k3d-io/k3d
K3D_VERSION = v5.7.3

KUBECTL_VERSION = 1.28.12
# curl -L "https://dl.k8s.io/release/v$KUBECTL_VERSION/bin/linux/arm64/kubectl.sha256"
KUBECTL_SUM_arm64 = f7e01dfffebb1d5811c37d558f28eefd80cbfadc0b9783b0b0ebf37c40c5c891
# curl -L "https://dl.k8s.io/release/v$KUBECTL_VERSION/bin/linux/amd64/kubectl.sha256"
KUBECTL_SUM_amd64 = e8aee7c9206c00062ced394418a17994b58f279a93a1be1143b08afe1758a3a2
KUBECTL_SUM = KUBECTL_SUM_amd64

HELM_VERSION = v3.15.3
HELM_SUM_amd64 = ad871aecb0c9fd96aa6702f6b79e87556c8998c2e714a4959bf71ee31282ac9c
HELM_SUM_arm64 = bd57697305ba46fef3299b50168a34faa777dd2cf5b43b50df92cca7ed118cce
HELM_SUM = HELM_SUM_amd64
