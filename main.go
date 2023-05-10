//go:generate go run pkg/codegen/cleanup/main.go
//go:generate /bin/rm -rf pkg/generated
//go:generate go run pkg/codegen/main.go

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	cisoperator "github.com/rancher/cis-operator/pkg/securityscan"
	corev1 "k8s.io/api/core/v1"
)

var (
	Version                       = "v0.0.0-dev"
	GitCommit                     = "HEAD"
	kubeConfig                    string
	threads                       int
	name                          string
	metricsPort                   string
	alertSeverity                 string
	debug                         bool
	securityScanImage             = "rancher/security-scan"
	securityScanImageTag          = "v0.2.9"
	sonobuoyImage                 = "rancher/mirrored-sonobuoy-sonobuoy"
	sonobuoyImageTag              = "v0.56.14"
	clusterName                   string
	securityScanJobTolerationsVal string
)

func main() {
	app := cli.NewApp()
	app.Name = "cis-operator"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "cis-operator needs help!"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Destination: &kubeConfig,
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
			Value:       "cis-operator",
			Destination: &name,
		},
		cli.StringFlag{
			Name:        "security-scan-image",
			EnvVar:      "SECURITY_SCAN_IMAGE",
			Value:       "rancher/security-scan",
			Destination: &securityScanImage,
		},
		cli.StringFlag{
			Name:        "security-scan-image-tag",
			EnvVar:      "SECURITY_SCAN_IMAGE_TAG",
			Value:       "latest",
			Destination: &securityScanImageTag,
		},
		cli.StringFlag{
			Name:        "sonobuoy-image",
			EnvVar:      "SONOBUOY_IMAGE",
			Value:       "rancher/sonobuoy-sonobuoy",
			Destination: &sonobuoyImage,
		},
		cli.StringFlag{
			Name:        "sonobuoy-image-tag",
			EnvVar:      "SONOBUOY_IMAGE_TAG",
			Value:       "v0.16.3",
			Destination: &sonobuoyImageTag,
		},
		cli.StringFlag{
			Name:        "cis_metrics_port",
			EnvVar:      "CIS_METRICS_PORT",
			Value:       "8080",
			Destination: &metricsPort,
		},
		cli.BoolFlag{
			Name:        "debug",
			EnvVar:      "CIS_OPERATOR_DEBUG",
			Destination: &debug,
		},
		cli.StringFlag{
			Name:        "alertSeverity",
			EnvVar:      "CIS_ALERTS_SEVERITY",
			Value:       "warning",
			Destination: &alertSeverity,
		},
		cli.StringFlag{
			Name:        "clusterName",
			EnvVar:      "CLUSTER_NAME",
			Value:       "",
			Destination: &clusterName,
		},
		cli.StringFlag{
			Name:        "security-scan-job-tolerations",
			EnvVar:      "SECURITY_SCAN_JOB_TOLERATIONS",
			Value:       "",
			Destination: &securityScanJobTolerationsVal,
		},
		cli.BoolFlag{
			Name:   "alertEnabled",
			EnvVar: "CIS_ALERTS_ENABLED",
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) {
	logrus.Info("Starting CIS-Operator")

	ctx := context.Background()
	handler := signals.SetupSignalHandler()
	go func() {
		<-handler
		ctx.Done()
	}()

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	kubeConfig = c.String("kubeconfig")
	threads = c.Int("threads")
	securityScanImage = c.String("security-scan-image")
	securityScanImageTag = c.String("security-scan-image-tag")
	sonobuoyImage = c.String("sonobuoy-image")
	sonobuoyImageTag = c.String("sonobuoy-image-tag")
	name = c.String("name")

	securityScanJobTolerations := []corev1.Toleration{{
		Operator: corev1.TolerationOpExists,
	}}

	securityScanJobTolerationsVal = c.String("security-scan-job-tolerations")

	if securityScanJobTolerationsVal != "" {
		err := json.Unmarshal([]byte(securityScanJobTolerationsVal), &securityScanJobTolerations)
		if err != nil {
			logrus.Fatalf("invalid value received for security-scan-job-tolerations flag:%s", err.Error())
		}
	}

	kubeConfig, err := kubeconfig.GetNonInteractiveClientConfig(kubeConfig).ClientConfig()
	if err != nil {
		logrus.Fatalf("failed to find kubeconfig: %v", err)
	}

	imgConfig := &cisoperatorapiv1.ScanImageConfig{
		SecurityScanImage:    securityScanImage,
		SecurityScanImageTag: securityScanImageTag,
		SonobuoyImage:        sonobuoyImage,
		SonobuoyImageTag:     sonobuoyImageTag,
		AlertSeverity:        alertSeverity,
		ClusterName:          clusterName,
		AlertEnabled:         c.Bool("alertEnabled"),
	}

	if err := validateConfig(imgConfig); err != nil {
		logrus.Fatalf("Error starting CIS-Operator: %v", err)
	}

	ctl, err := cisoperator.NewController(ctx, kubeConfig, cisoperatorapiv1.ClusterScanNS, name, imgConfig, securityScanJobTolerations)
	if err != nil {
		logrus.Fatalf("Error building controller: %s", err.Error())
	}

	if err := ctl.Start(ctx, threads, 2*time.Hour); err != nil {
		logrus.Fatalf("Error starting: %v", err)
	}
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":"+metricsPort, nil))

	<-handler
	ctx.Done()
	logrus.Info("Registered CIS controller")
}

func validateConfig(imgConfig *cisoperatorapiv1.ScanImageConfig) error {
	if imgConfig.SecurityScanImage == "" {
		return errors.New("No Security-Scan Image specified")
	}
	if imgConfig.SonobuoyImage == "" {
		return errors.New("No Sonobuoy tool Image specified")
	}
	return nil
}
