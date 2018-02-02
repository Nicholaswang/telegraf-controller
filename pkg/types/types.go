package types

import (
	"k8s.io/api/core/v1"
)

type (
	// PodsConfig has pods with specified labels
	// key: appGroup
	PodsConfig struct {
		Pods map[string][]*v1.Pod
	}
)
