package types

import (
	"k8s.io/api/core/v1"
)

type (
	// PodsConfig has pods with specified labels
	// key: appGroup
	Backend struct {
		//PodInfo map[string]string // ip, port, or namespace etc.,
		IP   string
		Port string
	}
	ControllerConfig struct {
		Pods     map[string][]v1.Pod
		Backends map[string][]Backend
	}
)

func (be1 Backend) Equal(be2 Backend) bool {
	if be1 == be2 {
		return true
	}
	if be1.IP != be2.IP || be1.Port != be2.Port {
		return false
	}

	return true
}

func ArrEqual(bes1 []Backend, bes2 []Backend) bool {
	if len(bes1) != len(bes2) {
		return false
	}
	for _, be1 := range bes1 {
		found := false
		for _, be2 := range bes2 {
			if be1.Equal(be2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (cc *ControllerConfig) Equal(uc *ControllerConfig) bool {
	if cc == uc {
		return true
	}
	if cc == nil || uc == nil {
		return false
	}
	if len(cc.Backends) != len(uc.Backends) {
		return false
	}
	for ccName, ccBe := range cc.Backends {
		found := false
		for ucName, ucBe := range uc.Backends {
			if ccName == ucName && ArrEqual(ccBe, ucBe) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
