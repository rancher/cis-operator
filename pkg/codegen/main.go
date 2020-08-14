package main

import (
	"os"

	v1 "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
	"github.com/prachidamle/clusterscan-operator/pkg/crds"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
)

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/prachidamle/clusterscan-operator/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group {
			"clusterscan-operator.cattle.io": {
				Types: []interface{}{
					v1.ClusterScan{},
					v1.ClusterScanProfile{},
				},
				GenerateTypes: true,
			},
		},
	})

	err := crds.WriteCRD()
	if err != nil {
		panic(err)
	}
}
