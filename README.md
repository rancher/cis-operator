# cis-operator

This is an operator that can run on a given Kubernetes cluster and provide ability to run security scans
as per the CIS benchmarks, on the cluster.

## Building

`make`


## Running
1. Install the custom resource definitions:
- `kubectl apply -f crds/`
2. Install the operator
`./bin/cis-operator`

## License
Copyright (c) 2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
