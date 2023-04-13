package crds

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	cisoperator "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/crd"
	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io" //using init
	"github.com/rancher/wrangler/pkg/yaml"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func WriteCRD() error {
	for _, crdDef := range List() {
		bCrd, err := crdDef.ToCustomResourceDefinition()
		if err != nil {
			return err
		}
		newObj, _ := bCrd.(*unstructured.Unstructured)
		var crd apiextv1.CustomResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(newObj.Object, &crd); err != nil {
			return err
		}

		if crd.Name == "clusterscans.cis.cattle.io" {
			customizeClusterScan(&crd)
		}
		yamlBytes, err := yaml.Export(&crd)
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("./crds/%s.yaml", strings.ToLower(crd.Spec.Names.Kind))
		err = ioutil.WriteFile(filename, yamlBytes, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&cisoperator.ClusterScan{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("ClusterScanProfile", ".status.lastRunScanProfileName").
				WithColumn("Total", ".status.summary.total").
				WithColumn("Pass", ".status.summary.pass").
				WithColumn("Fail", ".status.summary.fail").
				WithColumn("Skip", ".status.summary.skip").
				WithColumn("Warn", ".status.summary.warn").
				WithColumn("Not Applicable", ".status.summary.notApplicable").
				WithColumn("LastRunTimestamp", ".status.lastRunTimestamp").
				WithColumn("CronSchedule", ".spec.scheduledScanConfig.cronSchedule")
		}),
		newCRD(&cisoperator.ClusterScanProfile{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("BenchmarkVersion", ".spec.benchmarkVersion")
		}),
		newCRD(&cisoperator.ClusterScanReport{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("LastRunTimestamp", ".spec.lastRunTimestamp").
				WithColumn("BenchmarkVersion", ".spec.benchmarkVersion")
		}),
		newCRD(&cisoperator.ClusterScanBenchmark{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("ClusterProvider", ".spec.clusterProvider").
				WithColumn("MinKubernetesVersion", ".spec.minKubernetesVersion").
				WithColumn("MaxKubernetesVersion", ".spec.maxKubernetesVersion").
				WithColumn("customBenchmarkConfigMapName", ".spec.customBenchmarkConfigMapName").
				WithColumn("customBenchmarkConfigMapNamespace", ".spec.customBenchmarkConfigMapNamespace")
		}),
	}
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "cis.cattle.io",
			Version: "v1",
		},
		NonNamespace: true,
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}

func customizeClusterScan(clusterScan *apiextv1.CustomResourceDefinition) {
	properties := clusterScan.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties

	if len(properties) == 0 {
		return
	}

	spec := properties["spec"]
	scoreWarning := spec.Properties["scoreWarning"]
	passRaw, _ := json.Marshal(cisoperator.ClusterScanPassOnWarning)
	failRaw, _ := json.Marshal(cisoperator.ClusterScanFailOnWarning)
	scoreWarning.Enum = []apiextv1.JSON{{Raw: passRaw}, {Raw: failRaw}}
	spec.Properties["scoreWarning"] = scoreWarning
	properties["spec"] = spec
}
