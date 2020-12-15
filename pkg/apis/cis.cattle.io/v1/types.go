package v1

import (
	condition "github.com/rancher/cis-operator/pkg/condition"
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterProviderRKE = "rke"
	ClusterProviderEKS = "eks"
	ClusterProviderGKE = "gke"
	ClusterProviderAKS = "aks"
	ClusterProviderK3s = "k3s"

	CISV1NS                            = "security-scan"
	ClusterScanNS                      = "cis-operator-system"
	ClusterScanSA                      = "cis-serviceaccount"
	ClusterScanConfigMap               = "cis-s-config-cm"
	ClusterScanPluginsConfigMap        = "cis-s-plugins-cm"
	ClusterScanUserSkipConfigMap       = "cis-s-user-skip-cm"
	DefaultClusterScanProfileConfigMap = "default-clusterscanprofiles"
	ClusterScanService                 = "service-rancher-cis-benchmark"
	DefaultScanOutputFileName          = "output.json"
	DefaultRetention                   = 3
	DefaultCronSchedule                = "0 0 * * *"
	CustomBenchmarkBaseDir             = "/etc/kbs/custombenchmark/cfg"
	CustomBenchmarkConfigMap           = "cis-bmark-cm"

	ClusterScanConditionCreated      = condition.Cond("Created")
	ClusterScanConditionPending      = condition.Cond("Pending")
	ClusterScanConditionRunCompleted = condition.Cond("RunCompleted")
	ClusterScanConditionComplete     = condition.Cond("Complete")
	ClusterScanConditionFailed       = condition.Cond("Failed")
	ClusterScanConditionAlerted      = condition.Cond("Alerted")
	ClusterScanConditionReconciling  = condition.Cond("Reconciling")
	ClusterScanConditionStalled      = condition.Cond("Stalled")

	ClusterScanFailOnWarning = "fail"
	ClusterScanPassOnWarning = "pass"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec"`
	Status ClusterScanStatus `yaml:"status" json:"status,omitempty"`
}

type ClusterScanSpec struct {
	// scan profile to use
	ScanProfileName string `json:"scanProfileName,omitempty"`
	//config for scheduled scan
	ScheduledScanConfig *ScheduledScanConfig `yaml:"scheduled_scan_config" json:"scheduledScanConfig,omitempty"`
	// Specify if tests with "warn" output should be counted towards scan failure
	ScoreWarning string `yaml:"score_warning" json:"scoreWarning,omitempty"`
}

type ClusterScanStatus struct {
	Display                *ClusterScanStatusDisplay           `json:"display,omitempty"`
	LastRunTimestamp       string                              `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	LastRunScanProfileName string                              `json:"lastRunScanProfileName,omitempty"`
	Summary                *ClusterScanSummary                 `json:"summary,omitempty"`
	ObservedGeneration     int64                               `json:"observedGeneration"`
	Conditions             []genericcondition.GenericCondition `json:"conditions,omitempty"`
	NextScanAt             string                              `json:"NextScanAt"`
	ScanAlertingRuleName   string                              `json:"ScanAlertingRuleName"`
}

type ClusterScanStatusDisplay struct {
	State         string `json:"state"`
	Message       string `json:"message"`
	Error         bool   `json:"error"`
	Transitioning bool   `json:"transitioning"`
}

type ClusterScanSummary struct {
	Total         int `json:"total"`
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	Skip          int `json:"skip"`
	Warn          int `json:"warn"`
	NotApplicable int `json:"notApplicable"`
}

type ScheduledScanConfig struct {
	// Cron Expression for Schedule
	CronSchedule string `yaml:"cron_schedule" json:"cronSchedule,omitempty"`
	// Number of past scans to keep
	RetentionCount int `yaml:"retentionCount" json:"retentionCount,omitempty"`
	//configure the alerts to be sent out
	ScanAlertRule *ClusterScanAlertRule `json:"scanAlertRule,omitempty"`
}

type ClusterScanAlertRule struct {
	AlertOnComplete bool `json:"alertOnComplete,omitempty"`
	AlertOnFailure  bool `json:"alertOnFailure,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScanBenchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterScanBenchmarkSpec `json:"spec"`
}

type ClusterScanBenchmarkSpec struct {
	ClusterProvider      string `json:"clusterProvider,omitempty"`
	MinKubernetesVersion string `json:"minKubernetesVersion,omitempty"`
	MaxKubernetesVersion string `json:"maxKubernetesVersion,omitempty"`

	CustomBenchmarkConfigMapName      string `json:"customBenchmarkConfigMapName,omitempty"`
	CustomBenchmarkConfigMapNamespace string `json:"customBenchmarkConfigMapNamespace,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScanProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterScanProfileSpec `json:"spec"`
}

type ClusterScanProfileSpec struct {
	BenchmarkVersion string   `json:"benchmarkVersion,omitempty"`
	SkipTests        []string `json:"skipTests,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterScanReportSpec `json:"spec"`
}

type ClusterScanReportSpec struct {
	BenchmarkVersion string `json:"benchmarkVersion,omitempty"`
	LastRunTimestamp string `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	ReportJSON       string `json:"reportJSON"`
}

type ScanImageConfig struct {
	SecurityScanImage    string
	SecurityScanImageTag string
	SonobuoyImage        string
	SonobuoyImageTag     string
	AlertSeverity        string
	ClusterName          string
	AlertEnabled         bool
}
