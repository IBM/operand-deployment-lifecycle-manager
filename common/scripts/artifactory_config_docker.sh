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

KUBECTL=$(which kubectl)
DOCKER_REGISTRY="docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-integration-docker-local"
DOCKER_EDGE_REGISTRY="docker-na-public.artifactory.swg-devops.com/hyc-cloud-private-edge-docker-local"
DOCKER_USERNAME=$(${KUBECTL} -n default get secret artifactory-cred -o jsonpath='{.data.username}' | base64 --decode)
DOCKER_PASSWORD=$(${KUBECTL} -n default get secret artifactory-cred -o jsonpath='{.data.password}' | base64 --decode)

# support other container tools, e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}

# login the docker registry
${CONTAINER_CLI} login "${DOCKER_REGISTRY}" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"
${CONTAINER_CLI} login "${DOCKER_EDGE_REGISTRY}" -u "${DOCKER_USERNAME}" -p "${DOCKER_PASSWORD}"
