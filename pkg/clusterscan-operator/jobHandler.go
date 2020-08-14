package clusterscan_operator

import(
	"context"
	"strings"
	"fmt"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/api/errors"

	batchctlv1 "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	"github.com/rancher/security-scan/pkg/kb-summarizer/report"

	"github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io/v1"
	cisoperatorapi "github.com/prachidamle/clusterscan-operator/pkg/apis/clusterscan-operator.cattle.io"
)


// job events (successful completions) should remove the job after validatinf Done annotation and Output CM
func (c *Controller) handleJobs(ctx context.Context) error {
	scans := c.cisFactory.Clusterscanoperator().V1().ClusterScan()
	jobs := c.batchFactory.Batch().V1().Job()
	configmaps := c.coreFactory.Core().V1().ConfigMap()

	jobs.OnChange(ctx, c.Name, func(key string, obj *batchv1.Job) (*batchv1.Job, error) {
		if obj == nil || obj.DeletionTimestamp != nil {
			return obj, nil
		}
		jobSelector := labels.SelectorFromSet(labels.Set{
			cisoperatorapi.LabelController: c.Name,
		})
		// avoid commandeering jobs from other controllers
		if obj.Labels == nil || !jobSelector.Matches(labels.Set(obj.Labels)) {
			return obj, nil
		}
		// identify the clusterscan object for this job
		scanName, ok := obj.Labels[cisoperatorapi.LabelClusterScan]
		if !ok {
			// malformed, just delete it and move on
			logrus.Errorf("malformed scan, deleting the job %v", obj.Name)
			return obj, deleteJob(jobs, obj, metav1.DeletePropagationBackground)
		}
		// get the scan being run
		scan, err := scans.Get("default", scanName, metav1.GetOptions{})
		switch {
			case errors.IsNotFound(err):
				// scan is gone, delete
				logrus.Errorf("scan gone, deleting the job %v", obj.Name)
				return obj, deleteJob(jobs, obj, metav1.DeletePropagationBackground)
			case err != nil:
				return obj, err
		}

		// if the scan has completed then delete the job
		if v1.ClusterScanConditionComplete.IsTrue(scan) {
			v1.ClusterScanConditionAlerted.Unknown(scan)
			logrus.Infof("Marking ClusterScanConditionAlerted for clusterscan: %v", scanName)
			//update scan
			_, err = scans.UpdateStatus(scan)
			if err != nil {
				return nil, fmt.Errorf("error updating condition of cluster scan object: %v", scanName)
			}
			return obj, deleteJob(jobs, obj, metav1.DeletePropagationBackground)
		}

		if v1.ClusterScanConditionRunCompleted.IsTrue(scan) {
			scancopy := scan.DeepCopy()
			if !v1.ClusterScanConditionFailed.IsTrue(scan) {
				//get the output configmap and create a report
				outputConfigName := strings.Join([]string{`cisscan-output-for`,scanName},"-")
				cm, err := configmaps.Cache().Get(obj.Namespace, outputConfigName)
				if err != nil {
					return nil, fmt.Errorf("cisScanHandler: Updated: error fetching configmap %v: %v", outputConfigName, err)
				}
				r, err := report.Get([]byte(cm.Data[v1.DefaultScanOutputFileName]))
				if err != nil {
					return nil, fmt.Errorf("cisScanHandler: Updated: error getting report from configmap %v: %v", outputConfigName, err)
				}
				if r == nil {
					return nil, fmt.Errorf("cisScanHandler: Updated: error: got empty report from configmap %v", outputConfigName)
				}
				cisScanStatus := &v1.ClusterScanSummary {
					Total:         r.Total,
					Pass:          r.Pass,
					Fail:          r.Fail,
					Skip:          r.Skip,
					NotApplicable: r.NotApplicable,
				}
				scancopy.Status.Summary = cisScanStatus
			}
			v1.ClusterScanConditionComplete.True(scancopy)
			//update scan
			_, err = scans.UpdateStatus(scancopy)
			if err != nil {
				return nil, fmt.Errorf("error updating condition of clusterscan object: %v", scanName)
			}
			logrus.Infof("Marking ClusterScanConditionComplete for clusterscan: %v", scanName)
			jobs.Enqueue(obj.Namespace, obj.Name)
		}
		return obj, nil
	})
	return nil
}

func deleteJob(jobController batchctlv1.JobController, job *batchv1.Job, deletionPropagation metav1.DeletionPropagation) error {
	return jobController.Delete(job.Namespace, job.Name, &metav1.DeleteOptions{PropagationPolicy: &deletionPropagation})
}