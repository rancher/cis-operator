package securityscan

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "github.com/rancher/cis-operator/pkg/apis/cis.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/genericcondition"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) handleScheduledClusterScans(ctx context.Context) error {
	scheduledScans := c.cisFactory.Cis().V1().ClusterScan()

	scheduledScans.OnChange(ctx, c.Name, func(key string, obj *v1.ClusterScan) (*v1.ClusterScan, error) {
		if obj == nil || obj.DeletionTimestamp != nil {
			return obj, nil
		}

		if obj.Spec.ScheduledScanConfig != nil && obj.Spec.ScheduledScanConfig.CronSchedule == "" {
			return obj, nil
		}

		//if nextScanAt is set then make sure we process only if the time is right
		if v1.ClusterScanConditionComplete.IsTrue(obj) && obj.Status.LastRunTimestamp != "" && obj.Status.NextScanAt != "" {
			currTime := time.Now().Format(time.RFC3339)
			logrus.Debugf("scheduledScanHandler: sync called for scheduled ClusterScan CR %v ", obj.Name)
			logrus.Debugf("scheduledScanHandler: next run is scheduled for: %v, current time: %v", obj.Status.NextScanAt, currTime)

			nextScanTime, err := time.Parse(time.RFC3339, obj.Status.NextScanAt)
			if err != nil {
				return obj, fmt.Errorf("scheduledScanHandler: retrying, got error %v in parsing NextScanAt %v time for scheduledScan: %v ", err, obj.Status.NextScanAt, obj.Name)
			}
			if nextScanTime.After(time.Now()) {
				logrus.Debugf("scheduledScanHandler: run time is later, skipping this run scheduledScan CR %v ", obj.Name)
				after := nextScanTime.Sub(time.Now())
				scheduledScans.EnqueueAfter(obj.Name, after)
				if obj.Generation != obj.Status.ObservedGeneration {
					obj.Status.ObservedGeneration = obj.Generation
					return scheduledScans.UpdateStatus(obj)
				}
				return obj, nil
			}
			// can process this scan again
			logrus.Infof("scheduledScanHandler: now processing scheduledScan CR %v ", obj.Name)
			updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var err error
				scheduledScanObj, err := scheduledScans.Get(obj.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				// reset conditions
				scheduledScanObj.Status.Conditions = []genericcondition.GenericCondition{}
				scheduledScanObj.Status.LastRunTimestamp = ""
				scheduledScanObj.Status.NextScanAt = ""

				_, err = scheduledScans.UpdateStatus(scheduledScanObj)
				return err
			})

			if updateErr != nil {
				return obj, fmt.Errorf("Retrying, got error %v in updating status for scheduledScan: %v ", updateErr, obj.Name)
			}
		}

		return obj, nil
	})

	return nil
}

func (c *Controller) validateScheduledScanSpec(scan *v1.ClusterScan) error {
	if scan.Spec.ScheduledScanConfig != nil && scan.Spec.ScheduledScanConfig.CronSchedule != "" {
		_, err := cron.ParseStandard(scan.Spec.ScheduledScanConfig.CronSchedule)
		if err != nil {
			return fmt.Errorf("error parsing invalid cron string for schedule: %v", err)
		}
	}
	return nil
}

func (c *Controller) getCronSchedule(scan *v1.ClusterScan) (cron.Schedule, error) {
	schedule := v1.DefaultCronSchedule
	if scan.Spec.ScheduledScanConfig != nil && scan.Spec.ScheduledScanConfig.CronSchedule != "" {
		schedule = scan.Spec.ScheduledScanConfig.CronSchedule
	}
	cronSchedule, err := cron.ParseStandard(schedule)
	if err != nil {
		return nil, fmt.Errorf("Error parsing invalid cron string for schedule: %v", err)
	}
	return cronSchedule, nil
}

func (c *Controller) getRetentionCount(scan *v1.ClusterScan) int {
	retentionCount := v1.DefaultRetention
	if scan.Spec.ScheduledScanConfig != nil && scan.Spec.ScheduledScanConfig.RetentionCount != 0 {
		retentionCount = scan.Spec.ScheduledScanConfig.RetentionCount
	}
	return retentionCount
}

func (c *Controller) rescheduleScan(scan *v1.ClusterScan) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	cronSchedule, err := c.getCronSchedule(scan)
	if err != nil {
		return fmt.Errorf("Cannot reschedule, Error parsing invalid cron string for schedule: %v", err)
	}
	now := time.Now()
	nextScanAt := cronSchedule.Next(now)
	scan.Status.NextScanAt = nextScanAt.Format(time.RFC3339)
	after := nextScanAt.Sub(now)
	scans.EnqueueAfter(scan.Name, after)
	return nil
}

func (c *Controller) purgeOldClusterScanReports(obj *v1.ClusterScan) error {
	reports := c.cisFactory.Cis().V1().ClusterScanReport()
	retention := c.getRetentionCount(obj)
	allClusterScanReportsList, err := reports.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing cluster scans for scheduledScan %v: %v", obj.Name, err)
	}
	allClusterScanReports := allClusterScanReportsList.Items
	var clusterScanReports []v1.ClusterScanReport
	for _, cs := range allClusterScanReports {
		if !strings.HasPrefix(cs.Name, name.SafeConcatName("scan-report", obj.Name)+"-") {
			continue
		}
		clusterScanReports = append(clusterScanReports, cs)
	}
	if len(clusterScanReports) <= retention {
		return nil
	}
	sort.Slice(clusterScanReports, func(i, j int) bool {
		return !clusterScanReports[i].CreationTimestamp.Before(&clusterScanReports[j].CreationTimestamp)
	})

	for _, cs := range clusterScanReports[retention:] {
		logrus.Infof("scheduledScanHandler: purgeOldScans: deleting cs: %v %v", cs.Name, cs.CreationTimestamp.String())
		if err := c.deleteClusterScanReportWithRetry(cs.Name); err != nil {
			logrus.Errorf("scheduledScanHandler: purgeOldScans: error deleting cluster scan: %v: %v",
				cs.Name, err)
		}
	}
	return nil
}

func (c *Controller) deleteClusterScanReportWithRetry(name string) error {
	reports := c.cisFactory.Cis().V1().ClusterScanReport()
	delErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		err = reports.Delete(name, &metav1.DeleteOptions{})
		return err
	})
	return delErr
}
