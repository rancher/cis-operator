package securityscan

import (
	"context"
	"fmt"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) handleClusterScanMetrics(ctx context.Context) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()

	scans.OnChange(ctx, c.Name, func(key string, obj *v1.ClusterScan) (*v1.ClusterScan, error) {
		if obj == nil || obj.DeletionTimestamp != nil {
			return obj, nil
		}
		if !(v1.ClusterScanConditionAlerted.IsUnknown(obj) && v1.ClusterScanConditionComplete.IsTrue(obj)) {
			return obj, nil
		}

		logrus.Debugf("Updating metrics for scan %v", obj.Name)

		scanName := "manual"
		if obj.Spec.ScheduledScanConfig != nil && obj.Spec.ScheduledScanConfig.CronSchedule != "" {
			scanName = obj.Name
		}
		scanProfileName := obj.Status.LastRunScanProfileName
		numTestsFailed := float64(obj.Status.Summary.Fail)
		numTestsTotal := float64(obj.Status.Summary.Total)
		numTestsNA := float64(obj.Status.Summary.NotApplicable)
		numTestsSkip := float64(obj.Status.Summary.Skip)
		numTestsPass := float64(obj.Status.Summary.Pass)
		numTestsWarn := float64(obj.Status.Summary.Warn)
		clusterName := c.ImageConfig.ClusterName

		c.numTestsFailed.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsFailed)
		c.numScansComplete.WithLabelValues(scanName, scanProfileName, clusterName).Inc()
		c.numTestsTotal.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsTotal)
		c.numTestsPassed.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsPass)
		c.numTestsSkipped.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsSkip)
		c.numTestsNA.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsNA)
		c.numTestsWarn.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsWarn)

		logrus.Debugf("Done updating metrics for scan %v", obj.Name)

		if obj.Spec.ScheduledScanConfig != nil {
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var err error
				scanObj, err := scans.Get(obj.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if scanObj.Spec.ScheduledScanConfig.ScanAlertRule == nil ||
					(scanObj.Spec.ScheduledScanConfig.ScanAlertRule != nil &&
						!scanObj.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnComplete &&
						!scanObj.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnFailure) {
					logrus.Debugf("No AlertRules configured for scan %v", scanObj.Name)
					v1.ClusterScanConditionAlerted.False(scanObj)
					v1.ClusterScanConditionAlerted.Message(scanObj, "No AlertRule configured for this scan")
				} else if scanObj.Status.ScanAlertingRuleName == "" {
					logrus.Debugf("Error creating PrometheusRule for scan %v", scanObj.Name)
					v1.ClusterScanConditionAlerted.False(scanObj)
					v1.ClusterScanConditionAlerted.Message(scanObj, "Alerts will not work due to the error creating PrometheusRule, Please check if Monitoring app is installed")
				} else {
					v1.ClusterScanConditionAlerted.True(scanObj)
				}
				_, err = scans.UpdateStatus(scanObj)
				return err
			})

			if updateErr != nil {
				return obj, fmt.Errorf("Retrying, got error %v in updating condition of scan object: %v ", updateErr, obj.Name)
			}
		}

		return obj, nil
	})
	return nil
}
