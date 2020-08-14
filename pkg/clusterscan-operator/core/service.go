package core

import (
	corev1 "k8s.io/api/core/v1"
	"github.com/rancher/wrangler/pkg/name"

	cisoperatorapiv1 "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
)

func NewService(clusterscan *cisoperatorapiv1.ClusterScan, clusterscanprofile *cisoperatorapiv1.ClusterScanProfile, controllerName string) (service *corev1.Service, err error) {

	servicedata := map[string]interface{} {
		"namespace": cisoperatorapiv1.ClusterScanNS,
		"name": cisoperatorapiv1.ClusterScanService,
		"runName": name.SafeConcatName("security-scan-runner", clusterscan.Name),
		"appName": "rancher-cis-benchmark",
	}
	service, err = generateService(clusterscan, "service.template", "./pkg/clusterscan-operator/core/templates/service.template", servicedata)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func generateService(clusterscan *cisoperatorapiv1.ClusterScan, templateName string, templateFile string, data map[string]interface{}) (*corev1.Service, error) {
	service := &corev1.Service{}

	obj, err := parseTemplate(clusterscan, templateName, templateFile, data)
	if err != nil {
		return nil, err
	}

	if err := obj.Decode(&service); err != nil {
		return nil, err
	}
	return service, nil
}