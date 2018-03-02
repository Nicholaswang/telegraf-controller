package controller

import (
	"log"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func (c *TelegrafController) createIndexerInformer() {
	labelSelector := labels.Set{
		"telegraf": "true",
	}

	c.indexer, c.informer = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labelSelector.String()
				return c.clientset.CoreV1().Pods(c.Namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labelSelector.String()
				return c.clientset.CoreV1().Pods(c.Namespace).Watch(options)
			},
		},
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
					log.Printf("Ignore Add Event for Pod had been terminate: %s: %s", pod.Name, pod.Status.ContainerStatuses[0].State.Terminated.Reason)
				} else {
					if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
						if pod.Status.Phase == "Running" || pod.Status.Phase == "Pending" {
							log.Println("New pod, Adding to new workqueue: ", pod.Name)
							c.queueNew.Add(key)
						} else {
							log.Printf("Pod %s is in %s status", pod.Name, pod.Status.Phase)
						}
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				// Delete corresponding pod in telegraf conf
				if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
					//if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
					log.Println("Pod exit event, Adding to new workqueue: ", pod.Name)
					c.queueNew.Add(key)
					//}
				}
			},
		},
		cache.Indexers{},
	)
}
