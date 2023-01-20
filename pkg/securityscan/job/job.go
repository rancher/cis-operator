package job

import (
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	wcorev1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"

	cisoperatorapi "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io"
	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/cis-operator/pkg/condition"
)

const (
	defaultTerminationGracePeriodSeconds = int64(0)
	defaultBackoffLimit                  = int32(0)
	defaultTTLSecondsAfterFinished       = int32(0)
)

var (
	ConditionComplete = condition.Cond(batchv1.JobComplete)
	ConditionFailed   = condition.Cond(batchv1.JobFailed)

	backoffLimit = readFromEnv("CIS_JOB_BACKOFF_LIMIT", defaultBackoffLimit)

	TerminationGracePeriodSeconds = func(defaultValue int64) int64 {
		return defaultValue
	}(defaultTerminationGracePeriodSeconds)

	ttlSecondsAfterFinished = readFromEnv("CIS_JOB_TTL_SECONDS_AFTER_FINISH", defaultTTLSecondsAfterFinished)
)

func readFromEnv(key string, defaultValue int32) int32 {
	if str, ok := os.LookupEnv(key); ok {
		i, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			logrus.Errorf("failed to parse $%s: %v", key, err)
			return defaultValue
		}
		return int32(i)
	}
	return defaultValue
}

func New(clusterscan *cisoperatorapiv1.ClusterScan, clusterscanprofile *cisoperatorapiv1.ClusterScanProfile, clusterscanbenchmark *cisoperatorapiv1.ClusterScanBenchmark,
	controllerName string, imageConfig *cisoperatorapiv1.ScanImageConfig, configmapsClient wcorev1.ConfigMapController, tolerations []corev1.Toleration) *batchv1.Job {
	privileged := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.SafeConcatName("security-scan-runner", clusterscan.Name),
			Namespace:   cisoperatorapiv1.ClusterScanNS,
			Annotations: labels.Set{},
			Labels: labels.Set{
				cisoperatorapi.LabelController:  controllerName,
				cisoperatorapi.LabelProfile:     clusterscan.Spec.ScanProfileName,
				cisoperatorapi.LabelClusterScan: clusterscan.Name,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "cis.cattle.io/v1",
				Kind:       "ClusterScan",
				Name:       clusterscan.Name,
				UID:        clusterscan.GetUID(),
			}},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels.Set{
						"app.kubernetes.io/name":        "rancher-cis-benchmark",
						"app.kubernetes.io/instance":    name.SafeConcatName("security-scan-runner", clusterscan.Name),
						"run":                           "sonobuoy-master",
						cisoperatorapi.LabelController:  controllerName,
						cisoperatorapi.LabelProfile:     clusterscan.Spec.ScanProfileName,
						cisoperatorapi.LabelClusterScan: clusterscan.Name,
					},
				},
				Spec: corev1.PodSpec{
					HostPID:                       true,
					HostIPC:                       true,
					ServiceAccountName:            cisoperatorapiv1.ClusterScanSA,
					TerminationGracePeriodSeconds: &TerminationGracePeriodSeconds,
					Tolerations:                   tolerations,
					NodeSelector: labels.Set{
						"kubernetes.io/os": "linux",
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{
						Name: `s-config-volume`,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: name.SafeConcatName(cisoperatorapiv1.ClusterScanConfigMap, clusterscan.Name),
								},
							},
						},
					}, {
						Name: `s-plugins-volume`,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: name.SafeConcatName(cisoperatorapiv1.ClusterScanPluginsConfigMap, clusterscan.Name),
								},
							},
						},
					}, {
						Name: `output-volume`,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}, {
						Name: `rke2-root`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/var/lib/rancher`,
							},
						},
					}, {
						Name: `rke2-root-config`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/etc/rancher`,
							},
						},
					}, {
						Name: `rke2-cni`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/etc/cni/net.d`,
							},
						},
					}, {
						Name: `etc-passwd`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/etc/passwd`,
							},
						},
					}, {
						Name: `etc-group`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/etc/group`,
							},
						},
					}, {
						Name: `var-log`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/var/log`,
							},
						},
					}, {
						Name: `run-log`,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: `/run/log`,
							},
						},
					},
					},
					Containers: []corev1.Container{{
						Name:            `rancher-cis-benchmark`,
						Image:           imageConfig.SecurityScanImage + ":" + imageConfig.SecurityScanImageTag,
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privileged,
						},
						Env: []corev1.EnvVar{{
							Name:  `OVERRIDE_BENCHMARK_VERSION`,
							Value: clusterscanprofile.Spec.BenchmarkVersion,
						}, {
							Name:  `SONOBUOY_NS`,
							Value: cisoperatorapiv1.ClusterScanNS,
						}, {
							Name: `SONOBUOY_POD_NAME`,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: `metadata.name`,
								},
							},
						}, {
							Name:  `SONOBUOY_ADVERTISE_IP`,
							Value: `cisscan-rancher-cis-benchmark`,
						}, {
							Name:  `OUTPUT_CONFIGMAPNAME`,
							Value: strings.Join([]string{`cisscan-output-for`, clusterscan.Name}, "-"),
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      `s-config-volume`,
							MountPath: `/etc/sonobuoy`,
						}, {
							Name:      `s-plugins-volume`,
							MountPath: `/plugins.d`,
						}, {
							Name:      `output-volume`,
							MountPath: `/tmp/sonobuoy`,
						}, {
							Name:      `rke2-root`,
							MountPath: `/var/lib/rancher`,
						}, {
							Name:      `rke2-root-config`,
							MountPath: `/etc/rancher`,
						}, {
							Name:      `rke2-cni`,
							MountPath: `/etc/cni/net.d`,
						}, {
							Name:      `etc-passwd`,
							MountPath: `/etc/passwd`,
						}, {
							Name:      `etc-group`,
							MountPath: `/etc/group`,
						}, {
							Name:      `var-log`,
							MountPath: `/var/log/`,
						}, {
							Name:      `run-log`,
							MountPath: `/run/log/`,
						}},
					}},
				},
			},
		},
	}
	//add userskip configmap if present
	if clusterscanprofile.Spec.SkipTests != nil && len(clusterscanprofile.Spec.SkipTests) > 0 {
		skipVol := corev1.Volume{
			Name: `user-skip-info-volume`,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name.SafeConcatName(cisoperatorapiv1.ClusterScanUserSkipConfigMap, clusterscan.Name),
					},
				},
			},
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, skipVol)

		//volume mount
		skipVolMnt := corev1.VolumeMount{
			Name:      `user-skip-info-volume`,
			MountPath: `/etc/kbs/userskip`,
		}

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, skipVolMnt)
	}

	//add custom benchmark config and volume
	if clusterscanbenchmark.Spec.CustomBenchmarkConfigMapName != "" {
		//this env variable is read by kb-summarizer tool in security-scan image
		configDirEnv := corev1.EnvVar{
			Name:  `CONFIG_DIR`,
			Value: cisoperatorapiv1.CustomBenchmarkBaseDir,
		}
		job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, configDirEnv)

		//add the volume
		customcm, err := loadCustomBenchmarkConfigMap(clusterscanbenchmark, clusterscan, configmapsClient)
		if err != nil {
			logrus.Errorf("Error loading custom CustomBenchmarkConfigMap %v %v", clusterscanbenchmark.Spec.CustomBenchmarkConfigMapNamespace, clusterscanbenchmark.Spec.CustomBenchmarkConfigMapName)
			return job
		}
		customVol := corev1.Volume{
			Name: `custom-benchmark-volume`,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: customcm.Name,
					},
				},
			},
		}
		for key := range customcm.Data {
			if key == "config.yaml" {
				customVol.VolumeSource.ConfigMap.Items = append(customVol.VolumeSource.ConfigMap.Items, corev1.KeyToPath{Key: key, Path: key})
			} else {
				customVol.VolumeSource.ConfigMap.Items = append(customVol.VolumeSource.ConfigMap.Items, corev1.KeyToPath{Key: key, Path: clusterscanbenchmark.Name + "/" + key})
			}
		}
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, customVol)
		//volume mount
		customVolMnt := corev1.VolumeMount{
			Name:      `custom-benchmark-volume`,
			MountPath: cisoperatorapiv1.CustomBenchmarkBaseDir,
		}
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, customVolMnt)
	}

	return job
}

func loadCustomBenchmarkConfigMap(benchmark *cisoperatorapiv1.ClusterScanBenchmark, clusterscan *cisoperatorapiv1.ClusterScan, configmapsClient wcorev1.ConfigMapController) (*corev1.ConfigMap, error) {
	if benchmark.Spec.CustomBenchmarkConfigMapName == "" {
		return nil, nil
	}
	if benchmark.Spec.CustomBenchmarkConfigMapNamespace == cisoperatorapiv1.ClusterScanNS {
		return configmapsClient.Get(cisoperatorapiv1.ClusterScanNS, benchmark.Spec.CustomBenchmarkConfigMapName, metav1.GetOptions{})
	}
	//get copy of the configmap in ClusterScanNS created while creating plugin configmap
	cmName := name.SafeConcatName(cisoperatorapiv1.CustomBenchmarkConfigMap, clusterscan.Name)
	configmapCopy, err := configmapsClient.Get(cisoperatorapiv1.ClusterScanNS, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return configmapCopy, nil
}
