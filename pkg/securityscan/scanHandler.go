package securityscan

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/rancher/wrangler/pkg/generic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	cisctlv1 "github.com/rancher/cis-operator/pkg/generated/controllers/cis.cattle.io/v1"
	ciscore "github.com/rancher/cis-operator/pkg/securityscan/core"
	cisjob "github.com/rancher/cis-operator/pkg/securityscan/job"
)

const (
	kubeBenchJobManifest    = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"namespace\": \"cisscan-system\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"metadata\": {\r\n            \"labels\": {\r\n               \"app\": \"kube-bench\"\r\n            }\r\n         },\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-etcd\",\r\n                        \"mountPath\": \"/var/lib/etcd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"usr-bin\",\r\n                        \"mountPath\": \"/usr/local/mount-from-host/bin\",\r\n                        \"readOnly\": true\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-etcd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/etcd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"usr-bin\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/usr/bin\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
	kubeBenchEKSJobManifest = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"name\": \"kube-bench\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\",\r\n                     \"node\",\r\n                     \"--benchmark\",\r\n                     \"eks-1.0\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\",\r\n                        \"readOnly\": true\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\",\r\n                        \"readOnly\": true\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
	kubeBenchGKEJobManifest = "{\r\n   \"apiVersion\": \"batch/v1\",\r\n   \"kind\": \"Job\",\r\n   \"metadata\": {\r\n      \"name\": \"kube-bench\"\r\n   },\r\n   \"spec\": {\r\n      \"template\": {\r\n         \"spec\": {\r\n            \"hostPID\": true,\r\n            \"containers\": [\r\n               {\r\n                  \"name\": \"kube-bench\",\r\n                  \"image\": \"aquasec/kube-bench:latest\",\r\n                  \"command\": [\r\n                     \"kube-bench\",\r\n                     \"--benchmark\",\r\n                     \"gke-1.0\",\r\n                     \"run\",\r\n                     \"--targets\",\r\n                     \"node,policies,managedservices\"\r\n                  ],\r\n                  \"volumeMounts\": [\r\n                     {\r\n                        \"name\": \"var-lib-kubelet\",\r\n                        \"mountPath\": \"/var/lib/kubelet\"\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-systemd\",\r\n                        \"mountPath\": \"/etc/systemd\"\r\n                     },\r\n                     {\r\n                        \"name\": \"etc-kubernetes\",\r\n                        \"mountPath\": \"/etc/kubernetes\"\r\n                     }\r\n                  ]\r\n               }\r\n            ],\r\n            \"restartPolicy\": \"Never\",\r\n            \"volumes\": [\r\n               {\r\n                  \"name\": \"var-lib-kubelet\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/var/lib/kubelet\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-systemd\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/systemd\"\r\n                  }\r\n               },\r\n               {\r\n                  \"name\": \"etc-kubernetes\",\r\n                  \"hostPath\": {\r\n                     \"path\": \"/etc/kubernetes\"\r\n                  }\r\n               }\r\n            ]\r\n         }\r\n      }\r\n   }\r\n}"
)

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
			if obj.Status.LastRunTimestamp == "" {
				//new on demand scan
				profile, err := c.getClusterScanProfile(obj)
				if err != nil {
					return objects, status, fmt.Errorf("Error %v loading v1.ClusterScanProfile for name %v", obj.Spec.ScanProfileName, err)
				}
				logrus.Infof("Launching a new on demand Job to run cis using profile %v", profile.Name)
				configmaps, err := ciscore.NewConfigMaps(obj, profile, c.Name, c.ImageConfig)
				if err != nil {
					return objects, status, fmt.Errorf("Error when creating ConfigMaps: %v", err)
				}
				service, err := ciscore.NewService(obj, profile, c.Name)
				if err != nil {
					return objects, status, fmt.Errorf("Error when creating Service: %v", err)
				}

				objects = append(objects, cisjob.New(obj, profile, c.Name, c.ImageConfig), configmaps[0], configmaps[1], configmaps[2], service)

				obj.Status.LastRunTimestamp = time.Now().String()
				obj.Status.Enabled = true
				v1.ClusterScanConditionCreated.True(obj)
				v1.ClusterScanConditionRunCompleted.Unknown(obj)

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
	clusterscanprofiles := c.cisFactory.Cis().V1().ClusterScanProfile()

	if scan.Spec.ScanProfileName != "" {
		profileName = scan.Spec.ScanProfileName
	} else {
		//pick the default profile by checking the cluster provider
		profileName = c.getDefaultClusterScanProfile(c.ClusterProvider)
	}
	profile, err := clusterscanprofiles.Get(profileName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (c Controller) getDefaultClusterScanProfile(clusterprovider string) string {
	var profileName string
	//load clusterScan
	switch clusterprovider {
	case v1.ClusterProviderRKE:
		profileName = "rke-profile-permissive"
	case v1.ClusterProviderEKS:
		profileName = "eks-profile"
	case v1.ClusterProviderGKE:
		profileName = "gke-profile"
	}
	return profileName
}

// https://github.com/kubernetes-sigs/cli-utils/tree/master/pkg/kstatus
// Reconciling and Stalled conditions are present and with a value of true whenever something unusual happens.
func (c Controller) setReconcilingCondition(scan *v1.ClusterScan, originalErr error) (*v1.ClusterScan, error) {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	v1.ClusterScanConditionReconciling.True(scan)
	if updScan, err := scans.UpdateStatus(scan); err != nil {
		return updScan, errors.New(originalErr.Error() + err.Error())
	}
	return scan, originalErr
}
