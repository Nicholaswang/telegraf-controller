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
				if pod.Labels["telegraf"] == "true" {
					if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Terminated != nil {
						log.Printf("Ignore Add Event for Pod had been terminate: %s: %s", pod.Name, pod.Status.ContainerStatuses[0].State.Terminated.Reason)
					} else {
						if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
							log.Println("For Add Event, Adding to workqueue: ", pod.Name)
							c.queue.Add(key)
						}
					}
				}
			},
			UpdateFunc: func(old interface{}, new interface{}) {
				pod := new.(*v1.Pod)

				if pod.Status.Phase != "Pending" {
					if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
						log.Println("For Update Event, Adding to workqueue: ", key)
						c.queue.Add(key)
					}
				}
			},
		},
		cache.Indexers{},
	)
}
