package securityscan

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	detector "github.com/rancher/kubernetes-provider-detector"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/crd"
	appsctl "github.com/rancher/wrangler/pkg/generated/controllers/apps"
	appsctlv1 "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	batchctl "github.com/rancher/wrangler/pkg/generated/controllers/batch"
	batchctlv1 "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	corectl "github.com/rancher/wrangler/pkg/generated/controllers/core"
	corectlv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/start"

	"sync"

	"github.com/prometheus/client_golang/prometheus"

	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	cisoperatorctl "github.com/rancher/cis-operator/pkg/generated/controllers/cis.cattle.io"
	cisoperatorctlv1 "github.com/rancher/cis-operator/pkg/generated/controllers/cis.cattle.io/v1"
	"github.com/rancher/cis-operator/pkg/securityscan/scan"
	corev1 "k8s.io/api/core/v1"
)

type Controller struct {
	Namespace         string
	Name              string
	ClusterProvider   string
	KubernetesVersion string
	ImageConfig       *cisoperatorapiv1.ScanImageConfig

	kcs              *kubernetes.Clientset
	cfg              *rest.Config
	coreFactory      *corectl.Factory
	batchFactory     *batchctl.Factory
	appsFactory      *appsctl.Factory
	cisFactory       *cisoperatorctl.Factory
	apply            apply.Apply
	monitoringClient v1monitoringclient.MonitoringV1Interface

	mu              *sync.Mutex
	currentScanName string

	numTestsFailed   *prometheus.GaugeVec
	numScansComplete *prometheus.CounterVec
	numTestsSkipped  *prometheus.GaugeVec
	numTestsTotal    *prometheus.GaugeVec
	numTestsNA       *prometheus.GaugeVec
	numTestsPassed   *prometheus.GaugeVec
	numTestsWarn     *prometheus.GaugeVec

	scans                      cisoperatorctlv1.ClusterScanController
	jobs                       batchctlv1.JobController
	configmaps                 corectlv1.ConfigMapController
	configMapCache             corectlv1.ConfigMapCache
	services                   corectlv1.ServiceController
	pods                       corectlv1.PodController
	podCache                   corectlv1.PodCache
	daemonsets                 appsctlv1.DaemonSetController
	daemonsetCache             appsctlv1.DaemonSetCache
	securityScanJobTolerations []corev1.Toleration
}

func NewController(ctx context.Context, cfg *rest.Config, namespace, name string,
	imgConfig *cisoperatorapiv1.ScanImageConfig, securityScanJobTolerations []corev1.Toleration) (ctl *Controller, err error) {
	if cfg == nil {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	ctl = &Controller{
		Namespace:   namespace,
		Name:        name,
		ImageConfig: imgConfig,
		mu:          &sync.Mutex{},
	}

	ctl.kcs, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	ctl.cfg = cfg

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ctl.ClusterProvider, err = detectClusterProvider(ctx, clientset)
	if err != nil {
		return nil, err
	}
	logrus.Infof("ClusterProvider detected %v", ctl.ClusterProvider)

	ctl.KubernetesVersion, err = detectKubernetesVersion(ctx, clientset)
	if err != nil {
		return nil, err
	}
	logrus.Infof("KubernetesVersion detected %v", ctl.KubernetesVersion)

	ctl.apply, err = apply.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	ctl.cisFactory, err = cisoperatorctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building securityscan NewFactoryFromConfig: %s", err.Error())
	}

	ctl.batchFactory, err = batchctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building batch NewFactoryFromConfig: %s", err.Error())
	}

	ctl.coreFactory, err = corectl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building core NewFactoryFromConfig: %s", err.Error())
	}

	ctl.appsFactory, err = appsctl.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building apps NewFactoryFromConfig: %s", err.Error())
	}

	ctl.monitoringClient, err = v1monitoringclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building v1 monitoring client from config: %s", err.Error())
	}

	err = initializeMetrics(ctl)
	if err != nil {
		return nil, fmt.Errorf("Error registering CIS Metrics: %s", err.Error())
	}

	ctl.scans = ctl.cisFactory.Cis().V1().ClusterScan()
	ctl.jobs = ctl.batchFactory.Batch().V1().Job()
	ctl.configmaps = ctl.coreFactory.Core().V1().ConfigMap()
	ctl.configMapCache = ctl.coreFactory.Core().V1().ConfigMap().Cache()
	ctl.services = ctl.coreFactory.Core().V1().Service()
	ctl.pods = ctl.coreFactory.Core().V1().Pod()
	ctl.podCache = ctl.coreFactory.Core().V1().Pod().Cache()
	ctl.daemonsets = ctl.appsFactory.Apps().V1().DaemonSet()
	ctl.daemonsetCache = ctl.appsFactory.Apps().V1().DaemonSet().Cache()
	ctl.securityScanJobTolerations = securityScanJobTolerations
	return ctl, nil
}

func (c *Controller) Start(ctx context.Context, threads int, resync time.Duration) error {
	// register our handlers
	if err := c.handleJobs(ctx); err != nil {
		return err
	}
	if err := c.handlePods(ctx); err != nil {
		return err
	}
	if err := c.handleClusterScans(ctx); err != nil {
		return err
	}
	if err := c.handleScheduledClusterScans(ctx); err != nil {
		return err
	}
	if err := c.handleClusterScanMetrics(ctx); err != nil {
		return err
	}
	return start.All(ctx, threads, c.cisFactory, c.coreFactory, c.batchFactory)
}

func (c *Controller) registerCRD(ctx context.Context) error {
	factory, err := crd.NewFactoryFromClient(c.cfg)
	if err != nil {
		return err
	}

	var crds []crd.CRD
	for _, crdFn := range []func() (*crd.CRD, error){
		scan.ClusterScanCRD,
	} {
		crdef, err := crdFn()
		if err != nil {
			return err
		}
		crds = append(crds, *crdef)
	}
	return factory.BatchCreateCRDs(ctx, crds...).BatchWait()
}

func (c *Controller) refreshClusterKubernetesVersion(ctx context.Context) error {
	clusterK8sVersion, err := detectKubernetesVersion(ctx, c.kcs)
	if err != nil {
		return err
	}
	if !strings.EqualFold(clusterK8sVersion, c.KubernetesVersion) {
		c.KubernetesVersion = clusterK8sVersion
		logrus.Infof("New KubernetesVersion detected %v", c.KubernetesVersion)
	}
	return nil
}

func detectClusterProvider(ctx context.Context, k8sClient kubernetes.Interface) (string, error) {
	provider, err := detector.DetectProvider(ctx, k8sClient)
	if err != nil {
		return "", err
	}
	return provider, err
}

func detectKubernetesVersion(ctx context.Context, k8sClient kubernetes.Interface) (string, error) {
	v, err := k8sClient.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return v.GitVersion, nil
}

func initializeMetrics(ctl *Controller) error {
	ctl.numTestsFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_fail",
			Help: "Number of test failed in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsFailed); err != nil {
		return err
	}

	ctl.numScansComplete = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cis_scan_num_scans_complete",
			Help: "Number of CIS clusterscans completed, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numScansComplete); err != nil {
		return err
	}

	ctl.numTestsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_total",
			Help: "Total Number of tests run in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsTotal); err != nil {
		return err
	}

	ctl.numTestsPassed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_pass",
			Help: "Number of tests passing in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsPassed); err != nil {
		return err
	}

	ctl.numTestsSkipped = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_skipped",
			Help: "Number of test skipped in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsSkipped); err != nil {
		return err
	}

	ctl.numTestsNA = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_na",
			Help: "Number of tests not applicable in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsNA); err != nil {
		return err
	}

	ctl.numTestsWarn = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cis_scan_num_tests_warn",
			Help: "Number of tests having warn status in the CIS scans, partioned by scan_name, scan_profile_name",
		},
		[]string{
			// scan_name will be set to "manual" for on-demand manual scans and the actual name set for the scheduled scans
			"scan_name",
			// name of the clusterScanProfile used for scanning
			"scan_profile_name",
			"cluster_name",
		},
	)
	if err := prometheus.Register(ctl.numTestsWarn); err != nil {
		return err
	}

	return nil
}
