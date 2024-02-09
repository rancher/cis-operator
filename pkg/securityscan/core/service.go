package core

import (
	_ "embed" // nolint

	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"

	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
)

//go:embed templates/service.template
var serviceTemplate string

func NewService(clusterscan *cisoperatorapiv1.ClusterScan, _ *cisoperatorapiv1.ClusterScanProfile, _ string) (service *corev1.Service, err error) {

	servicedata := map[string]interface{}{
		"namespace": cisoperatorapiv1.ClusterScanNS,
		"name":      cisoperatorapiv1.ClusterScanService,
		"runName":   name.SafeConcatName("security-scan-runner", clusterscan.Name),
		"appName":   "rancher-cis-benchmark",
	}
	service, err = generateService(clusterscan, "service.template", serviceTemplate, servicedata)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func generateService(clusterscan *cisoperatorapiv1.ClusterScan, templateName string, templContent string, data map[string]interface{}) (*corev1.Service, error) {
	service := &corev1.Service{}

	obj, err := parseTemplate(clusterscan, templateName, templContent, data)
	if err != nil {
		return nil, err
	}

	if err := obj.Decode(&service); err != nil {
		return nil, err
	}
	return service, nil
}
