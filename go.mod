module github.com/rancher/cis-operator

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.19.2

require (
	github.com/blang/semver v3.5.0+incompatible
	github.com/prometheus-operator/prometheus-operator v0.43.2
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.43.0
	github.com/prometheus/client_golang v1.8.0
	github.com/rancher/kubernetes-provider-detector v0.0.0-20200807181951-690274ab1fb3
	github.com/rancher/lasso v0.0.0-20200820172840-0e4cc0ef5cb0
	github.com/rancher/security-scan v0.2.2-rc2
	github.com/rancher/wrangler v0.6.2-0.20200829053106-7e1dd4260224
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/urfave/cli v1.22.2
	golang.org/x/tools v0.0.0-20201120032337-6d151481565c // indirect
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v12.0.0+incompatible
)
