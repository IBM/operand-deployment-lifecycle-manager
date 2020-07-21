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
	@common/scripts/config_docker.sh

install-operator-sdk:
	@operator-sdk version 2> /dev/null ; if [ $$? -ne 0 ]; then ./common/scripts/install-operator-sdk.sh; fi

FINDFILES=find . \( -path ./.git -o -path ./.github \) -prune -o -type f
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

# Run the operator-sdk commands to generated code (k8s and openapi and csv)
code-gen:
	@echo Updating the deep copy files with the changes in the API
	$(OPERATOR_SDK) generate k8s
	@echo Updating the CRD files with the OpenAPI validations
	$(OPERATOR_SDK) generate crds
	@echo Updating/Generating a ClusterServiceVersion YAML manifest for the operator
	$(OPERATOR_SDK) generate csv --csv-version ${CSV_VERSION} --update-crds

# Use released manifests to build stable index image
build-released-index-image:
	@echo "Building the $(INDEX_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- CHANNELS=stable-v1 DEFAULT_CHANNEL=stable-v1 BUNDLE_IMAGE_VERSION=1.1.0 BUNDLE_MANIFESTS_PATH=1.1.0 make build-push-bundle-image
	- CHANNELS=stable-v1 DEFAULT_CHANNEL=stable-v1 BUNDLE_IMAGE_VERSION=1.2.0 BUNDLE_MANIFESTS_PATH=1.2.0 make build-push-bundle-image
	- CHANNELS=stable-v1 DEFAULT_CHANNEL=stable-v1 BUNDLE_IMAGE_VERSION=1.2.1 BUNDLE_MANIFESTS_PATH=1.2.1 make build-push-bundle-image
	- CHANNELS=stable-v1 DEFAULT_CHANNEL=stable-v1 BUNDLE_IMAGE_VERSION=1.2.2 BUNDLE_MANIFESTS_PATH=1.2.2 make build-push-bundle-image
	- CHANNELS=stable-v1 DEFAULT_CHANNEL=stable-v1 BUNDLE_IMAGE_VERSION=1.2.3 BUNDLE_MANIFESTS_PATH=1.2.3 make build-push-bundle-image
	- opm index add --permissive -c docker --bundles \
	$(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):1.1.0,\
	$(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):1.2.0,\
	$(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):1.2.1,\
	$(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):1.2.2,\
	$(IMAGE_REPO)/$(BUNDLE_IMAGE_NAME)-$(LOCAL_ARCH):1.2.3 \
	--tag $(IMAGE_REPO)/$(INDEX_IMAGE_NAME)-$(LOCAL_ARCH):$(RELEASED_VERSION)
	@echo "Pushing the $(INDEX_IMAGE_NAME) docker image for $(LOCAL_ARCH)..."
	- docker push $(IMAGE_REPO)/$(INDEX_IMAGE_NAME)-$(LOCAL_ARCH):$(RELEASED_VERSION)

bundle:
	@common/scripts/create_bundle.sh ${CSV_VERSION}

.PHONY: code-vet code-fmt code-tidy code-gen csv-gen lint-copyright-banner lint-go lint-all config-docker install-operator-sdk bundle
