#!/bin/bash
#
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
#

############################################################
# GKE section
############################################################
PROJECT ?= oceanic-guard-191815
ZONE    ?= us-east5-c
CLUSTER ?= bedrock-prow
NAMESPACESCOPE_VERSION = 1.17.3
OLM_API_VERSION = 0.3.8

activate-serviceaccount:
ifdef GOOGLE_APPLICATION_CREDENTIALS
	gcloud auth activate-service-account --key-file="$(GOOGLE_APPLICATION_CREDENTIALS)" || true
endif

get-cluster-credentials: activate-serviceaccount
	mkdir -p ~/.kube; cp -v /etc/kubeconfig/config ~/.kube; kubectl config use-context default; kubectl get nodes; echo going forward retiring google cloud
	
ifdef GOOGLE_APPLICATION_CREDENTIALS
	gcloud container clusters get-credentials "$(CLUSTER)" --project="$(PROJECT)" --zone="$(ZONE)" || true
endif

config-docker: get-cluster-credentials
	@common/scripts/artifactory_config_docker.sh

# find or download operator-sdk
# download operator-sdk if necessary
operator-sdk:
ifeq (, $(OPERATOR_SDK))
	@./common/scripts/install-operator-sdk.sh
OPERATOR_SDK=/usr/local/bin/operator-sdk
endif

# find or download kubebuilder
# download kubebuilder if necessary
kube-builder:
ifeq (, $(wildcard /usr/local/kubebuilder))
	@./common/scripts/install-kubebuilder.sh
endif

# find or download opm
# download opm if necessary
opm:
ifeq (,$(OPM))
	@./common/scripts/install-opm.sh
endif

fetch-test-crds:
	@{ \
	curl -L -O "https://github.com/operator-framework/api/archive/v${OLM_API_VERSION}.tar.gz" ;\
	tar -zxf v${OLM_API_VERSION}.tar.gz api-${OLM_API_VERSION}/crds && mv api-${OLM_API_VERSION}/crds/* ${ENVCRDS_DIR} ;\
	rm -rf api-${OLM_API_VERSION} v${OLM_API_VERSION}.tar.gz ;\
	}
	@{ \
	curl -L -O "https://github.com/mongodb/mongodb-atlas-kubernetes/archive/refs/tags/v1.7.3.tar.gz" ;\
	tar -zxf v1.7.3.tar.gz mongodb-atlas-kubernetes-1.7.3/deploy/crds && mv mongodb-atlas-kubernetes-1.7.3/deploy/crds/* ${ENVCRDS_DIR} ;\
	rm -rf mongodb-atlas-kubernetes-1.7.3 v1.7.3.tar.gz ;\
	}
	@{ \
	curl -L -O "https://github.com/jaegertracing/jaeger-operator/archive/refs/tags/v1.36.0.tar.gz" ;\
	tar -zxf v1.36.0.tar.gz jaeger-operator-1.36.0/bundle/manifests && mv jaeger-operator-1.36.0/bundle/manifests/jaegertracing.io_jaegers.yaml ${ENVCRDS_DIR}/jaegertracing.io_jaegers.yaml ;\
	rm -rf jaeger-operator-1.36.0 v1.36.0.tar.gz ;\
	}
	@{ \
	curl -L -O "https://github.com/IBM/ibm-namespace-scope-operator/archive/v${NAMESPACESCOPE_VERSION}.tar.gz" ;\
	tar -zxf v${NAMESPACESCOPE_VERSION}.tar.gz ibm-namespace-scope-operator-${NAMESPACESCOPE_VERSION}/bundle/manifests && mv ibm-namespace-scope-operator-${NAMESPACESCOPE_VERSION}/bundle/manifests/operator.ibm.com_namespacescopes.yaml ${ENVCRDS_DIR}/operator.ibm.com_namespacescopes.yaml ;\
	rm -rf ibm-namespace-scope-operator-${NAMESPACESCOPE_VERSION} v${NAMESPACESCOPE_VERSION}.tar.gz ;\
	}
	@{ \
	cp ./controllers/testutil/packagemanifests_crd.yaml ${ENVCRDS_DIR}/packagemanifests_crd.yaml ;\
	}


CONTROLLER_GEN ?= $(shell pwd)/common/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0)

KIND ?= $(shell pwd)/common/bin/kind
kind: ## Download kind locally if necessary.
	$(call go-get-tool,$(KIND),sigs.k8s.io/kind@v0.17.0)

ENVTEST = $(shell pwd)/common/bin/setup-envtest
setup-envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@7b4325d5a38dff0c7eb9a939d079950eafcc4f7e)

FINDFILES=find . \( -path ./.git -o -path ./.github -o -path ./testcrds \) -prune -o -type f
XARGS = xargs -0 ${XARGS_FLAGS}
CLEANXARGS = xargs ${XARGS_FLAGS}

lint-copyright-banner:
	@${FINDFILES} \( -name '*.go' -o -name '*.cc' -o -name '*.h' -o -name '*.proto' -o -name '*.py' -o -name '*.sh' \) \( ! \( -name '*.gen.go' -o -name '*.pb.go' -o -name '*_pb2.py' \) \) -print0 |\
		${XARGS} common/scripts/lint_copyright_banner.sh

lint-go:
	@${FINDFILES} -name '*.go' \( ! \( -name '*.gen.go' -o -name '*.pb.go' \) \) -print0 | ${XARGS} common/scripts/lint_go.sh

lint-all: lint-copyright-banner lint-go

# Run go vet for this project. More info: https://golang.org/cmd/vet/
code-vet:
	@echo go vet
	go vet $$(go list ./...)

# Run go fmt for this project
code-fmt:
	@echo go fmt
	go fmt $$(go list ./...)

# Run go mod tidy to update dependencies
code-tidy:
	@echo go mod tidy
	go mod tidy -v

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
unset GOSUMDB ;\
go env -w GOSUMDB=off ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: code-vet code-fmt code-tidy code-gen lint-copyright-banner lint-go lint-all config-docker operator-sdk kube-builder opm setup-envtest controller-gen fetch-test-crds kustomize kind
