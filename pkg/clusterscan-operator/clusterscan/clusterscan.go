package clusterscan

import (
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/schemas/openapi"
	cisoperatorapiv1 "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
)

func CRD() (*crd.CRD, error) {
	prototype := cisoperatorapiv1.NewClusterScan("", "", cisoperatorapiv1.ClusterScan{})
	schema, err := openapi.ToOpenAPIFromStruct(*prototype)
	if err != nil {
		return nil, err
	}
	return &crd.CRD{
		GVK:        prototype.GroupVersionKind(),
		PluralName: cisoperatorapiv1.ClusterScanResourceName,
		Status:     true,
		Schema:     schema,
		Categories: []string{"clusterscan-operator"},
	}, nil
}