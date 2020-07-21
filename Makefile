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

KUBECTL ?= $(shell command -v kubectl)
OPERATOR_SDK ?= $(shell command -v operator-sdk)

# Specify whether this repo is build locally or not, default values is '1';
# If set to 1, then you need to also set 'DOCKER_USERNAME' and 'DOCKER_PASSWORD'
# environment variables before build the repo.
BUILD_LOCALLY ?= 1

VCS_URL ?= https://github.com/IBM/operand-deployment-lifecycle-manager
VCS_REF ?= $(shell git rev-parse HEAD)

# The namespcethat operator will be deployed in
NAMESPACE=ibm-common-services

# Image URL to use all building/pushing image targets;
# Use your own docker registry and image name for dev/test by overridding the
# IMAGE_REPO, IMAGE_NAME and RELEASE_TAG environment variable.
IMAGE_REPO ?= quay.io/opencloudio
IMAGE_NAME ?= odlm
BUNDLE_IMAGE_NAME ?= odlm-operator-bundle
BUNDLE_IMAGE_VERSION ?= latest
BUNDLE_MANIFESTS_PATH ?= manifests
INDEX_IMAGE_NAME ?= odlm-catalog
INDEX_IMAGE_VERSION ?= latest
RELEASED_VERSION ?= 1.2.3
STABLE_VERSION ?= 1.2.3
BETA_VERSION ?= 1.2.4
DEV_VERSION ?= 1.2.5
CHANNELS ?= dev
DEFAULT_CHANNEL ?= dev
CSV_VERSION ?= 1.2.5

QUAY_USERNAME ?=
QUAY_PASSWORD ?=

MARKDOWN_LINT_WHITELIST=https://quay.io/cnr

TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
				git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)

LOCAL_OS := $(shell uname)
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

ARCH := $(shell uname -m)
LOCAL_ARCH := "amd64"
ifeq ($(ARCH),x86_64)
    LOCAL_ARCH="amd64"
else ifeq ($(ARCH),ppc64le)
    LOCAL_ARCH="ppc64le"
else ifeq ($(ARCH),s390x)
    LOCAL_ARCH="s390x"
else
    $(error "This system's ARCH $(ARCH) isn't recognized/supported")
endif

include common/Makefile.common.mk

##@ Application

install: ## Install all resources (CR/CRD's, RBAC and Operator)
	@echo ....... Set environment variables ......
	- export WATCH_NAMESPACE=${NAMESPACE}
	@echo ....... Creating namespace .......
	- $(KUBECTL) create namespace ${NAMESPACE}
	@echo ....... Applying CRDs .......
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_operandregistries_crd.yaml
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_operandconfigs_crd.yaml
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_operandrequests_crd.yaml
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_operandbindinfos_crd.yaml
	@echo ....... Applying RBAC .......
	- $(KUBECTL) apply -f deploy/service_account.yaml -n ${NAMESPACE}
	- $(KUBECTL) apply -f deploy/role.yaml
	- $(KUBECTL) apply -f deploy/role_binding.yaml
	# @echo ....... Applying Operator .......
	# - $(KUBECTL) apply -f deploy/operator.yaml -n ${NAMESPACE}
	@echo ....... Creating the Instances .......
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_v1alpha1_operandregistry_cr.yaml -n ${NAMESPACE}
	- $(KUBECTL) apply -f deploy/crds/operator.ibm.com_v1alpha1_operandconfig_cr.yaml -n ${NAMESPACE}
uninstall: ## Uninstall all that all performed in the $ make install
	@echo ....... Uninstalling .......
	@echo ....... Deleting the Instances .......
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_v1alpha1_operandrequest_cr.yaml -n ${NAMESPACE} --ignore-not-found
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_v1alpha1_operandregistry_cr.yaml -n ${NAMESPACE} --ignore-not-found
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_v1alpha1_operandconfig_cr.yaml -n ${NAMESPACE} --ignore-not-found
	@echo ....... Deleting Operator .......
	- $(KUBECTL) delete -f deploy/operator.yaml -n ${NAMESPACE} --ignore-not-found
	@echo ....... Deleting CRDs .......
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_operandbindinfos_crd.yaml --ignore-not-found
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_operandrequests_crd.yaml --ignore-not-found
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_operandconfigs_crd.yaml --ignore-not-found
	- $(KUBECTL) delete -f deploy/crds/operator.ibm.com_operandregistries_crd.yaml --ignore-not-found
	@echo ....... Deleting RBAC .......
	- $(KUBECTL) delete -f deploy/role_binding.yaml --ignore-not-found
	- $(KUBECTL) delete -f deploy/service_account.yaml -n ${NAMESPACE} --ignore-not-found
	- $(KUBECTL) delete -f deploy/role.yaml --ignore-not-found
	@echo ....... Deleting namespace ${NAMESPACE}.......
	- $(KUBECTL) delete namespace ${NAMESPACE} --ignore-not-found

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
	@echo ....... Start Operator locally with go run ......
	WATCH_NAMESPACE= go run ./cmd/manager/main.go -v=2 --zap-encoder=console

ifeq ($(BUILD_LOCALLY),0)
    export CONFIG_DOCKER_TARGET = config-docker
endif

##@ Build

build:
	@echo "Building the $(IMAGE_NAME) binary for $(LOCAL_ARCH)..."
	@GOARCH=$(LOCAL_ARCH) common/scripts/gobuild.sh build/_output/bin/$(IMAGE_NAME) ./cmd/manager
	@strip $(STRIP_FLAGS) build/_output/bin/$(IMAGE_NAME)

build-push-image: push-image

build-image: build
	@echo "Building the $(IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker build -t $(IMAGE_REPO)/$(IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) --build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) -f build/Dockerfile .

push-image: $(CONFIG_DOCKER_TARGET) build-image
	@echo "Pushing the $(IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker push $(IMAGE_REPO)/$(IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

# Build and push operator bundle image
build-push-bundle-image:
	@echo "Bulding the $(BUNDLE_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- $(OPERATOR_SDK) bundle create $(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(BUNDLE_IMAGE_VERSION) \
	--directory deploy/olm-catalog/operand-deployment-lifecycle-manager/$(BUNDLE_MANIFESTS_PATH) \
	--package operand-deployment-lifecycle-manager-app \
	--channels $(CHANNELS) \
	--default-channel $(DEFAULT_CHANNEL) \
	--overwrite
	@echo "Pushing the $(BUNDLE_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- docker push $(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(BUNDLE_IMAGE_VERSION)

# Build and push index image
build-push-index-image: build-push-bundle-image
	@echo "Building the $(INDEX_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- opm index add --permissive -c docker \
	--bundles $(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(BUNDLE_IMAGE_VERSION) \
	--from-index $(IMAGE_REPO)/$(INDEX_IMAGE_NAME)-$(LOCAL_ARCH):$(FROM_INDEX_IMAGE_VERSION) \
	--tag $(IMAGE_REPO)/$(INDEX_IMAGE_NAME)-$(LOCAL_ARCH):$(NEW_INDEX_IMAGE_VERSION)
	@echo "Pushing the $(INDEX_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- docker push $(IMAGE_REPO)/$(INDEX_IMAGE_NAME)-$(LOCAL_ARCH):$(NEW_INDEX_IMAGE_VERSION)

build-dev-index-image: ## Build and push dev index image
	CHANNELS=dev \
	DEFAULT_CHANNEL=dev \
	FROM_INDEX_IMAGE_VERSION=$(BETA_VERSION) \
	NEW_INDEX_IMAGE_VERSION=$(DEV_VERSION) \
	BUNDLE_IMAGE_VERSION=$(DEV_VERSION) \
	BUNDLE_MANIFESTS_PATH=manifests \
	make build-push-index-image

build-beta-index-image: ## Build and push beta index image
	CHANNELS=beta \
	DEFAULT_CHANNEL=beta \
	FROM_INDEX_IMAGE_VERSION=$(STABLE_VERSION) \
	NEW_INDEX_IMAGE_VERSION=$(BETA_VERSION) \
	BUNDLE_IMAGE_VERSION=$(BETA_VERSION) \
	BUNDLE_MANIFESTS_PATH=$(BETA_VERSION) \
	make build-push-index-image


build-stable-index-image: ## Build and push stable index image
	CHANNELS=stable-v1 \
	DEFAULT_CHANNEL=stable-v1 \
	FROM_INDEX_IMAGE_VERSION=$(RELEASED_VERSION) \
	NEW_INDEX_IMAGE_VERSION=$(STABLE_VERSION) \
	BUNDLE_IMAGE_VERSION=$(STABLE_VERSION) \
	BUNDLE_MANIFESTS_PATH=$(STABLE_VERSION) \
	make build-push-index-image

##@ Test

test: ## Run unit test
	@echo "Running the tests for $(IMAGE_NAME) on $(LOCAL_ARCH)..."
	@go test $(TESTARGS) ./pkg/controller/...

test-e2e: ## Run integration e2e tests with different options.
	@echo ....... Creating namespace .......
	- $(KUBECTL) create namespace e2e-test-op-ns
	@echo ... Running e2e tests on locally ...
	- $(OPERATOR_SDK) test local ./test/e2e --verbose --up-local --namespace=e2e-test-op-ns
	@echo ....... Deleting namespace .......
	- $(KUBECTL) delete namespace e2e-test-op-ns

coverage: ## Run code coverage test
	@common/scripts/codecov.sh ${BUILD_LOCALLY} "pkg/controller"

scorecard: ## Run scorecard test
	@echo ... Running the scorecard test
	- $(OPERATOR_SDK) scorecard --verbose

##@ Release

multiarch-image: $(CONFIG_DOCKER_TARGET)
	@MAX_PULLING_RETRY=20 RETRY_INTERVAL=30 common/scripts/multiarch_image.sh $(IMAGE_REPO) $(IMAGE_NAME) $(VERSION)

csv: ## Push CSV package to the catalog
	@RELEASE=${CSV_VERSION} common/scripts/push-csv.sh

all: check test coverage build images

##@ Cleanup
clean: ## Clean build binary
	rm -f build/_output/bin/$(IMAGE_NAME)

##@ Help
help: ## Display this help
	@echo "Usage:\n  make \033[36m<target>\033[0m"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: all build run check install uninstall code-dev test test-e2e coverage build multiarch-image csv clean help
