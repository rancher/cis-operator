package securityscan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/genericcondition"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	cisctlv1 "github.com/rancher/cis-operator/pkg/generated/controllers/cis.cattle.io/v1"
	cisalert "github.com/rancher/cis-operator/pkg/securityscan/alert"
	ciscore "github.com/rancher/cis-operator/pkg/securityscan/core"
	cisjob "github.com/rancher/cis-operator/pkg/securityscan/job"
)

const (
	kubeBenchJobManifest    = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"namespace\": \"cisscan-system\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"metadata\": {\r\n            \"labels\": {\r\n               \"app\": \"kube-bench\"\r\n            }\r\n         },\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-etcd\",\r\n                        \"mountPath\": \"/var/lib/etcd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"usr-bin\",\r\n                        \"mountPath\": \"/usr/local/mount-from-host/bin\",\r\n                        \"readOnly\": true\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-etcd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/etcd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"usr-bin\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/usr/bin\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
	kubeBenchEKSJobManifest = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"name\": \"kube-bench\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\",\r\n                     \"node\",\r\n                     \"--benchmark\",\r\n                     \"eks-1.0\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\",\r\n                        \"readOnly\": true\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
	kubeBenchGKEJobManifest = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"name\": \"kube-bench\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\",\r\n                     \"--benchmark\",\r\n                     \"gke-1.0\",\r\n                     \"run\",\r\n                     \"--targets\",\r\n                     \"node,policies,managedservices\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\"\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\"\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\"\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
)

var SonobuoyMasterLabel = map[string]string{"run": "sonobuoy-master"}

func (c *Controller) handleClusterScans(ctx context.Context) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	jobs := c.batchFactory.Batch().V1().Job()
	configmaps := c.coreFactory.Core().V1().ConfigMap()
	services := c.coreFactory.Core().V1().Service()

	cisctlv1.RegisterClusterScanGeneratingHandler(ctx, scans, c.apply.WithCacheTypes(configmaps, services).WithGVK(jobs.GroupVersionKind()).WithDynamicLookup().WithNoDelete(), "", c.Name,
		func(obj *v1.ClusterScan, status v1.ClusterScanStatus) (objects []runtime.Object, _ v1.ClusterScanStatus, _ error) {
			if obj == nil || obj.DeletionTimestamp != nil {
				return objects, status, nil
			}

			logrus.Infof("ClusterScan GENERATING HANDLER: scan=%s/%s@%s, %v, status=%+v", obj.Namespace, obj.Name, obj.Spec.ScanProfileName, obj.ResourceVersion, status.LastRunTimestamp)

			if obj.Status.LastRunTimestamp == "" && !v1.ClusterScanConditionCreated.IsTrue(obj) {
				if !v1.ClusterScanConditionPending.IsTrue(obj) {
					v1.ClusterScanConditionPending.True(obj)
					v1.ClusterScanConditionPending.Message(obj, "ClusterScan run pending")
					c.setClusterScanStatusDisplay(obj)
					scans.Enqueue(obj.Name)
					return objects, obj.Status, nil
				}
				obj.Status.Conditions = []genericcondition.GenericCondition{}
				v1.ClusterScanConditionPending.True(obj)
				v1.ClusterScanConditionPending.Message(obj, "ClusterScan run pending")

				if err := c.isRunnerPodPresent(); err != nil {
					return objects, obj.Status, fmt.Errorf("Retrying ClusterScan %v since got error: %v ", obj.Name, err)
				}

				profile, err := c.getClusterScanProfile(obj)
				if err != nil {
					v1.ClusterScanConditionFailed.True(obj)
					message := fmt.Sprintf("Error validating ClusterScanProfile %v, error: %v", obj.Spec.ScanProfileName, err)
					v1.ClusterScanConditionFailed.Message(obj, message)
					logrus.Errorf(message)
					c.setClusterScanStatusDisplay(obj)
					return objects, obj.Status, nil
				}

				if err := c.validateScheduledScanSpec(obj); err != nil {
					v1.ClusterScanConditionFailed.True(obj)
					message := fmt.Sprintf("Error validating Schedule %v, error: %v", obj.Spec.ScheduledScanConfig.CronSchedule, err)
					v1.ClusterScanConditionFailed.Message(obj, message)
					logrus.Errorf(message)
					c.setClusterScanStatusDisplay(obj)
					return objects, obj.Status, nil
				}

				if obj.Spec.ScheduledScanConfig != nil && obj.Spec.ScheduledScanConfig.ScanAlertRule != nil {
					if obj.Status.ScanAlertingRuleName == "" {
						alertRule, err := cisalert.NewPrometheusRule(obj, profile, c.ImageConfig)
						if err != nil {
							v1.ClusterScanConditionReconciling.True(obj)
							return objects, obj.Status, fmt.Errorf("Error when trying to create a PrometheusRule: %v", err)
						}
						ruleCreated, err := c.monitoringClient.PrometheusRules(v1.ClusterScanNS).Create(ctx, alertRule, metav1.CreateOptions{})
						if err != nil {
							v1.ClusterScanConditionReconciling.True(obj)
							return objects, obj.Status, fmt.Errorf("Error when creating PrometheusRule: %v", err)
						}
						obj.Status.ScanAlertingRuleName = ruleCreated.Name
					}
				}
				if err := c.isRunnerPodPresent(); err != nil {
					return objects, obj.Status, fmt.Errorf("Retrying ClusterScan %v since got error: %v ", obj.Name, err)
				}
				//launch new on demand scan
				c.mu.Lock()
				defer c.mu.Unlock()
				logrus.Infof("Launching a new on demand Job for scan %v to run cis using profile %v", obj.Name, profile.Name)
				benchmark, err := c.getClusterScanBenchmark(profile)
				if err != nil {
					v1.ClusterScanConditionReconciling.True(obj)
					return objects, obj.Status, fmt.Errorf("Error when getting Benchmark: %v", err)
				}
				cmMap, err := ciscore.NewConfigMaps(obj, profile, benchmark, c.Name, c.ImageConfig, configmaps)
				if err != nil {
					v1.ClusterScanConditionReconciling.True(obj)
					return objects, obj.Status, fmt.Errorf("Error when creating ConfigMaps: %v", err)

				}
				service, err := ciscore.NewService(obj, profile, c.Name)
				if err != nil {
					v1.ClusterScanConditionReconciling.True(obj)
					return objects, obj.Status, fmt.Errorf("Error when creating Service: %v", err)
				}
				objects = append(objects, cisjob.New(obj, profile, c.Name, c.ImageConfig), cmMap["configcm"], cmMap["plugincm"], cmMap["skipConfigcm"], service)

				if v1.ClusterScanConditionFailed.IsTrue(obj) {
					//clear the earlier failed status
					v1.ClusterScanConditionFailed.False(obj)
				}
				obj.Status.LastRunTimestamp = time.Now().Round(time.Second).Format(time.RFC3339)
				obj.Status.LastRunScanProfileName = profile.Name
				v1.ClusterScanConditionCreated.True(obj)
				v1.ClusterScanConditionRunCompleted.Unknown(obj)
				v1.ClusterScanConditionRunCompleted.Message(obj, "Creating Job to run the CIS scan")
				c.setClusterScanStatusDisplay(obj)
				return objects, obj.Status, nil
			}
			return objects, obj.Status, nil
		},
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		},
	)
	return nil
}

func (c *Controller) getClusterScanProfile(scan *v1.ClusterScan) (*v1.ClusterScanProfile, error) {
	var profileName string
	var err error
	clusterscanprofiles := c.cisFactory.Cis().V1().ClusterScanProfile()

	if scan.Spec.ScanProfileName != "" {
		profileName = scan.Spec.ScanProfileName
	} else {
		//pick the default profile by checking the cluster provider
		profileName, err = c.getDefaultClusterScanProfile(c.ClusterProvider)
		if err != nil {
			return nil, err
		}
	}
	profile, err := clusterscanprofiles.Get(profileName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = c.validateClusterScanProfile(profile)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (c *Controller) getClusterScanBenchmark(profile *v1.ClusterScanProfile) (*v1.ClusterScanBenchmark, error) {
	clusterscanbmks := c.cisFactory.Cis().V1().ClusterScanBenchmark()
	return clusterscanbmks.Get(profile.Spec.BenchmarkVersion, metav1.GetOptions{})
}

func (c *Controller) getDefaultClusterScanProfile(clusterprovider string) (string, error) {
	var err error
	configmaps := c.coreFactory.Core().V1().ConfigMap()
	cm, err := configmaps.Cache().Get(v1.ClusterScanNS, v1.DefaultClusterScanProfileConfigMap)
	if err != nil {
		return "", fmt.Errorf("Configmap to load default ClusterScanProfiles not found: %v", err)
	}
	profileName, ok := cm.Data[clusterprovider]
	if !ok {
		profileName = cm.Data["default"]
	}
	return profileName, nil
}

func (c Controller) validateClusterScanProfile(profile *v1.ClusterScanProfile) error {
	// validate benchmarkVersion is valid and is applicable to this cluster
	clusterscanbmks := c.cisFactory.Cis().V1().ClusterScanBenchmark()
	benchmark, err := clusterscanbmks.Get(profile.Spec.BenchmarkVersion, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// validate benchmark's provider matches the cluster
	if benchmark.Spec.ClusterProvider != "" {
		if !strings.EqualFold(benchmark.Spec.ClusterProvider, c.ClusterProvider) {
			return fmt.Errorf("ClusterScanProfile %v is not valid for this cluster's provider type %v", profile.Name, c.ClusterProvider)
		}
	}

	// validate cluster's k8s version matches the benchmark's k8s version range
	clusterK8sToMatch, err := semver.Make(c.KubernetesVersion[1:])
	if err != nil {
		return fmt.Errorf("Cluster's k8sVersion is not sem-ver %s %v", c.KubernetesVersion, err)
	}
	var k8sRange string
	if benchmark.Spec.MinKubernetesVersion != "" {
		k8sRange = ">=" + benchmark.Spec.MinKubernetesVersion
	}
	if benchmark.Spec.MaxKubernetesVersion != "" {
		k8sRange = k8sRange + " <=" + benchmark.Spec.MaxKubernetesVersion
	}
	if k8sRange != "" {
		benchmarkK8sRange, err := semver.ParseRange(k8sRange)
		if err != nil {
			return fmt.Errorf("Range for Benchmark %s not sem-ver %v, error: %v", benchmark.Name, k8sRange, err)
		}
		if !benchmarkK8sRange(clusterK8sToMatch) {
			return fmt.Errorf("Kubernetes version mismatch, ClusterScanProfile %v is not valid for this cluster's K8s version %v", profile.Name, c.KubernetesVersion)
		}
	}

	return nil
}

func (c Controller) isRunnerPodPresent() error {
	v2Pods, err := c.listRunnerPods(v1.ClusterScanNS)
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}
	if v2Pods != 0 {
		return fmt.Errorf("A rancher-cis-benchmark runner pod is already running")
	}

	v1Pods, err := c.listRunnerPods(v1.CISV1NS)
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}
	if v1Pods != 0 {
		return fmt.Errorf("A CIS v1 rancher-cis-benchmark runner pod is already running")
	}

	return nil
}

func (c Controller) listRunnerPods(namespace string) (int, error) {
	pods := c.coreFactory.Core().V1().Pod()
	podList, err := pods.Cache().List(namespace, labels.Set(SonobuoyMasterLabel).AsSelector())
	if err != nil {
		return 0, fmt.Errorf("error listing pods: %v", err)
	}
	return len(podList), nil
}

func (c Controller) setClusterScanStatusDisplay(scan *v1.ClusterScan) {
	errorState := "error"
	failedState := "fail"
	passedState := "pass"
	message := ""

	failed := false
	completed := false
	runCompleted := false
	pending := false
	running := false

	if v1.ClusterScanConditionPending.IsTrue(scan) {
		pending = true
	}
	if v1.ClusterScanConditionRunCompleted.IsUnknown(scan) {
		running = true
	}
	if v1.ClusterScanConditionRunCompleted.IsTrue(scan) {
		runCompleted = true
	}
	if v1.ClusterScanConditionFailed.IsTrue(scan) {
		message = v1.ClusterScanConditionFailed.GetMessage(scan)
		failed = true
	}
	if v1.ClusterScanConditionComplete.IsTrue(scan) {
		completed = true
	}

	display := &v1.ClusterScanStatusDisplay{}
	scan.Status.Display = display
	if pending {
		display.State = "pending"
		display.Message = "Scan is Pending, Waiting for another scan to finish"
		display.Transitioning = true
		display.Error = false
	}
	if running {
		display.State = "running"
		display.Message = "Scan is now running"
		display.Transitioning = true
		display.Error = false
	}
	if runCompleted {
		display.State = "reporting"
		display.Message = "ClusterScan scan finished, reporting the results"
		display.Transitioning = true
		display.Error = false
	}
	if failed {
		display.State = errorState
		display.Message = message
		display.Error = true
		return
	}
	if completed {
		summary := scan.Status.Summary
		if summary == nil {
			display.State = errorState
			display.Error = true
			display.Message = "ClusterScan complete, failed to generate report"
			return
		}
		if summary.Fail > 0 {
			display.State = failedState
			display.Message = "ClusterScan complete, there are some test failures, please check the ClusterScanReport"
			display.Error = true
		} else {
			if summary.Warn > 0 && scan.Spec.ScoreWarning == v1.ClusterScanFailOnWarning {
				display.State = failedState
				display.Message = "ClusterScan complete, warnings have been generated for some manual tests, please check the ClusterScanReport"
				display.Error = true
			} else {
				display.State = passedState
				display.Error = false
			}
		}
		display.Transitioning = false
	}
}
