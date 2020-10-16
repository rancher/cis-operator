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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) handleScheduledScans(ctx context.Context) error {
	scheduledScans := c.cisFactory.Cis().V1().ScheduledScan()
	scans := c.cisFactory.Cis().V1().ClusterScan()

	scheduledScans.OnChange(ctx, c.Name, func(key string, obj *v1.ScheduledScan) (*v1.ScheduledScan, error) {
		if obj == nil || obj.DeletionTimestamp != nil {
			return obj, nil
		}

		logrus.Infof("scheduledScanHandler: sync called for scheduledScan CR %v ", obj.Name)

		if err := c.validateScheduledScanSpec(obj); err != nil {
			v1.ClusterScanConditionFailed.True(obj)
			message := fmt.Sprintf("Error validating Schedule %v, error: %v", obj.Spec.CronSchedule, err)
			v1.ClusterScanConditionFailed.Message(obj, message)
			logrus.Errorf(message)
			return scheduledScans.UpdateStatus(obj)
		}

		//if nextScanAt is set then make sure we process only if the time is right
		if obj.Status.LastRunTimestamp != "" && obj.Status.NextScanAt != "" {
			currTime := time.Now().Format(time.RFC3339)
			logrus.Infof("scheduledScanHandler: next scan is scheduled for: %v, current time: %v", obj.Status.NextScanAt, currTime)

			nextScanTime, err := time.Parse(time.RFC3339, obj.Status.NextScanAt)
			if err != nil {
				return obj, fmt.Errorf("scheduledScanHandler: retrying, got error %v in parsing NextScanAt %v time for scheduledScan: %v ", err, obj.Status.NextScanAt, obj.Name)
			}
			if nextScanTime.After(time.Now()) {
				logrus.Infof("scheduledScanHandler: run time is later, skipping this run scheduledScan CR %v ", obj.Name)
				after := nextScanTime.Sub(time.Now())
				scheduledScans.EnqueueAfter(obj.Name, after)
				if obj.Generation != obj.Status.ObservedGeneration {
					obj.Status.ObservedGeneration = obj.Generation
					return scheduledScans.UpdateStatus(obj)
				}
				return obj, nil
			}
		}

		logrus.Infof("scheduledScanHandler: processing scheduledScan CR %v ", obj.Name)

		schedule := c.getCronSchedule(obj)
		cronSchedule, err := cron.ParseStandard(schedule)
		if err != nil {
			return obj, fmt.Errorf("Error parsing invalid cron string for schedule: %v", err)
		}

		clusterScan := c.createClusterScan(obj)
		logrus.Infof("scheduledScanHandler: creating new clusterScan CR %v ", clusterScan.GenerateName)
		createdClusterScan, err := scans.Create(clusterScan)
		if err != nil {
			return nil, fmt.Errorf("Error %v saving clusterscan object", err)
		}

		updateErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var err error
			scheduledScanObj, err := scheduledScans.Get(obj.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			// reset conditions to remove the reconciling condition, because as per kstatus lib its presence is considered an error
			scheduledScanObj.Status.Conditions = []genericcondition.GenericCondition{}
			now := time.Now()
			scheduledScanObj.Status.LastRunTimestamp = now.Format(time.RFC3339)
			scheduledScanObj.Status.LastClusterScanName = createdClusterScan.Name

			nextScanAt := cronSchedule.Next(now)
			scheduledScanObj.Status.NextScanAt = nextScanAt.Format(time.RFC3339)
			after := nextScanAt.Sub(now)
			scheduledScans.EnqueueAfter(scheduledScanObj.Name, after)

			scheduledScanObj.Status.ObservedGeneration = scheduledScanObj.Generation
			_, err = scheduledScans.UpdateStatus(scheduledScanObj)
			return err
		})

		if updateErr != nil {
			return obj, fmt.Errorf("Retrying, got error %v in updating status for scheduledScan: %v ", err, obj.Name)
		}

		if err := c.purgeOldClusterScans(obj); err != nil {
			return obj, err
		}

		return obj, nil
	})

	return nil
}

func (c *Controller) validateScheduledScanSpec(obj *v1.ScheduledScan) error {
	if obj.Spec.CronSchedule != "" {
		_, err := cron.ParseStandard(obj.Spec.CronSchedule)
		if err != nil {
			return fmt.Errorf("error parsing invalid cron string for schedule: %v", err)
		}
	}
	return nil
}

func (c *Controller) getCronSchedule(obj *v1.ScheduledScan) string {
	cronSchedule := v1.DefaultCronSchedule
	if obj.Spec.CronSchedule != "" {
		cronSchedule = obj.Spec.CronSchedule
	}
	return cronSchedule
}

func (c *Controller) getRetentionCount(obj *v1.ScheduledScan) int {
	retentionCount := v1.DefaultRetention
	if obj.Spec.RetentionCount != 0 {
		retentionCount = obj.Spec.RetentionCount
	}
	return retentionCount
}

func (c *Controller) createClusterScan(scheduleScan *v1.ScheduledScan) *v1.ClusterScan {
	clusterScan := &v1.ClusterScan{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name.SafeConcatName("ss", scheduleScan.Name) + "-",
		},
		Spec: v1.ClusterScanSpec{
			ScanProfileName: scheduleScan.Spec.ScanProfileName,
		},
	}
	ownerRef := metav1.OwnerReference{
		APIVersion: "cis.cattle.io/v1",
		Kind:       "ScheduledScan",
		Name:       scheduleScan.Name,
		UID:        scheduleScan.GetUID(),
	}
	clusterScan.ObjectMeta.OwnerReferences = append(clusterScan.ObjectMeta.OwnerReferences, ownerRef)

	return clusterScan
}

func (c *Controller) purgeOldClusterScans(scheduleScan *v1.ScheduledScan) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	retention := c.getRetentionCount(scheduleScan)
	logrus.Infof("scheduledScanHandler: purgeOldScans for scheduledScan: %v ,retention: %v", scheduleScan.Name, retention)
	allClusterScans, err := scans.Cache().List(labels.NewSelector())
	if err != nil {
		return fmt.Errorf("error listing cluster scans for scheduledScan %v: %v", scheduleScan.Name, err)
	}
	var clusterScans []*v1.ClusterScan
	for _, cs := range allClusterScans {
		if !strings.HasPrefix(cs.Name, name.SafeConcatName("ss", scheduleScan.Name)+"-") {
			continue
		}
		clusterScans = append(clusterScans, cs)
	}
	if len(clusterScans) <= retention {
		return nil
	}
	sort.Slice(clusterScans, func(i, j int) bool {
		return !clusterScans[i].CreationTimestamp.Before(&clusterScans[j].CreationTimestamp)
	})
	for _, cs := range clusterScans[retention:] {
		logrus.Infof("scheduledScanHandler: purgeOldScans: deleting cs: %v %v", cs.Name, cs.CreationTimestamp.String())
		if err := c.deleteClusterScanWithRetry(cs.Name); err != nil {
			logrus.Errorf("scheduledScanHandler: purgeOldScans: error deleting cluster scan: %v: %v",
				cs.Name, err)
		}
	}
	return nil
}

func (c *Controller) deleteClusterScanWithRetry(name string) error {
	scans := c.cisFactory.Cis().V1().ClusterScan()
	delErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		err = scans.Delete(name, &metav1.DeleteOptions{})
		return err
	})
	return delErr
}
