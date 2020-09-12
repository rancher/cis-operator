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

	ClusterScanConditionCreated      = condition.Cond("Created")
	ClusterScanConditionPending      = condition.Cond("Pending")
	ClusterScanConditionRunCompleted = condition.Cond("RunCompleted")
	ClusterScanConditionComplete     = condition.Cond("Complete")
	ClusterScanConditionFailed       = condition.Cond("Failed")
	ClusterScanConditionAlerted      = condition.Cond("Alerted")
	ClusterScanConditionReconciling  = condition.Cond("Reconciling")
	ClusterScanConditionStalled      = condition.Cond("Stalled")
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
}

type ClusterScanStatus struct {
	Display                *ClusterScanStatusDisplay           `json:"display,omitempty"`
	LastRunTimestamp       string                              `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	LastRunScanProfileName string                              `json:"lastRunScanProfileName,omitempty"`
	Summary                *ClusterScanSummary                 `json:"summary,omitempty"`
	ObservedGeneration     int64                               `json:"observedGeneration"`
	Conditions             []genericcondition.GenericCondition `json:"conditions,omitempty"`
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
	NotApplicable int `json:"notApplicable"`
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
	CustomBenchmarkConfigMapNameSpace string `json:"customBenchmarkConfigMapNameSpace,omitempty"`
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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ScheduledScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScheduledScanSpec   `json:"spec"`
	Status ScheduledScanStatus `yaml:"status" json:"status,omitempty"`
}

type ScheduledScanSpec struct {
	// scan profile to use
	ScanProfileName string `json:"scanProfileName,omitempty"`
	// Cron Expression for Schedule
	CronSchedule string `yaml:"cron_schedule" json:"cronSchedule,omitempty"`
	// Number of past scans to keep
	RetentionCount int `yaml:"retentionCount" json:"retentionCount,omitempty"`
}

type ScheduledScanStatus struct {
	Enabled             bool                                `yaml:"enabled" json:"enabled,omitempty"`
	LastRunTimestamp    string                              `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	LastClusterScanName string                              `yaml:"last_cis_name" json:"lastClusterScanName"`
	ObservedGeneration  int64                               `json:"observedGeneration"`
	Conditions          []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type ScanImageConfig struct {
	SecurityScanImage    string
	SecurityScanImageTag string
	SonobuoyImage        string
	SonobuoyImageTag     string
}
