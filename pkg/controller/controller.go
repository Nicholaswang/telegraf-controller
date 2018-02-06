package controller

import (
	"encoding/json"
	"fmt"
	"github.com/Nicholaswang/telegraf-controller/pkg/types"
	"github.com/Nicholaswang/telegraf-controller/pkg/utils"
	"github.com/Nicholaswang/telegraf-controller/pkg/version"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"net/http"
	"os/exec"
	"strings"
)

type TelegrafController struct {
	command        string
	reloadStrategy *string
	configFile     string
	template       *template
	currentConfig  *types.ControllerConfig
	indexer        cache.Indexer
	queueRunning   workqueue.RateLimitingInterface
	queueNew       workqueue.RateLimitingInterface
	informer       cache.Controller
	clientset      kubernetes.Interface
}

// NewTelegrafController constructor
func NewTelegrafController() *TelegrafController {
	tc := &TelegrafController{
		clientset:    clientset,
		Config:       config,
		queueRunning: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		queueNew:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	tc.createIndexerInformer()
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

	err := tc.sync(key.(string))
	tc.handleErr(err, key)
	return true
}

// HasSynced returns true if the monitor has synced.
func (tc *TelegrafController) HasSynced() bool {
	return tc.informer.HasSynced()
}

// sync is the business logic of the monitor.
// The retry logic should not be part of the business logic.
func (tc *TelegrafController) sync(key string) error {
	pod, exists, err := tc.getPod(key)
	if err != nil {
		log.Printf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	if !exists {
		fmt.Printf("Pod %s does not exist anymore\n", key)
	} else if pod.Labels["telegraf"] == "true" {
		return tc.syncPod(pod)
	}
	return nil
}

func (tc *TelegrafController) syncPod(pod *v1.Pod) error {
	// TODO: deal with deleted pod

	return nil
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
func (tc *TelegrafController) Run(threadiness int, stopCh chan struct{}) {
	defer utilruntime.HandleCrash()

	// Let the workers stop when we are done
	defer tc.queueNew.ShutDown()
	log.Printf("Starting Telegraf Pod Monitor")

	go tc.informer.Run(stopCh)

	// Wait for all involved caches to be synced, before processing items from the queueNew is started
	if !cache.WaitForCacheSync(stopCh, tc.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	tc.dealQueueRunning()

	for i := 0; i < threadiness; i++ {
		go wait.Until(tc.runWorker, time.Second, stopCh)
	}

	<-stopCh
	log.Print("Stopping Telegraf Pod Monitor")
}

func (tc *TelegrafController) dealQueueRunning() {
	// TODO deal with queueRunning

}

func (tc *TelegrafController) runWorker() {
	for tc.processNextItem() {
	}
}

func (tc *TelegrafController) Update(updatedConfig *types.ControllerConfig) error {
	reloadRequired := reconfigureBackends(tc.currentConfig, updatedConfig)
	tc.currentConfig = updatedConfig

	data, err := tc.template.execute(updatedConfig)
	if err != nil {
		return err
	}

	err = utils.RewriteConfigFiles(data, *tc.reloadStrategy, tc.configFile)
	if err != nil {
		return err
	}

	if !reloadRequired {
		glog.Infoln("Telegraf reload not required")
		return nil
	}

	out, err := tc.reloadTelegraf()
	if len(out) > 0 {
		glog.Infof("Telegraf output:\n%v", string(out))
	}
	return err
}

// OnUpdate regenerate the configuration file of the backend
func (tc *TelegrafController) OnUpdate(pod *v1.pod) {
	//TODO delete the pod in the map

	//updatedConfig := newControllerConfig(tc)
	Update(updatedConfig)
}

func (tc *TelegrafController) reloadTelegraf() ([]byte, error) {
	out, err := exec.Command(tc.command, *tc.reloadStrategy, tc.configFile).CombinedOutput()
	return out, err
}
