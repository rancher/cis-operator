package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	condition "github.com/prachidamle/clusterscan-operator/pkg/condition"
)


const (
	ClusterProviderRKE = "rke"
	ClusterProviderEKS = "eks"
	ClusterProviderGKE = "gke"

	ClusterScanNS = "clusterscan-system"
	ClusterScanSA = "clusterscan-serviceaccount"
	ClusterScanConfigMap = "clusterscan-s-config-cm"
	ClusterScanPluginsConfigMap = "clusterscan-s-plugins-cm"
	ClusterScanService = "service-rancher-cis-benchmark"
	DefaultScanOutputFileName = "output.json"


	ClusterScanConditionCreated      = condition.Cond("Created")
	ClusterScanConditionRunCompleted  = condition.Cond("RunCompleted")
	ClusterScanConditionComplete = condition.Cond("Complete")
	ClusterScanConditionFailed   = condition.Cond("Failed")
	ClusterScanConditionAlerted = condition.Cond("Alerted")
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec"`
	Status ClusterScanStatus `yaml:"status" json:"status,omitempty"`
}

type ClusterScanSpec struct {
	// scan profile to use
	ScanProfileName string              `json:"scanProfileName,omitempty"`
	ScanSchedule    *ClusterScanSchedule `yaml:"scanSchedule,omitempty" json:"scanSchedule,omitempty"`
}

type ClusterScanSchedule struct {
	// Cron Expression for Schedule
	CronSchedule string `yaml:"cron_schedule" json:"cronSchedule,omitempty"`
	// Number of past scans to keep
	Retention int `yaml:"retention" json:"retention,omitempty"`
}

type ClusterScanStatus struct {
	Enabled          bool               `yaml:"enabled" json:"enabled,omitempty"`
	LastRunTimestamp string             `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	Summary          *ClusterScanSummary `json:"summary,omitempty"`
	//Follow K8sStatus conventions from #2dotx
	//Wrangler: Status  handlers  or generating status handler (useful for CIS V2)
	//Examples: SystemUpgradeController or helmController
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

type ClusterScanSummary struct {
	Total         int `json:"total"`
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	Skip          int `json:"skip"`
	NotApplicable int `json:"notApplicable"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScanProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterScanProfileSpec `json:"spec"`
}

type ClusterScanProfileSpec struct {
	ClusterProvider string `json:"clusterProvider,omitempty"`

	BenchmarkVersion string `json:"benchmarkVersion,omitempty"`

	SkipTests []string `json:"skipTests,omitempty"`

	MinKubernetesVersion string `json:"minKubernetesVersion,omitempty"`

	MaxKubernetesVersion string `json:"maxKubernetesVersion,omitempty"`

	//RENAME
	ConfigMap string `json:"configMap,omitempty"`
	//RENAME
	ConfigMapNamespace string `json:"configMapNamespace,omitempty"`

	//UserSkipSet []string `json:"userSkipSet"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScanReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterScanReportSpec `json:"spec"`
}

type ClusterScanReportSpec struct {
	BenchmarkVersion string `json:"benchmarkVersion,omitempty"`
	LastRunTimestamp string             `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
	Total         int `json:"total"`
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	Skip          int `json:"skip"`
	reportJSON string `json:"reportJSON"`
}
