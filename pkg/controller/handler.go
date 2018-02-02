package controller

import (
	"k8s.io/api/core/v1"
)

// Normally. for a pod migration, controller would revice two events
func (c *TelegrafController) syncPod(pod *v1.Pod) error {
	if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
		// Update: delete corresponding pod
		c.OnUpdate(pod)
	} else if pod.Status.Phase == "Pending" {
		// Add: add corresponding pod
		c.OnAdd(pod)
	}
	return nil
}
