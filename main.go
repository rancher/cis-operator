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

	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	cisoperator "github.com/rancher/cis-operator/pkg/securityscan"

	// Automatically sets fallback trusted x509 roots, in case they are
	// not available at runtime. This is required to establish trust
	// when deployed into a scratch container.
	_ "golang.org/x/crypto/x509roots/fallback"

	// Embed a copy of the timezone database, so that it does not depend
	// on it being available at runtime.
	_ "time/tzdata"

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
	securityScanImage             string
	securityScanImageTag          string
	sonobuoyImage                 string
	sonobuoyImageTag              string
	clusterName                   string
	securityScanJobTolerationsVal string
)

func main() {
	app := cli.NewApp()
	app.Name = "cis-operator"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Usage = "cis-operator needs help!"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "kubeconfig",
			EnvVars:     []string{"KUBECONFIG"},
			Destination: &kubeConfig,
		},
		&cli.IntFlag{
			Name:        "threads",
			EnvVars:     []string{"CIS_OPERATOR_THREADS"},
			Value:       2,
			Destination: &threads,
		},
		&cli.StringFlag{
			Name:        "name",
			EnvVars:     []string{"CIS_OPERATOR_NAME"},
			Value:       "cis-operator",
			Destination: &name,
		},
		&cli.StringFlag{
			Name:        "security-scan-image",
			EnvVars:     []string{"SECURITY_SCAN_IMAGE"},
			Value:       "rancher/security-scan",
			Destination: &securityScanImage,
		},
		&cli.StringFlag{
			Name:        "security-scan-image-tag",
			EnvVars:     []string{"SECURITY_SCAN_IMAGE_TAG"},
			Value:       "latest",
			Destination: &securityScanImageTag,
		},
		&cli.StringFlag{
			Name:        "sonobuoy-image",
			EnvVars:     []string{"SONOBUOY_IMAGE"},
			Value:       "rancher/sonobuoy-sonobuoy",
			Destination: &sonobuoyImage,
		},
		&cli.StringFlag{
			Name:        "sonobuoy-image-tag",
			EnvVars:     []string{"SONOBUOY_IMAGE_TAG"},
			Value:       "latest",
			Destination: &sonobuoyImageTag,
		},
		&cli.StringFlag{
			Name:        "cis_metrics_port",
			EnvVars:     []string{"CIS_METRICS_PORT"},
			Value:       "8080",
			Destination: &metricsPort,
		},
		&cli.BoolFlag{
			Name:        "debug",
			EnvVars:     []string{"CIS_OPERATOR_DEBUG"},
			Destination: &debug,
		},
		&cli.StringFlag{
			Name:        "alertSeverity",
			EnvVars:     []string{"CIS_ALERTS_SEVERITY"},
			Value:       "warning",
			Destination: &alertSeverity,
		},
		&cli.StringFlag{
			Name:        "clusterName",
			EnvVars:     []string{"CLUSTER_NAME"},
			Value:       "",
			Destination: &clusterName,
		},
		&cli.StringFlag{
			Name:        "security-scan-job-tolerations",
			EnvVars:     []string{"SECURITY_SCAN_JOB_TOLERATIONS"},
			Value:       "",
			Destination: &securityScanJobTolerationsVal,
		},
		&cli.BoolFlag{
			Name:    "alertEnabled",
			EnvVars: []string{"CIS_ALERTS_ENABLED"},
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
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
	if err := http.ListenAndServe(":"+metricsPort, nil); err != nil {
		log.Fatal(err)
	}

	<-handler
	ctx.Done()
	logrus.Info("Registered CIS controller")
	return nil
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
