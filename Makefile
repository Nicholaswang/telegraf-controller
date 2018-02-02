.PHONY: default
default: build

REPO_LOCAL=localhost/telegraf-controller
include container.mk

GOOS=linux
GOARCH=amd64
GIT_REPO=$(shell git config --get remote.origin.url)
ROOT_PKG=github.com/Nicholaswang/telegraf-controller/pkg

.PHONY: build
build:
	mkdir -p ~/gopath/src/github.com/Nicholaswang
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
	  -installsuffix cgo \
	  -ldflags "-s -w -X $(ROOT_PKG)/version.RELEASE=$(TAG) -X $(ROOT_PKG)/version.COMMIT=$(GIT_COMMIT) -X $(ROOT_PKG)/version.REPO=$(GIT_REPO)" \
	  -o rootfs/telegraf-controller \
	  $(ROOT_PKG)
