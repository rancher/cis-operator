package securityscan

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

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
		if obj.Spec.ScheduledScanConfig != nil && obj.Spec.ScheduledScanConfig.ScanAlertRule == nil {
			logrus.Debugf("No AlertRules configured for scan %v", obj.Name)
			v1.ClusterScanConditionAlerted.False(obj)
			v1.ClusterScanConditionAlerted.Message(obj, "No AlertRule configured for this scan")
			_, err := scans.UpdateStatus(obj)
			if err != nil {
				return obj, fmt.Errorf("Retrying, got error %v in updating condition of scan object: %v ", err, obj.Name)
			}
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

		c.numTestsFailed.WithLabelValues(scanName, scanProfileName).Set(numTestsFailed)
		c.numScansComplete.WithLabelValues(scanName, scanProfileName).Inc()
		c.numTestsTotal.WithLabelValues(scanName, scanProfileName).Set(numTestsTotal)
		c.numTestsPassed.WithLabelValues(scanName, scanProfileName).Set(numTestsPass)
		c.numTestsSkipped.WithLabelValues(scanName, scanProfileName).Set(numTestsSkip)
		c.numTestsNA.WithLabelValues(scanName, scanProfileName).Set(numTestsNA)

		logrus.Debugf("Done updating metrics for scan %v", obj.Name)
		v1.ClusterScanConditionAlerted.True(obj)
		_, err := scans.UpdateStatus(obj)
		if err != nil {
			return obj, fmt.Errorf("Retrying, got error %v in updating condition of scan object: %v ", err, obj.Name)
		}
		return obj, nil
	})
	return nil
}
