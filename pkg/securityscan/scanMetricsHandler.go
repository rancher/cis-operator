package securityscan

import (
	"context"
	"fmt"
	"strconv"

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
		if obj.Spec.ScanAlertRule == nil {
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
		if obj.Spec.CronSchedule != "" {
			scanName = obj.Name
		}
		scanProfileName := obj.Status.LastRunScanProfileName
		numTestsFailed := float64(obj.Status.Summary.Fail)
		numTestsTotal := strconv.Itoa(obj.Status.Summary.Total)
		numTestsNA := strconv.Itoa(obj.Status.Summary.NotApplicable)
		numTestsSkip := strconv.Itoa(obj.Status.Summary.Skip)
		numTestsPass := strconv.Itoa(obj.Status.Summary.Pass)
		numTestsFail := strconv.Itoa(obj.Status.Summary.Fail)

		if obj.Spec.ScanAlertRule.AlertOnFailure {
			c.numTestsFailed.WithLabelValues(scanName, scanProfileName, numTestsTotal, numTestsSkip, numTestsNA).Set(numTestsFailed)
		}
		if obj.Spec.ScanAlertRule.AlertOnComplete {
			c.numScansComplete.WithLabelValues(scanName, scanProfileName, numTestsTotal, numTestsSkip, numTestsNA, numTestsPass, numTestsFail).Inc()
		}
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
