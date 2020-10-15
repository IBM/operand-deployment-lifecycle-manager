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
#

############################################################
# GKE section
############################################################
PROJECT ?= oceanic-guard-191815
ZONE    ?= us-west1-a
CLUSTER ?= prow

activate-serviceaccount:
ifdef GOOGLE_APPLICATION_CREDENTIALS
	gcloud auth activate-service-account --key-file="$(GOOGLE_APPLICATION_CREDENTIALS)"
endif

get-cluster-credentials: activate-serviceaccount
	gcloud container clusters get-credentials "$(CLUSTER)" --project="$(PROJECT)" --zone="$(ZONE)"

config-docker: get-cluster-credentials
	@common/scripts/artifactory_config_docker.sh

config-docker-quay: get-cluster-credentials
	@common/scripts/quay_config_docker.sh

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

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(CONTROLLER_GEN))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
endif

fetch-olm-crds:
	@{ \
	curl -L -O "https://github.com/operator-framework/api/archive/v0.3.8.tar.gz" ;\
	tar -zxf v0.3.8.tar.gz api-0.3.8/crds && mv api-0.3.8/crds ${ENVTEST_ASSETS_DIR}/crds ;\
	rm -rf api-0.3.8 v0.3.8.tar.gz ;\
	}
	@{ \
	curl -L -O "https://github.com/redhat-developer/jenkins-operator/archive/v0.3.3.tar.gz" ;\
	tar -zxf v0.3.3.tar.gz jenkins-operator-0.3.3/deploy/crds && mv jenkins-operator-0.3.3/deploy/crds/jenkins_v1alpha2_jenkins_crd.yaml ${ENVTEST_ASSETS_DIR}/crds/jenkins_v1alpha2_jenkins_crd.yaml ;\
	rm -rf jenkins-operator-0.3.3 v0.3.3.tar.gz ;\
	}
	@{ \
	curl -L -O "https://github.com/horis233/etcd-operator/archive/v0.9.4-crd.tar.gz" ;\
	tar -zxf v0.9.4-crd.tar.gz etcd-operator-0.9.4-crd/deploy/crds && mv etcd-operator-0.9.4-crd/deploy/crds/etcdclusters.etcd.database.coreos.com.crd.yaml ${ENVTEST_ASSETS_DIR}/crds/etcdclusters.etcd.database.coreos.com.crd.yaml ;\
	rm -rf etcd-operator-0.9.4-crd v0.9.4-crd.tar.gz ;\
	}

# find or download kustomize
# download kustomize if necessary
kustomize:
ifeq (, $(KUSTOMIZE))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
endif

opm:
ifeq (,$(OPM))
	@./common/scripts/install-opm.sh
endif

FINDFILES=find . \( -path ./.git -o -path ./.github -o -path ./testbin \) -prune -o -type f
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
	go vet $$(go list ./... )

# Run go fmt for this project
code-fmt:
	@echo go fmt
	go fmt $$(go list ./... )

# Run go mod tidy to update dependencies
code-tidy:
	@echo go mod tidy
	go mod tidy -v

.PHONY: code-vet code-fmt code-tidy code-gen lint-copyright-banner lint-go lint-all config-docker operator-sdk kube-builder opm controller-gen fetch-olm-crds kustomize
