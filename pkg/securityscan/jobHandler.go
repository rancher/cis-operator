package securityscan

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rancher/security-scan/pkg/kb-summarizer/report"
	reportLibrary "github.com/rancher/security-scan/pkg/kb-summarizer/report"
	batchctlv1 "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"

	"time"

	cisoperatorapi "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io"
	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/name"
)

var sonobuoyWorkerLabel = map[string]string{"sonobuoy-plugin": "rancher-kube-bench"}

// job events (successful completions) should remove the job after validatinf Done annotation and Output CM
func (c *Controller) handleJobs(ctx context.Context) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	reports := c.cisFactory.Cis().V1().ClusterScanReport()
	jobs := c.batchFactory.Batch().V1().Job()

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
		// identify the scan object for this job
		scanName, ok := obj.Labels[cisoperatorapi.LabelClusterScan]
		if !ok {
			// malformed, just delete it and move on
			logrus.Errorf("malformed scan, deleting the job %v", obj.Name)
			return obj, c.deleteJob(jobs, obj, metav1.DeletePropagationBackground)
		}
		// get the scan being run
		scan, err := scans.Get(scanName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			// scan is gone, delete
			logrus.Errorf("scan gone, deleting the job %v", obj.Name)
			return obj, c.deleteJob(jobs, obj, metav1.DeletePropagationBackground)
		case err != nil:
			return obj, err
		}

		// if the scan has completed then delete the job
		if v1.ClusterScanConditionComplete.IsTrue(scan) {
			if !v1.ClusterScanConditionFailed.IsTrue(scan) {
				logrus.Infof("Marking ClusterScanConditionAlerted for scan: %v", scanName)
				v1.ClusterScanConditionAlerted.Unknown(scan)
			}
			scan.Status.ObservedGeneration = scan.Generation
			c.setClusterScanStatusDisplay(scan)

			if scan.Spec.ScheduledScanConfig != nil && scan.Spec.ScheduledScanConfig.CronSchedule != "" {
				c.rescheduleScan(scan)
				c.purgeOldClusterScanReports(scan)
			}
			err := c.deleteJob(jobs, obj, metav1.DeletePropagationBackground)
			if err != nil {
				return obj, fmt.Errorf("error deleting job: %v", err)
			}
			err = c.ensureCleanup(scan)
			if err != nil {
				return obj, err
			}
			//update scan
			_, err = scans.UpdateStatus(scan)
			if err != nil {
				return nil, fmt.Errorf("error updating condition of cluster scan object: %v", scanName)
			}
			c.currentScanName = ""
			return obj, nil
		}

		if v1.ClusterScanConditionRunCompleted.IsTrue(scan) {
			scancopy := scan.DeepCopy()

			if !v1.ClusterScanConditionFailed.IsTrue(scan) {
				summary, report, err := c.getScanResults(ctx, scan)
				if err != nil {
					return nil, fmt.Errorf("error %v reading results of cluster scan object: %v", err, scanName)
				}
				scancopy.Status.Summary = summary
				_, err = reports.Create(report)
				if err != nil {
					return nil, fmt.Errorf("error %v saving clusterscanreport object", err)
				}
			}
			v1.ClusterScanConditionComplete.True(scancopy)
			/* update scan */
			_, err = scans.UpdateStatus(scancopy)
			if err != nil {
				return nil, fmt.Errorf("error updating condition of scan object: %v", scanName)
			}
			logrus.Infof("Marking ClusterScanConditionComplete for scan: %v", scanName)
			jobs.Enqueue(obj.Namespace, obj.Name)
		}
		return obj, nil
	})
	return nil
}

func (c *Controller) deleteJob(jobController batchctlv1.JobController, job *batchv1.Job, deletionPropagation metav1.DeletionPropagation) error {
	return jobController.Delete(job.Namespace, job.Name, &metav1.DeleteOptions{PropagationPolicy: &deletionPropagation})
}

func (c *Controller) getScanResults(ctx context.Context, scan *v1.ClusterScan) (*v1.ClusterScanSummary, *v1.ClusterScanReport, error) {
	configmaps := c.coreFactory.Core().V1().ConfigMap()
	//get the output configmap and create a report
	outputConfigName := strings.Join([]string{`cisscan-output-for`, scan.Name}, "-")
	cm, err := configmaps.Cache().Get(v1.ClusterScanNS, outputConfigName)
	if err != nil {
		return nil, nil, fmt.Errorf("cisScanHandler: Updated: error fetching configmap %v: %v", outputConfigName, err)
	}
	outputBytes := []byte(cm.Data[v1.DefaultScanOutputFileName])
	cisScanSummary, err := c.getScanSummary(outputBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("cisScanHandler: Updated: error getting report from configmap %v: %v", outputConfigName, err)
	}
	if cisScanSummary == nil {
		return nil, nil, fmt.Errorf("cisScanHandler: Updated: error: got empty report from configmap %v", outputConfigName)
	}

	scanReport, err := c.createClusterScanReport(ctx, outputBytes, scan)
	if err != nil {
		return nil, nil, fmt.Errorf("cisScanHandler: Updated: error getting report from configmap %v: %v", outputConfigName, err)
	}

	return cisScanSummary, scanReport, nil
}

func (c *Controller) getScanSummary(outputBytes []byte) (*v1.ClusterScanSummary, error) {
	r, err := report.Get(outputBytes)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	cisScanSummary := &v1.ClusterScanSummary{
		Total:         r.Total,
		Pass:          r.Pass,
		Fail:          r.Fail,
		Skip:          r.Skip,
		Warn:          r.Warn,
		NotApplicable: r.NotApplicable,
	}
	return cisScanSummary, nil
}

func (c *Controller) createClusterScanReport(ctx context.Context, outputBytes []byte, scan *v1.ClusterScan) (*v1.ClusterScanReport, error) {
	scanReport := &v1.ClusterScanReport{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name.SafeConcatName("scan-report", scan.Name, scan.Spec.ScanProfileName) + "-",
		},
	}
	profile, err := c.getClusterScanProfile(ctx, scan)
	if err != nil {
		return nil, fmt.Errorf("Error %v loading v1.ClusterScanProfile for name %v", scan.Spec.ScanProfileName, err)
	}
	scanReport.Spec.BenchmarkVersion = profile.Spec.BenchmarkVersion
	scanReport.Spec.LastRunTimestamp = time.Now().String()

	data, err := reportLibrary.GetJSONBytes(outputBytes)
	if err != nil {
		return nil, fmt.Errorf("Error %v loading scan report json bytes", err)
	}
	scanReport.Spec.ReportJSON = string(data[:])

	ownerRef := metav1.OwnerReference{
		APIVersion: "cis.cattle.io/v1",
		Kind:       "ClusterScan",
		Name:       scan.Name,
		UID:        scan.GetUID(),
	}
	scanReport.ObjectMeta.OwnerReferences = append(scanReport.ObjectMeta.OwnerReferences, ownerRef)

	return scanReport, nil
}

func (c *Controller) ensureCleanup(scan *v1.ClusterScan) error {
	var err error
	// Delete the dameonset
	dsPrefix := "sonobuoy-rancher-kube-bench-daemon-set"
	dsList, err := c.daemonsetCache.List(v1.ClusterScanNS, labels.Set(sonobuoyWorkerLabel).AsSelector())
	if err != nil {
		return fmt.Errorf("cis: ensureCleanup: error listing daemonsets: %v", err)
	}
	for _, ds := range dsList {
		if !strings.HasPrefix(ds.Name, dsPrefix) {
			continue
		}
		if e := c.daemonsets.Delete(v1.ClusterScanNS, ds.Name, &metav1.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
			return fmt.Errorf("cis: ensureCleanup: error deleting daemonset %v: %v", ds.Name, e)
		}
	}

	// Delete the pod
	podPrefix := name.SafeConcatName("security-scan-runner", scan.Name)
	podList, err := c.podCache.List(v1.ClusterScanNS, labels.Set(SonobuoyMasterLabel).AsSelector())
	if err != nil {
		return fmt.Errorf("cis: ensureCleanup: error listing pods: %v", err)
	}
	for _, pod := range podList {
		if !strings.HasPrefix(pod.Name, podPrefix) {
			continue
		}
		if e := c.pods.Delete(v1.ClusterScanNS, pod.Name, &metav1.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
			return fmt.Errorf("cis: ensureCleanup: error deleting pod %v: %v", pod.Name, e)
		}
	}

	// Delete cms
	cms, err := c.configMapCache.List(v1.ClusterScanNS, labels.NewSelector())
	if err != nil {
		return fmt.Errorf("cis: ensureCleanup: error listing cm: %v", err)
	}
	for _, cm := range cms {
		if !strings.Contains(cm.Name, scan.Name) {
			continue
		}

		if e := c.configmaps.Delete(v1.ClusterScanNS, cm.Name, &metav1.DeleteOptions{}); e != nil && !errors.IsNotFound(e) {
			return fmt.Errorf("cis: ensureCleanup: error deleting cm %v: %v", cm.Name, e)
		}
	}

	return err
}
