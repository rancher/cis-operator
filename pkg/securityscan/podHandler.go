package securityscan

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	corectlv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"

	cisoperatorapi "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io"
	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
)

// pod events should update the job conditions after validating Done annotation and Output CM
func (c *Controller) handlePods(ctx context.Context) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	jobs := c.batchFactory.Batch().V1().Job()
	pods := c.coreFactory.Core().V1().Pod()
	pods.OnChange(ctx, c.Name, func(key string, obj *corev1.Pod) (*corev1.Pod, error) {
		if obj == nil || obj.DeletionTimestamp != nil {
			return obj, nil
		}
		podSelector := labels.SelectorFromSet(labels.Set{
			cisoperatorapi.LabelController: c.Name,
		})
		// only handle pods launched by securityscan
		if obj.Labels == nil || !podSelector.Matches(labels.Set(obj.Labels)) {
			return obj, nil
		}
		// Check the annotation to see if it's done processing
		done, ok := obj.Annotations[cisoperatorapi.SonobuoyCompletionAnnotation]
		if !ok {
			return nil, nil
		}

		scanName, ok := obj.Labels[cisoperatorapi.LabelClusterScan]
		if !ok {
			// malformed
			return nil, nil
		}
		// get the scan being run
		scan, err := scans.Get(scanName, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			// scan is gone, delete
			logrus.Infof("scan gone, just delete it and move on %v", scanName)
			return nil, nil
		case err != nil:
			return obj, err
		}

		//find the job for this Pod and the clusterScan as well
		jobName := name.SafeConcatName("security-scan-runner", scanName)
		job, err := jobs.Cache().Get(obj.Namespace, jobName)
		switch {
		case errors.IsNotFound(err):
			return nil, nil
		case err != nil:
			return obj, err
		}

		scanCopy := scan.DeepCopy()
		if !v1.ClusterScanConditionRunCompleted.IsTrue(scan) {
			v1.ClusterScanConditionRunCompleted.True(scanCopy)
			if done != "true" {
				v1.ClusterScanConditionFailed.True(scanCopy)
				if done != "error" {
					v1.ClusterScanConditionFailed.Message(scanCopy, done)
				}
				logrus.Infof("Marking ClusterScanConditionFailed for scan: %v, error %v", scanName, done)
			}
			c.setClusterScanStatusDisplay(scanCopy)
			//update scan
			_, err = scans.UpdateStatus(scanCopy)
			if err != nil {
				return nil, fmt.Errorf("error updating condition of cluster scan object: %v", scanName)
			}
			logrus.Infof("Marking ClusterScanConditionRunCompleted for scan: %v", scanName)
			jobs.Enqueue(job.Namespace, job.Name)
		}
		return obj, nil
	})
	return nil
}

func deletePod(podController corectlv1.PodController, pod *corev1.Pod, deletionPropagation metav1.DeletionPropagation) error {
	logrus.Infof("delete pod called %v", pod.Status.Conditions)
	return podController.Delete(pod.Namespace, pod.Name, &metav1.DeleteOptions{PropagationPolicy: &deletionPropagation})
}
