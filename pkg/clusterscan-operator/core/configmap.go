package core

import(
	"text/template"
	"bytes"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"
	corev1 "k8s.io/api/core/v1"

	"github.com/rancher/wrangler/pkg/name"

	cisoperatorapiv1 "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
)

func NewConfigMaps(clusterscan *cisoperatorapiv1.ClusterScan, clusterscanprofile *cisoperatorapiv1.ClusterScanProfile, controllerName string) (configmaps []*corev1.ConfigMap, err error) {

	configdata := map[string]interface{} {
		"namespace": cisoperatorapiv1.ClusterScanNS,
		"name": cisoperatorapiv1.ClusterScanConfigMap,
		"runName": name.SafeConcatName("security-scan-runner", clusterscan.Name),
		"appName": "rancher-cis-benchmark",
		"advertiseAddress": cisoperatorapiv1.ClusterScanService,
		"sonobuoyImage": "rancher/sonobuoy-sonobuoy:v0.16.3",
		"sonobuoyVersion": "v0.16.3",
	}
	configcm, err := generateConfigMap(clusterscan, "cisscanConfig.template", "./pkg/clusterscan-operator/core/templates/cisscanConfig.template", configdata)
	if err != nil {
		return configmaps, err
	}
	plugindata := map[string]interface{} {
		"namespace": cisoperatorapiv1.ClusterScanNS,
		"name": cisoperatorapiv1.ClusterScanPluginsConfigMap,
		"runName": name.SafeConcatName("security-scan-runner", clusterscan.Name),
		"appName": "rancher-cis-benchmark",
		"serviceaccount": cisoperatorapiv1.ClusterScanSA,
		"securityScanImage": "rancher/security-scan:v0.1.14",
		"benchmarkVersion": clusterscanprofile.Spec.BenchmarkVersion,
	}
	plugincm, err := generateConfigMap(clusterscan, "pluginConfig.template", "./pkg/clusterscan-operator/core/templates/pluginConfig.template", plugindata)
	if err != nil {
		return configmaps, err
	}
	configmaps = append(configmaps, configcm, plugincm)
	return configmaps, nil
}

func generateConfigMap(clusterscan *cisoperatorapiv1.ClusterScan, templateName string, templateFile string, data map[string]interface{}) (*corev1.ConfigMap, error) {
	configcm := &corev1.ConfigMap{}

	obj, err := parseTemplate(clusterscan, templateName, templateFile, data)
	if err != nil {
		return nil, err
	}

	if err := obj.Decode(&configcm); err != nil {
		return nil, err
	}
	return configcm, nil
}

func parseTemplate(clusterscan *cisoperatorapiv1.ClusterScan, templateName string, templateFile string, data map[string]interface{}) (*k8Yaml.YAMLOrJSONDecoder, error) {
	cmTemplate, err := template.New(templateName).ParseFiles(templateFile)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	err = cmTemplate.Execute(&b, data)
	if err != nil {
		return nil, err
	}

	return k8Yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(b.String())), 1000), nil
}

