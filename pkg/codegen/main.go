package main

import (
	"os"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"

	v1 "github.com/rancher/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
	"github.com/rancher/clusterscan-operator/pkg/crds"
)

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/clusterscan-operator/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"clusterscan-operator.cattle.io": {
				Types: []interface{}{
					v1.ClusterScan{},
					v1.ClusterScanProfile{},
					v1.ClusterScanReport{},
					v1.ScheduledScan{},
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
