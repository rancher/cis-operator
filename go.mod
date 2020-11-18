module github.com/rancher/cis-operator

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/prometheus/client_golang v1.0.0
	github.com/rancher/kubernetes-provider-detector v0.0.0-20200807181951-690274ab1fb3
	github.com/rancher/lasso v0.0.0-20200820172840-0e4cc0ef5cb0
	github.com/rancher/security-scan v0.2.2-0.20201117171930-af478b83fbe4
	github.com/rancher/wrangler v0.6.2-0.20200829053106-7e1dd4260224
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/urfave/cli v1.22.2
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v10.0.0+incompatible
)
