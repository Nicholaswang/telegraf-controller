package controller

import (
	"errors"
	"github.com/Nicholaswang/cron"
	"github.com/Nicholaswang/telegraf-controller/pkg/types"
	"github.com/Nicholaswang/telegraf-controller/pkg/utils"
	//"github.com/Nicholaswang/telegraf-controller/pkg/version"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	//extensions "k8s.io/api/extensions/v1beta1"
	//"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	//"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"os/exec"
)

type TelegrafController struct {
	command        string
	reloadStrategy string
	configFile     string
	template       *template
	currentConfig  *types.ControllerConfig
	indexer        cache.Indexer
	queueRunning   workqueue.RateLimitingInterface
	queueNew       workqueue.RateLimitingInterface
	informer       cache.Controller
	clientset      kubernetes.Interface
	Namespace      string
}

// NewTelegrafController constructor
func NewTelegrafController(clientset kubernetes.Interface, influxdbUrl string) *TelegrafController {
	tc := &TelegrafController{
		clientset:      clientset,
		queueRunning:   workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		queueNew:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		Namespace:      "default",
		reloadStrategy: "native",
		currentConfig: &types.ControllerConfig{
			Influxdb: influxdbUrl,
			Pods:     make(map[string][]v1.Pod),
			Backends: make(map[string][]types.Backend),
		},
	}

	tc.createIndexerInformer()
	InitTemplate(tc)

	return tc
}

// getPod gets the pod we are interested in
func (tc *TelegrafController) getPod(key string) (*v1.Pod, bool, error) {
	obj, exists, err := tc.indexer.GetByKey(key)

	if exists {
		return obj.(*v1.Pod), exists, err
	}
	return nil, exists, err
}

func (tc *TelegrafController) processNextItem() bool {
	// Wait until there is a new item in the working queueNew
	key, quit := tc.queueNew.Get()
	if quit {
		return false
	}
	defer tc.queueNew.Done(key)

	err := tc.sync(key.(string), true)
	tc.handleErr(err, key)
	return true
}

// HasSynced returns true if the monitor has synced.
func (tc *TelegrafController) HasSynced() bool {
	return tc.informer.HasSynced()
}

// sync is the business logic of the controller.
// The retry logic should not be part of the business logic.
func (tc *TelegrafController) sync(key string, newPod bool) error {
	pod, _, err := tc.getPod(key)
	if err != nil {
		log.Printf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	//TODO: reconfiu
	if pod == nil {
		log.Printf("Pod nil, ignore syncing")
		return nil
	}

	return tc.syncPod(pod)
}

func (tc *TelegrafController) syncPod(pod *v1.Pod) error {
	log.Println("Generate new tcConfig")
	updatedConfig, err := tc.generateConfig(pod)
	if err != nil {
		log.Printf("generateConfig failed %v", err)
		return err
	}
	/*
		err := tc.reconfigureBackends(updatedConfig)
		if err != nil {
			log.Printf("Error while reconfiguring backends, err: %v", err)
			return err
		}
	*/
	log.Printf("Telegraf config backend length: %d", len(tc.currentConfig.Backends))
	equal := tc.currentConfig.Equal(updatedConfig)
	if equal {
		log.Printf("No need for reload")
		return nil
	}

	tc.currentConfig = updatedConfig
	err = tc.Update(tc.currentConfig)
	if err != nil {
		log.Printf("Error occured while updating config, err: %v", err)
	}

	return nil
}

func (tc *TelegrafController) addToUpdatedConfig(updatedConfig *types.ControllerConfig, pod *v1.Pod) {
	app := pod.Labels["app"]
	if app == "" {
		return
	}
	log.Printf("app: %s", app)
	updatedConfig.Pods[app] = append(updatedConfig.Pods[app], *pod)
}

func (tc *TelegrafController) generateConfig(pod *v1.Pod) (*types.ControllerConfig, error) {
	updatedConfig := *tc.currentConfig
	app := pod.Labels["app"]
	if app == "" {
		return nil, errors.New("pod without app label")
	}
	for appName, podArr := range updatedConfig.Pods {
		if appName == app {
			var tmp = make([]v1.Pod, 0)
			for _, po := range podArr {
				//check pod status
				if tc.isPodAlive(&po) {
					tmp = append(tmp, po)
				} else {
					//discarding obsolete pod
					log.Printf("generateConfig: discarding obsolete pod: %s, namespace: %s", pod.Name, pod.ObjectMeta.Namespace)
				}
			}
			if tc.isPodExist(tmp, pod) {
				log.Printf("Pod: %s exist", pod.Name)
			} else {
				if tc.isPodAlive(pod) {
					tmp = append(tmp, *pod)
				}
			}
			updatedConfig.Pods[appName] = tmp
			return &updatedConfig, nil
		}
	}
	if tc.isPodAlive(pod) {
		var tmp = []v1.Pod{*pod}
		updatedConfig.Pods[app] = tmp
	}

	return &updatedConfig, nil
}

func (tc *TelegrafController) isPodExist(pods []v1.Pod, pod *v1.Pod) bool {
	for _, po := range pods {
		if po.Name == pod.Name && po.Status.PodIP == pod.Status.PodIP && po.Labels["monitorPort"] == pod.Labels["monitorPort"] {
			return true
		}
	}

	return false
}

func (tc *TelegrafController) isPodAlive(pod *v1.Pod) bool {
	phase := pod.Status.Phase
	if phase == "Running" || phase == "Pending" {
		return true
	} else {
		return false
	}

	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (tc *TelegrafController) handleErr(err error, key interface{}) {
	if err == nil {
		tc.queueNew.Forget(key)
		return
	}

	// This monitor retries 5 times if something goes wrong. After that, it stops trying.
	if tc.queueNew.NumRequeues(key) < 5 {
		log.Printf("Error syncing pod %v: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queueNew and the re-enqueue history, the key will be processed later again.
		tc.queueNew.AddRateLimited(key)
		return
	}

	tc.queueNew.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	utilruntime.HandleError(err)
	log.Printf("Dropping pod %q out of the queueNew: %v", key, err)
}

// Run executes the controller.
//func (tc *TelegrafController) Run(threadiness int, stopCh chan struct{}) {
func (tc *TelegrafController) Run() {
	defer utilruntime.HandleCrash()

	// Let the workers stop when we are done
	defer tc.queueRunning.ShutDown()
	defer tc.queueNew.ShutDown()
	log.Printf("Starting Telegraf Pod Monitor")
	tc.dealQueueRunning()

	cron_ := cron.New()
	cron_.Start()
	defer cron_.Stop()
	cron_.AddFunc("@every 60s", tc.dealQueueRunning)

	select {}

	/*
		tc.dealQueueRunning()

		go tc.informer.Run(stopCh)

		// Wait for all involved caches to be synced, before processing items from the queueNew is started
		if !cache.WaitForCacheSync(stopCh, tc.HasSynced) {
			utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
			return
		}

		for i := 0; i < threadiness; i++ {
			go wait.Until(tc.runWorker, time.Second, stopCh)
		}

		<-stopCh
		log.Print("Stopping Telegraf Pod Monitor")
	*/
}

func (tc *TelegrafController) processRunningQueue() bool {
	key, quit := tc.queueRunning.Get()
	if quit {
		return false
	}
	defer tc.queueRunning.Done(key)

	err := tc.sync(key.(string), false)
	tc.handleErr(err, key)

	return true
}

func (tc *TelegrafController) dealQueueRunning() {
	log.Println("Start dealing queueRunning")
	labelSelector := labels.Set{
		"telegraf": "true",
	}
	var options metav1.ListOptions
	options.LabelSelector = labelSelector.String()
	pods, err := tc.clientset.CoreV1().Pods(v1.NamespaceAll).List(options)
	if err != nil {
		glog.Errorf("Get current pods failed")
		return
	}
	log.Printf("Pods length: %d", len(pods.Items))
	if len(pods.Items) == 0 {
		return
	}

	updatedConfig := &types.ControllerConfig{
		Influxdb: tc.currentConfig.Influxdb,
		Pods:     make(map[string][]v1.Pod),
		Backends: make(map[string][]types.Backend),
	}
	for _, pod := range pods.Items {
		log.Printf("pod ip %s", pod.Status.PodIP)
		tc.addToUpdatedConfig(updatedConfig, &pod)
	}
	err = tc.reconfigureBackends(updatedConfig)
	if err != nil {
		glog.Errorf("Error while reconfiguring backends, err: %v", err)
		return
	}
	log.Printf("Telegraf config backend length: %d", len(updatedConfig.Backends))
	equal := tc.currentConfig.Equal(updatedConfig)
	if equal {
		log.Printf("No need for reload")
		return
	}
	tc.currentConfig = updatedConfig
	err = tc.Update(tc.currentConfig)
	if err != nil {
		glog.Errorf("Error occured while updating config, err: %v", err)
		return
	}
}

func (tc *TelegrafController) runWorker() {
	for tc.processNextItem() {
	}
}

func (tc *TelegrafController) Update(updatedConfig *types.ControllerConfig) error {
	data, err := tc.template.execute(updatedConfig)
	if err != nil {
		return err
	}

	err = utils.RewriteConfigFiles(data, tc.reloadStrategy, tc.configFile)
	if err != nil {
		return err
	}

	out, err := tc.reloadTelegraf()
	if len(out) > 0 {
		glog.Infof("Telegraf output:\n%v", string(out))
	}
	return err
}

func (tc *TelegrafController) reconfigureBackends(updatedConfig *types.ControllerConfig) error {
	for appName, pods := range updatedConfig.Pods {
		log.Printf("appName: %s", appName)
		var backends = make([]types.Backend, 0)
		log.Printf("pods length: %d", len(pods))
		for _, pod := range pods {
			var backend types.Backend
			var podName = pod.Name
			var podIP = pod.Status.PodIP
			var monitorPort = pod.Labels["monitorPort"]
			if monitorPort == "" {
				log.Printf("Pod %s has no monitor port label", podName)
				continue
			}
			backend.IP = podIP
			log.Printf("PODNAME: %s", podName)
			log.Printf("IP: %s", podIP)
			backend.Port = monitorPort
			backends = append(backends, backend)
		}
		updatedConfig.Backends[appName] = backends
	}

	return nil
}

func (tc *TelegrafController) reloadTelegraf() ([]byte, error) {
	out, err := exec.Command(tc.command, tc.reloadStrategy).CombinedOutput()
	return out, err
}
