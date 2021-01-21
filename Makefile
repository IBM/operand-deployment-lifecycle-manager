# Copyright 2021 IBM Corporation
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

# Dependence tools
KUBECTL ?= $(shell which kubectl)
OPERATOR_SDK ?= $(shell which operator-sdk)
CONTROLLER_GEN ?= $(shell which controller-gen)
KUSTOMIZE ?= $(shell which kustomize)
OPM ?= $(shell which opm)
KIND ?= $(shell which kind)

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin

# Specify whether this repo is build locally or not, default values is '1';
# If set to 1, then you need to also set 'DOCKER_USERNAME' and 'DOCKER_PASSWORD'
# environment variables before build the repo.
BUILD_LOCALLY ?= 1

VCS_URL ?= https://github.com/IBM/operand-deployment-lifecycle-manager
VCS_REF ?= $(shell git rev-parse HEAD)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
RELEASE_VERSION ?= $(shell cat ./version/version.go | grep "Version =" | awk '{ print $$3}' | tr -d '"')
LATEST_VERSION ?= latest

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

# Default image repo
QUAY_REGISTRY ?= quay.io/opencloudio

ifeq ($(BUILD_LOCALLY),0)
ARTIFACTORYA_REGISTRY ?= "hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom"
else
ARTIFACTORYA_REGISTRY ?= "hyc-cloud-private-scratch-docker-local.artifactory.swg-devops.com/ibmcom"
endif

# Current Operator image name
OPERATOR_IMAGE_NAME ?= odlm
# Current Operator bundle image name
BUNDLE_IMAGE_NAME ?= odlm-operator-bundle
# Current Operator version
OPERATOR_VERSION ?= 1.5.0

# Kind cluster name
KIND_CLUSTER_NAME ?= "ODLM"
# Operator image tag for test
OPERATOR_TEST_TAG ?= dev-test

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq ($(BUILD_LOCALLY),0)
    export CONFIG_DOCKER_TARGET = config-docker
    export CONFIG_DOCKER_TARGET_QUAY = config-docker-quay
endif

include common/Makefile.common.mk

##@ Development

check: lint-all ## Check all files lint error

code-dev: ## Run the default dev commands which are the go tidy, fmt, vet then execute the $ make code-gen
	@echo Running the common required commands for developments purposes
	- make code-tidy
	- make code-fmt
	- make code-vet
	@echo Running the common required commands for code delivery
	- make check
	- make test

manager: generate code-fmt code-vet ## Build manager binary
	go build -o bin/manager main.go

run: generate code-fmt code-vet manifests ## Run against the configured Kubernetes cluster in ~/.kube/config
	OPERATOR_NAMESPACE="ibm-common-services" INSTALL_SCOPE="namespaced" go run ./main.go -v=1

install: manifests kustomize ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/manager && $(KUSTOMIZE) edit set image quay.io/opencloudio/odlm=$(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

deploy-e2e: manifests kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/e2e/manager && $(KUSTOMIZE) edit set image quay.io/opencloudio/odlm=$(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG)
	$(KUSTOMIZE) build config/e2e | kubectl apply -f -

##@ Generate code and manifests

manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=operand-deployment-lifecycle-manager webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code e.g. API etc.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

bundle-manifests:
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle \
	-q --overwrite --version $(OPERATOR_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle

generate-all: manifests kustomize operator-sdk ## Generate bundle manifests, metadata and package manifests
	$(OPERATOR_SDK) generate kustomize manifests -q
	- make bundle-manifests CHANNELS=stable-v1,beta DEFAULT_CHANNEL=stable-v1

##@ Test

test: ## Run unit test on prow
	@echo "Running unit tests for the controllers."
	@mkdir -p ${ENVTEST_ASSETS_DIR}
	@test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh \
	|| curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	@test -d ${ENVTEST_ASSETS_DIR}/crds || make fetch-olm-crds
	@source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./controllers/... -coverprofile cover.out
	@rm -rf ${ENVTEST_ASSETS_DIR}


unit-test: generate code-fmt code-vet manifests ## Run unit test
	@make test

e2e-test:
	@echo ... Running the ODLM e2e test
	@go test ./test/e2e/...

e2e-test-kind: build-push-test-operator-image kind-start kind-load-img deploy-e2e e2e-test kind-delete

coverage: ## Run code coverage test
	@echo "Running unit tests for the controllers."
	@mkdir -p ${ENVTEST_ASSETS_DIR}
	@test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh \
	|| curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/master/hack/setup-envtest.sh
	@test -d ${ENVTEST_ASSETS_DIR}/crds || make fetch-olm-crds
	@source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); common/scripts/codecov.sh ${BUILD_LOCALLY} "controllers"
	@rm -rf ${ENVTEST_ASSETS_DIR}

scorecard: operator-sdk ## Run scorecard test
	@echo ... Running the scorecard test
	- $(OPERATOR_SDK) scorecard bundle --verbose

kind-install:
	@common/scripts/install-kind.sh

kind-start: kind-install
	@${KIND} get clusters | grep $(KIND_CLUSTER_NAME)  >/dev/null 2>&1 && \
	echo "KIND Cluster already exists" && exit 0 || \
	echo "Creating KIND Cluster" && \
	${KIND} create cluster --name ${KIND_CLUSTER_NAME} --config=./common/config/kind-config.yaml && \
	common/scripts/install-olm.sh 0.15.1


kind-delete:
	@echo Delete Kind cluster
	@${KIND} delete cluster --name ${KIND_CLUSTER_NAME}

kind-load-img:
	@echo Load ODLM images into Kind cluster
	@docker pull $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG)
	@${KIND} load docker-image $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG) --name ${KIND_CLUSTER_NAME} -v 5

##@ Build

build-operator-image: ## Build the operator image.
	@echo "Building the $(OPERATOR_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker build -t $(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) \
	--build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) \
	--build-arg GOARCH=$(LOCAL_ARCH) -f Dockerfile .

build-test-operator-image: ## Build the operator test image.
	@echo "Building the $(OPERATOR_IMAGE_NAME) docker image for testing..."
	@docker build -t $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG) \
	--build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) \
	--build-arg GOARCH=$(LOCAL_ARCH) -f Dockerfile .

##@ Release

build-push-image: $(CONFIG_DOCKER_TARGET) $(CONFIG_DOCKER_TARGET_QUAY) build-operator-image  ## Build and push the operator images.
	@echo "Pushing the $(OPERATOR_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker tag $(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) $(ARTIFACTORYA_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)
	@docker tag $(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)
	@docker push $(ARTIFACTORYA_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)
	@docker push $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

build-push-test-operator-image: $(CONFIG_DOCKER_TARGET_QUAY) build-test-operator-image  ## Build and push the operator test image.
	@echo "Pushing the $(OPERATOR_IMAGE_NAME) docker image for testing..."
	@docker push $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG)

build-push-bundle-image: $(CONFIG_DOCKER_TARGET_QUAY) build-bundle-image ## Build and push the bundle images.
	@echo "Pushing the $(BUNDLE_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker push $(QUAY_REGISTRY)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

multiarch-image: $(CONFIG_DOCKER_TARGET) $(CONFIG_DOCKER_TARGET_QUAY) ## Generate multiarch images for operator image.
	@MAX_PULLING_RETRY=20 RETRY_INTERVAL=30 common/scripts/multiarch_image.sh $(ARTIFACTORYA_REGISTRY) $(OPERATOR_IMAGE_NAME) $(VERSION) $(RELEASE_VERSION)
	@MAX_PULLING_RETRY=20 RETRY_INTERVAL=30 common/scripts/multiarch_image.sh $(QUAY_REGISTRY) $(OPERATOR_IMAGE_NAME) $(VERSION) $(LATEST_VERSION)

##@ Help
help: ## Display this help
	@echo "Usage:\n  make \033[36m<target>\033[0m"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: test
