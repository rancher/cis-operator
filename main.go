//go:generate go run pkg/codegen/cleanup/main.go
//go:generate /bin/rm -rf pkg/generated
//go:generate go run pkg/codegen/main.go

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	cisoperatorapiv1 "github.com/rancher/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
	clusterscan_operator "github.com/rancher/clusterscan-operator/pkg/clusterscan-operator"
)

var (
	Version    = "v0.0.0-dev"
	GitCommit  = "HEAD"
	KubeConfig string
	threads    int
	name       string
)

func main() {
	app := cli.NewApp()
	app.Name = "clusterscan-operator"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "clusterscan-operator needs help!"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Destination: &KubeConfig,
		},
		cli.IntFlag{
			Name:        "threads",
			EnvVar:      "CIS_OPERATOR_THREADS",
			Value:       2,
			Destination: &threads,
		},
		cli.StringFlag{
			Name:        "name",
			EnvVar:      "CIS_OPERATOR_NAME",
			Value:       "clusterscan-operator",
			Destination: &name,
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) {
	flag.Parse()

	logrus.Info("Starting ClusterScan-Operator")
	ctx := signals.SetupSignalHandler(context.Background())

	kubeConfig, err := kubeconfig.GetNonInteractiveClientConfig(KubeConfig).ClientConfig()
	if err != nil {
		logrus.Fatalf("failed to find kubeconfig: %v", err)
	}

	ctl, err := clusterscan_operator.NewController(kubeConfig, ctx, cisoperatorapiv1.ClusterScanNS, name)
	if err != nil {
		logrus.Fatalf("Error building controller: %s", err.Error())
	}
	logrus.Info("Registering ClusterScan controller")

	if err := ctl.Start(ctx, threads, 2*time.Hour); err != nil {
		logrus.Fatalf("Error starting: %v", err)
	}
	<-ctx.Done()
	logrus.Info("Registered ClusterScan controller")
}
