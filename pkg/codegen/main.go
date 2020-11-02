package main

import (
	"os"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/cis-operator/pkg/crds"
)

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/rancher/cis-operator/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"cis.cattle.io": {
				Types: []interface{}{
					v1.ClusterScan{},
					v1.ClusterScanProfile{},
					v1.ClusterScanReport{},
					v1.ClusterScanBenchmark{},
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
