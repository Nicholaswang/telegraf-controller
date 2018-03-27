package main

import (
	"flag"
	"github.com/Nicholaswang/telegraf-controller/pkg/controller"
	//"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	//"os"
	//"os/signal"
	//"syscall"
	"log"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var (
	clientset *kubernetes.Clientset
)

func main() {
	var (
		apiserver  string
		kubeconfig string
		influxdb   string
	)
	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&apiserver, "apiserver", "", "api server host")
	flag.StringVar(&influxdb, "influxdb", "", "influxdb url")

	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags(apiserver, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	tc := controller.NewTelegrafController(clientset, influxdb)
	/*
		stopCh := make(chan struct{})
		defer close(stopCh)
		go tc.Run(1, stopCh)
	*/
	go tc.Run()

	select {}
}
