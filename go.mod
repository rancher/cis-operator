module github.com/rancher/cis-operator

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/rancher/kubernetes-provider-detector v0.0.0-20200807181951-690274ab1fb3
	github.com/rancher/lasso v0.0.0-20200515155337-a34e1e26ad91
	github.com/rancher/security-scan v0.1.14
	github.com/rancher/wrangler v0.6.2-0.20200802063637-28dae3c1fc1b
	github.com/sirupsen/logrus v1.4.2
	github.com/urfave/cli v1.22.2
	k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v10.0.0+incompatible
)
