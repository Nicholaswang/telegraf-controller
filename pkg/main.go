package main

import (
	"github.com/Nicholaswang/telegraf-controller/pkg/controller"
	"github.com/golang/glog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	tc := controller.NewTelegrafController()
	stopCh := make(chan struct{})
	defer close(stopCh)
	go tc.Run(1, stopCh)

	select {}
}
