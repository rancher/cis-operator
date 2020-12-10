package securityscan

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/util/retry"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
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
		clusterName := c.ImageConfig.ClusterName

		logrus.Infof("clusterName %v", clusterName)

		c.numTestsFailed.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsFailed)
		c.numScansComplete.WithLabelValues(scanName, scanProfileName, clusterName).Inc()
		c.numTestsTotal.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsTotal)
		c.numTestsPassed.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsPass)
		c.numTestsSkipped.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsSkip)
		c.numTestsNA.WithLabelValues(scanName, scanProfileName, clusterName).Set(numTestsNA)

		logrus.Debugf("Done updating metrics for scan %v", obj.Name)

		if obj.Spec.ScheduledScanConfig != nil {
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var err error
				if obj.Spec.ScheduledScanConfig.ScanAlertRule == nil ||
					(obj.Spec.ScheduledScanConfig.ScanAlertRule != nil &&
						!obj.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnComplete &&
						!obj.Spec.ScheduledScanConfig.ScanAlertRule.AlertOnFailure) {
					logrus.Infof("No AlertRules configured for scan %v", obj.Name)
					v1.ClusterScanConditionAlerted.False(obj)
					v1.ClusterScanConditionAlerted.Message(obj, "No AlertRule configured for this scan")
				} else if obj.Status.ScanAlertingRuleName == "" {
					logrus.Infof("Error creating PrometheusRule for scan %v", obj.Name)
					v1.ClusterScanConditionAlerted.False(obj)
					v1.ClusterScanConditionAlerted.Message(obj, "Alerts will not work due to the error creating PrometheusRule")
				} else {
					v1.ClusterScanConditionAlerted.True(obj)
				}
				_, err = scans.UpdateStatus(obj)
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
