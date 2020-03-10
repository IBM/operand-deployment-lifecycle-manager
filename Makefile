#!/bin/bash
#
# Copyright 2020 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.DEFAULT_GOAL:=help
# Specify whether this repo is build locally or not, default values is '1';
# If set to 1, then you need to also set 'DOCKER_USERNAME' and 'DOCKER_PASSWORD'
# environment variables before build the repo.
BUILD_LOCALLY ?= 1
TARGET_GOOS=linux
TARGET_GOARCH=amd64

# The namespcethat operator will be deployed in
NAMESPACE=ibm-common-services

# Image URL to use all building/pushing image targets;
# Use your own docker registry and image name for dev/test by overridding the IMG and REGISTRY environment variable.
IMG ?= odlm
REGISTRY ?= quay.io/opencloudio
CSV_VERSION ?= 0.0.1

QUAY_USERNAME ?=
QUAY_PASSWORD ?=

MARKDOWN_LINT_WHITELIST=https://quay.io/cnr

TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

LOCAL_OS := $(shell uname)
LOCAL_ARCH := $(shell uname -m)
ifeq ($(LOCAL_OS),Linux)
    TARGET_OS ?= linux
    XARGS_FLAGS="-r"
	STRIP_FLAGS=
else ifeq ($(LOCAL_OS),Darwin)
    TARGET_OS ?= darwin
    XARGS_FLAGS=
	STRIP_FLAGS="-x"
else
    $(error "This system's OS $(LOCAL_OS) isn't recognized/supported")
endif

include common/Makefile.common.mk

##@ Application

install: ## Install all resources (CR/CRD's, RBAC and Operator)
	@echo ....... Set environment variables ......
	- export DEPLOY_DIR=deploy/crds
	- export WATCH_NAMESPACE=
	@echo ....... Creating namespace .......
	- kubectl create namespace ${NAMESPACE}
	@echo ....... Applying CRDs .......
	- kubectl apply -f deploy/crds/operator.ibm.com_operandregistries_crd.yaml
	- kubectl apply -f deploy/crds/operator.ibm.com_operandconfigs_crd.yaml
	- kubectl apply -f deploy/crds/operator.ibm.com_operandrequests_crd.yaml
	@echo ....... Applying RBAC .......
	- kubectl apply -f deploy/service_account.yaml -n ${NAMESPACE}
	- kubectl apply -f deploy/role.yaml
	- kubectl apply -f deploy/role_binding.yaml
	@echo ....... Applying Operator .......
	- kubectl apply -f deploy/operator.yaml -n ${NAMESPACE}
	@echo ....... Creating the Instance .......
	- kubectl apply -f deploy/crds/operator.ibm.com_v1alpha1_operandrequest_cr.yaml -n ${NAMESPACE}

uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@echo ....... Deleting CR .......
	- kubectl delete -f deploy/crds/operator.ibm.com_v1alpha1_operandrequest_cr.yaml -n ${NAMESPACE} --ignore-not-found
	@echo ....... Deleting Operator .......
	- kubectl delete -f deploy/operator.yaml -n ${NAMESPACE} --ignore-not-found
	@echo ....... Deleting CRDs.......
	- kubectl delete -f deploy/crds/operator.ibm.com_operandrequests_crd.yaml --ignore-not-found
	- kubectl delete -f deploy/crds/operator.ibm.com_operandconfigs_crd.yaml --ignore-not-found
	- kubectl delete -f deploy/crds/operator.ibm.com_operandregistries_crd.yaml --ignore-not-found
	@echo ....... Deleting Rules and Service Account .......
	- kubectl delete -f deploy/role_binding.yaml --ignore-not-found
	- kubectl delete -f deploy/service_account.yaml -n ${NAMESPACE} --ignore-not-found
	- kubectl delete -f deploy/role.yaml --ignore-not-found
	@echo ....... Deleting namespace ${NAMESPACE}.......
	- kubectl delete namespace ${NAMESPACE} --ignore-not-found

##@ Development

check: lint-all ## Check all files lint error

code-dev: ## Run the default dev commands which are the go tidy, fmt, vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-tidy
	- make code-fmt
	- make code-vet
	- make code-gen
	@echo Running the common required commands for code delivery
	- make check
	- make test
	- make build

run: ## Run against the configured Kubernetes cluster in ~/.kube/config
	WATCH_NAMESPACE= go run ./cmd/manager/main.go -v=3 --zap-encoder=console

ifeq ($(BUILD_LOCALLY),0)
    export CONFIG_DOCKER_TARGET = config-docker
endif

##@ Build

build:
	@echo "Building the odlm binary"
	@CGO_ENABLED=0 go build -o build/_output/bin/$(IMG) ./cmd/manager
	@strip $(STRIP_FLAGS) build/_output/bin/$(IMG)

build-image: build $(CONFIG_DOCKER_TARGET)
	$(eval ARCH := $(shell uname -m|sed 's/x86_64/amd64/'))
	docker build -t $(REGISTRY)/$(IMG)-$(ARCH):$(VERSION) -f build/Dockerfile .
	@\rm -f build/_output/bin/odlm
	@if [ $(BUILD_LOCALLY) -ne 1 ] && [ "$(ARCH)" = "amd64" ]; then docker push $(REGISTRY)/$(IMG)-$(ARCH):$(VERSION); fi

# runs on amd64 machine
build-image-ppc64le: $(CONFIG_DOCKER_TARGET)
ifeq ($(LOCAL_OS),Linux)
ifeq ($(LOCAL_ARCH),x86_64)
	GOOS=linux GOARCH=ppc64le CGO_ENABLED=0 go build -o build/_output/bin/odlm-ppc64le ./cmd/manager
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	docker build -t $(REGISTRY)/$(IMG)-ppc64le:$(VERSION) -f build/Dockerfile.ppc64le .
	@\rm -f build/_output/bin/odlm-ppc64le
	@if [ $(BUILD_LOCALLY) -ne 1 ]; then docker push $(REGISTRY)/$(IMG)-ppc64le:$(VERSION); fi
endif
endif

# runs on amd64 machine
build-image-s390x: $(CONFIG_DOCKER_TARGET)
ifeq ($(LOCAL_OS),Linux)
ifeq ($(LOCAL_ARCH),x86_64)
	GOOS=linux GOARCH=s390x CGO_ENABLED=0 go build -o build/_output/bin/odlm-s390x ./cmd/manager
	docker run --rm --privileged multiarch/qemu-user-static:register --reset
	docker build -t $(REGISTRY)/$(IMG)-s390x:$(VERSION) -f build/Dockerfile.s390x .
	@\rm -f build/_output/bin/odlm-s390x
	@if [ $(BUILD_LOCALLY) -ne 1 ]; then docker push $(REGISTRY)/$(IMG)-s390x:$(VERSION); fi
endif
endif

##@ Test

test: ## Run unit test
	@go test ${TESTARGS} ./pkg/...

test-e2e: ## Run integration e2e tests with different options.
	@echo ... Running the same e2e tests with different args ...
	@echo ... Running locally ...
	- operator-sdk test local ./test/e2e --verbose --up-local --namespace=${NAMESPACE}
	# @echo ... Running with the param ...
	# - operator-sdk test local ./test/e2e --namespace=${NAMESPACE}

coverage: ## Run code coverage test
	@common/scripts/codecov.sh ${BUILD_LOCALLY}

scorecard: ## Run scorecard test
	@echo ... Running the scorecard test
	- operator-sdk scorecard --verbose

##@ Red Hat Certify

bundle:
	@echo --- Updating the bundle directory with latest yamls from olm-catalog ---
	rm -rf bundle/*
	cp -r deploy/olm-catalog/operand-deployment-lifecycle-manager/${CSV_VERSION}/ bundle/
	cp deploy/olm-catalog/operand-deployment-lifecycle-manager/operand-deployment-lifecycle-manager.package.yaml bundle/
	zip bundle/operand-deployment-lifecycle-manager bundle/*.yaml

install-operator-courier:
	@echo --- Installing Operator Courier ---
	pip3 install operator-courier

verify-bundle:
	@echo --- Verify Bundle is Redhat Certify ready ---
	operator-courier --verbose verify --ui_validate_io bundle/

redhat-certify-ready: bundle install-operator-courier verify-bundle

##@ Release

images: build-image build-image-ppc64le build-image-s390x
ifeq ($(LOCAL_OS),Linux)
ifeq ($(LOCAL_ARCH),x86_64)
	@curl -L -o /tmp/manifest-tool https://github.com/estesp/manifest-tool/releases/download/v1.0.0/manifest-tool-linux-amd64
	@chmod +x /tmp/manifest-tool
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(REGISTRY)/$(IMG)-ARCH:$(VERSION) --target $(REGISTRY)/$(IMG) --ignore-missing
	/tmp/manifest-tool push from-args --platforms linux/amd64,linux/ppc64le,linux/s390x --template $(REGISTRY)/$(IMG)-ARCH:$(VERSION) --target $(REGISTRY)/$(IMG):$(VERSION) --ignore-missing
endif
endif

csv: ## Push CSV package to the catalog
	@RELEASE=${CSV_VERSION} common/scripts/push-csv.sh

all: check test coverage build images

##@ Cleanup
clean: ## Clean build binary
	rm -f build/_output/bin/$(IMG)

##@ Help
help: ## Display this help
	@echo "Usage:\n  make \033[36m<target>\033[0m"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all build run check install uninstall code-dev test test-e2e coverage images csv clean help
