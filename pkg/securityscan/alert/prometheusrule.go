package alert

import (
	"bytes"
	"fmt"
	"text/template"

	meta1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	k8Yaml "k8s.io/apimachinery/pkg/util/yaml"

	cisoperatorapiv1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/name"
)

const templateName = "prometheusrule.template"
const templatePath = "./pkg/securityscan/alert/templates/prometheusrule.template"

func NewPrometheusRule(clusterscan *cisoperatorapiv1.ClusterScan, clusterscanprofile *cisoperatorapiv1.ClusterScanProfile, imageConfig *cisoperatorapiv1.ScanImageConfig) (*monitoringv1.PrometheusRule, error) {
	configdata := map[string]interface{}{
		"namespace":       cisoperatorapiv1.ClusterScanNS,
		"name":            name.SafeConcatName("rancher-cis-alerts", clusterscan.Name),
		"severity":        imageConfig.AlertSeverity,
		"scanName":        clusterscan.Name,
		"scanProfileName": clusterscanprofile.Name,
		"alertOnFailure":  clusterscan.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnFailure,
		"alertOnComplete": clusterscan.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnComplete,
		"failOnWarn":      clusterscan.Spec.ScoreWarning == cisoperatorapiv1.ClusterScanFailOnWarning,
	}
	scanAlertRule, err := generatePrometheusRule(clusterscan, templateName, templatePath, configdata)
	if err != nil {
		return scanAlertRule, err
	}

	return scanAlertRule, nil
}

func generatePrometheusRule(clusterscan *cisoperatorapiv1.ClusterScan, templateName string, templateFile string, data map[string]interface{}) (*monitoringv1.PrometheusRule, error) {
	scanAlertRule := &monitoringv1.PrometheusRule{}
	obj, err := parseTemplate(clusterscan, templateName, templateFile, data)
	if err != nil {
		return nil, fmt.Errorf("Error parsing the template %v", err)
	}

	if err := obj.Decode(&scanAlertRule); err != nil {
		return nil, fmt.Errorf("Error decoding to template %v", err)
	}

	ownerRef := meta1.OwnerReference{
		APIVersion: "cis.cattle.io/v1",
		Kind:       "ClusterScan",
		Name:       clusterscan.Name,
		UID:        clusterscan.GetUID(),
	}
	scanAlertRule.ObjectMeta.OwnerReferences = append(scanAlertRule.ObjectMeta.OwnerReferences, ownerRef)

	return scanAlertRule, nil
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
