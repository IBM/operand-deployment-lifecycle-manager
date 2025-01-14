# Copyright 2022 IBM Corporation
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
OPM ?= $(shell which opm)
KUSTOMIZE ?= $(shell which kustomize)
KUSTOMIZE_VERSION=v3.8.7

ENVCRDS_DIR=$(shell pwd)/testcrds

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
OPERATOR_SDK_VERSION=v1.32.0
YQ_VERSION=v4.42.1
DEFAULT_CHANNEL ?= v$(shell cat ./version/version.go | grep "Version =" | awk '{ print $$3}' | tr -d '"' | cut -d '.' -f1,2)
CHANNELS ?= $(DEFAULT_CHANNEL)

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
QUAY_REGISTRY ?= quay.io/luzarragaben

ifeq ($(BUILD_LOCALLY),0)
ARTIFACTORYA_REGISTRY ?= "docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-integration-docker-local/ibmcom"
else
ARTIFACTORYA_REGISTRY ?= "docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-scratch-docker-local/ibmcom"
endif

ifdef DEV_REGISTRY
DEV_REGISTRY := $(DEV_REGISTRY)
else
DEV_REGISTRY := ${QUAY_REGISTRY}
endif

# Current Operator image name
OPERATOR_IMAGE_NAME ?= odlm
# Current Operator bundle image name
BUNDLE_IMAGE_NAME ?= odlm-operator-bundle
# Current Operator version
OPERATOR_VERSION ?= 4.3.9

# Kind cluster name
KIND_CLUSTER_NAME ?= "odlm"
# Operator image tag for test
OPERATOR_TEST_TAG ?= nolm-controller3

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

ifeq ($(BUILD_LOCALLY),0)
    export CONFIG_DOCKER_TARGET = config-docker
endif

include common/Makefile.common.mk

##@ Development

check: lint-all ## Check all files lint error

yq: ## Install yq, a yaml processor
ifneq ($(shell yq -V | cut -d ' ' -f 3 | cut -d '.' -f 1 ), 4)
	@{ \
	if [ v$(shell ./bin/yq --version | cut -d ' ' -f3) != $(YQ_VERSION) ]; then\
		set -e ;\
		mkdir -p bin ;\
		echo "Downloading yq ...";\
		curl -sSLO https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(LOCAL_OS)_$(LOCAL_ARCH);\
		mv yq_$(LOCAL_OS)_$(LOCAL_ARCH) ./bin/yq ;\
		chmod +x ./bin/yq ;\
	fi;\
	}
YQ=$(realpath ./bin/yq)
else
YQ=$(shell which yq)
endif

kustomize: ## Install kustomize
ifeq (, $(shell which kustomize 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p bin ;\
	echo "Downloading kustomize ...";\
	curl -sSLo - https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_$(LOCAL_OS)_$(LOCAL_ARCH).tar.gz | tar xzf - -C bin/ ;\
	}
KUSTOMIZE=$(realpath ./bin/kustomize)
else
KUSTOMIZE=$(shell which kustomize)
endif

operator-sdk:
ifneq ($(shell operator-sdk version | cut -d ',' -f1 | cut -d ':' -f2 | tr -d '"' | xargs | cut -d '.' -f1), v1)
	@{ \
	if [ "$(shell ./bin/operator-sdk version | cut -d ',' -f1 | cut -d ':' -f2 | tr -d '"' | xargs)" != $(OPERATOR_SDK_VERSION) ]; then \
		set -e ; \
		mkdir -p bin ;\
		echo "Downloading operator-sdk..." ;\
		curl -sSLo ./bin/operator-sdk "https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$(LOCAL_OS)_$(LOCAL_ARCH)" ;\
		chmod +x ./bin/operator-sdk ;\
	fi ;\
	}
OPERATOR_SDK=$(realpath ./bin/operator-sdk)
else
OPERATOR_SDK=$(shell which operator-sdk)
endif

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

deploy-e2e: kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/e2e/manager && $(KUSTOMIZE) edit set image quay.io/opencloudio/odlm=$(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG)
	$(KUSTOMIZE) build config/e2e | kubectl apply -f -

##@ Generate code and manifests

manifests: controller-gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) crd rbac:roleName=operand-deployment-lifecycle-manager webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code e.g. API etc.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

bundle-manifests: yq
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle \
	-q --overwrite --version $(OPERATOR_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
	$(YQ) eval-all -i '.spec.relatedImages |= load("config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml").spec.relatedImages' bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
	@# Need to replace fields this way to avoid changing PROJECT name and CSV file name, which may or may not impact CICD automation
	$(YQ) e -i '.annotations["operators.operatorframework.io.bundle.package.v1"] = "ibm-odlm"' bundle/metadata/annotations.yaml
	sed -i'' s/operand-deployment-lifecycle-manager/ibm-odlm/ bundle.Dockerfile

generate-all: yq manifests kustomize operator-sdk ## Generate bundle manifests, metadata and package manifests
	$(OPERATOR_SDK) generate kustomize manifests -q
	- make bundle-manifests CHANNELS=v4.3 DEFAULT_CHANNEL=v4.3

##@ Test

test: setup-envtest ## Run unit test on prow
	@echo "Running unit tests for the controllers."
	@mkdir -p ${ENVCRDS_DIR}
	@make fetch-test-crds
	@$(ENVTEST) use 1.21
	@OPERATOR_NAMESPACE="ibm-operators" go test ./controllers/... -coverprofile cover.out
	@rm -rf ${ENVCRDS_DIR}


unit-test: generate code-fmt code-vet manifests ## Run unit test
	@make test

e2e-test:
	@echo ... Running the ODLM e2e test
	@go test ./test/e2e/...

e2e-test-kind: build-test-operator-image kind-start kind-load-img deploy-e2e e2e-test kind-delete

scorecard: operator-sdk ## Run scorecard test
	@echo ... Running the scorecard test
	- $(OPERATOR_SDK) scorecard bundle --verbose

kind-start: kind
	@${KIND} get clusters | grep $(KIND_CLUSTER_NAME)  >/dev/null 2>&1 && \
	echo "KIND Cluster already exists" && exit 0 || \
	echo "Creating KIND Cluster" && \
	${KIND} create cluster --name ${KIND_CLUSTER_NAME} --config=./common/config/kind-config.yaml && \
	common/scripts/install-olm.sh v0.24.0


kind-delete:
	@echo Delete Kind cluster
	@${KIND} delete cluster --name ${KIND_CLUSTER_NAME}

kind-load-img:
	@echo Load ODLM images into Kind cluster
	@${KIND} load docker-image $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG) --name ${KIND_CLUSTER_NAME} -v 5

##@ Build

build-operator-image: $(CONFIG_DOCKER_TARGET) ## Build the operator image.
	@echo "Building the $(OPERATOR_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker build -t $(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) \
	--build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) \
	--build-arg GOARCH=$(LOCAL_ARCH) -f Dockerfile .

build-operator-dev-image: ## Build the operator dev image.
	@echo "Building the $(DEV_REGISTRY)/$(OPERATOR_IMAGE_NAME) docker image..."
	@docker build -t $(DEV_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(VERSION) \
	--build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) \
	--build-arg GOARCH=$(LOCAL_ARCH) -f Dockerfile .

build-test-operator-image: $(CONFIG_DOCKER_TARGET) ## Build the operator test image.
	@echo "Building the $(OPERATOR_IMAGE_NAME) docker image for testing..."
	@docker build -t $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_TEST_TAG) \
	--build-arg VCS_REF=$(VCS_REF) --build-arg VCS_URL=$(VCS_URL) \
	--build-arg GOARCH=$(LOCAL_ARCH) -f Dockerfile .

##@ Release

build-push-dev-image: build-operator-dev-image  ## Build and push the operator dev images.
	@echo "Pushing the $(DEV_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(VERSION) docker image to $(DEV_REGISTRY)..."
	@docker push $(DEV_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(VERSION)

build-push-image: $(CONFIG_DOCKER_TARGET) build-operator-image  ## Build and push the operator images.
	@echo "Pushing the $(OPERATOR_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker tag $(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) $(ARTIFACTORYA_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)
	@docker push $(ARTIFACTORYA_REGISTRY)/$(OPERATOR_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

build-push-bundle-image: yq
	@docker build -f bundle.Dockerfile -t $(QUAY_REGISTRY)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) .
	@echo "Pushing the $(BUNDLE_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	@docker push $(QUAY_REGISTRY)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION)

build-catalog-source:
	@opm -u docker index add --bundles $(QUAY_REGISTRY)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) --tag $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME)-catalog:$(VERSION)
	@docker push $(QUAY_REGISTRY)/$(OPERATOR_IMAGE_NAME)-catalog:$(VERSION)

build-catalog: build-push-bundle-image build-catalog-source

multiarch-image: $(CONFIG_DOCKER_TARGET) ## Generate multiarch images for operator image.
	@MAX_PULLING_RETRY=20 RETRY_INTERVAL=30 common/scripts/multiarch_image.sh $(ARTIFACTORYA_REGISTRY) $(OPERATOR_IMAGE_NAME) $(VERSION) $(RELEASE_VERSION)

run-bundle:
	$(OPERATOR_SDK) run bundle $(QUAY_REGISTRY)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):$(VERSION) --install-mode OwnNamespace

cleanup-bundle:
	$(OPERATOR_SDK) cleanup ibm-odlm

##@ Help
help: ## Display this help
	@echo "Usage:\n  make \033[36m<target>\033[0m"
	@awk 'BEGIN {FS = ":.*##"}; \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: test
