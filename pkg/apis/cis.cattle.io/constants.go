package cis

const (
	// LabelController is the name of the cis controller.
	LabelController = GroupName + `/controller`

	// LabelNode is the node being upgraded.
	LabelProfile = GroupName + `/clusterscanprofile`

	// LabelPlan is the plan being applied.
	LabelClusterScan = GroupName + `/scan`

	SonobuoyCompletionAnnotation = "field.cattle.io/sonobuoyDone"
)
