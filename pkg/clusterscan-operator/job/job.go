package job


import (
	"strings"
	"os"
	"strconv"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"github.com/sirupsen/logrus"

	"github.com/rancher/wrangler/pkg/name"

	"github.com/prachidamle/clusterscan-operator/pkg/condition"
	cisoperatorapiv1 "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
	cisoperatorapi "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io"
)

const (
	defaultTerminationGracePeriodSeconds            = int64(0)
	defaultBackoffLimit            = int32(0)
)

var (
	ConditionComplete = condition.Cond(batchv1.JobComplete)
	ConditionFailed   = condition.Cond(batchv1.JobFailed)

	BackoffLimit = func(defaultValue int32) int32 {
		if str, ok := os.LookupEnv("SYSTEM_UPGRADE_JOB_BACKOFF_LIMIT"); ok {
			if i, err := strconv.ParseInt(str, 10, 32); err != nil {
				logrus.Errorf("failed to parse $%s: %v", "SYSTEM_UPGRADE_JOB_BACKOFF_LIMIT", err)
			} else {
				return int32(i)
			}
		}
		return defaultValue
	}(defaultBackoffLimit)

	TerminationGracePeriodSeconds = func(defaultValue int64) int64 {
		return defaultValue
	}(defaultTerminationGracePeriodSeconds)
)

func New(clusterscan *cisoperatorapiv1.ClusterScan, clusterscanprofile *cisoperatorapiv1.ClusterScanProfile, controllerName string) *batchv1.Job {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.SafeConcatName("security-scan-runner", clusterscan.Name),
			Namespace: cisoperatorapiv1.ClusterScanNS,
			Annotations: labels.Set{},
			Labels: labels.Set{
				cisoperatorapi.LabelController: controllerName,
				cisoperatorapi.LabelProfile:       clusterscan.Spec.ScanProfileName,
				cisoperatorapi.LabelClusterScan:       clusterscan.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels.Set{
						"app.kubernetes.io/name": "rancher-cis-benchmark",
						"app.kubernetes.io/instance": name.SafeConcatName("security-scan-runner", clusterscan.Name),
						"run": "sonobuoy-master",
						cisoperatorapi.LabelController: controllerName,
						cisoperatorapi.LabelProfile:       clusterscan.Spec.ScanProfileName,
						cisoperatorapi.LabelClusterScan:       clusterscan.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cisoperatorapiv1.ClusterScanSA,
					TerminationGracePeriodSeconds: &TerminationGracePeriodSeconds,
					Tolerations: append([]corev1.Toleration{{
						Operator: corev1.TolerationOpExists,
					}}),
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{
						Name: `s-config-volume`,
						VolumeSource: corev1.VolumeSource {
							ConfigMap: &corev1.ConfigMapVolumeSource {
								LocalObjectReference: corev1.LocalObjectReference {
									Name: cisoperatorapiv1.ClusterScanConfigMap,
								},
							},
						},
					}, {
						Name: `s-plugins-volume`,
						VolumeSource: corev1.VolumeSource {
							ConfigMap: &corev1.ConfigMapVolumeSource {
								LocalObjectReference: corev1.LocalObjectReference {
									Name: cisoperatorapiv1.ClusterScanPluginsConfigMap,
								},
							},
						},
					}, {
						Name: `output-volume`,
						VolumeSource: corev1.VolumeSource {
							EmptyDir: &corev1.EmptyDirVolumeSource {
							},
						},
					}},
					Containers: []corev1.Container{{
						Name: `rancher-cis-benchmark`,
						Image: `rancher/security-scan:v0.1.14`,
						ImagePullPolicy: corev1.PullAlways,
						Env: []corev1.EnvVar{{
							Name: `OVERRIDE_BENCHMARK_VERSION`,
							Value: clusterscanprofile.Spec.BenchmarkVersion,
						}, {
							Name: `SONOBUOY_NS`,
							Value: cisoperatorapiv1.ClusterScanNS,
						}, {
							Name: `SONOBUOY_POD_NAME`,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: `metadata.name`,
								},
							},
						}, {
							Name: `SONOBUOY_ADVERTISE_IP`,
							Value: `cisscan-rancher-cis-benchmark`,
						}, {
							Name: `OUTPUT_CONFIGMAPNAME`,
							Value: strings.Join([]string{`cisscan-output-for`,clusterscan.Name},"-"),
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Protocol: corev1.ProtocolTCP,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name: `s-config-volume`,
							MountPath: `/etc/sonobuoy`,
						}, {
							Name: `s-plugins-volume`,
							MountPath: `/plugins.d`,
						}, {
							Name: `output-volume`,
							MountPath: `/tmp/sonobuoy`,
						}},
					}},

				},
			},
		},
	}
	return job
}