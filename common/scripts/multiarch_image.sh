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

# This script build and push multiarch(amd64, ppc64le and s390x) image for the one specified by
# IMAGE_REPO, IMAGE_NAME and VERSION.
# It assumes the specified image for each platform is already pushed into corresponding docker registry.

ALL_PLATFORMS="amd64 ppc64le s390x"

IMAGE_REPO=${1}
IMAGE_NAME=${2}
VERSION=${3-"$(git describe --exact-match 2> /dev/null || git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)"}
RELEASE_VERSION=${4}
MAX_PULLING_RETRY=${MAX_PULLING_RETRY-10}
RETRY_INTERVAL=${RETRY_INTERVAL-10}
# support other container tools, e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}

# Loop until the image for each single platform is ready in the docker registry.
# TODO: remove this if prow job support dependency.
for arch in ${ALL_PLATFORMS}
do
    for i in $(seq 1 "${MAX_PULLING_RETRY}")
    do
        echo "Checking image '${IMAGE_REPO}'/'${IMAGE_NAME}'-'${arch}':'${VERSION}'..."
        ${CONTAINER_CLI} manifest inspect "${IMAGE_REPO}"/"${IMAGE_NAME}"-"${arch}":"${VERSION}" && break
        sleep "${RETRY_INTERVAL}"
        if [ "${i}" -eq "${MAX_PULLING_RETRY}" ]; then
            echo "Failed to found image '${IMAGE_REPO}'/'${IMAGE_NAME}'-'${arch}':'${VERSION}'!!!"
            exit 1
        fi
    done
done

# create multi-arch manifest
echo "Creating the multi-arch image manifest for ${IMAGE_REPO}/${IMAGE_NAME}:${RELEASE_VERSION}..."
${CONTAINER_CLI} manifest create "${IMAGE_REPO}"/"${IMAGE_NAME}":"${RELEASE_VERSION}" \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-amd64:"${VERSION}" \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-ppc64le:"${VERSION}" \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-s390x:"${VERSION}"
echo "Creating the multi-arch image manifest for ${IMAGE_REPO}/${IMAGE_NAME}:latest..."
${CONTAINER_CLI} manifest create "${IMAGE_REPO}"/"${IMAGE_NAME}":latest \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-amd64:"${VERSION}" \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-ppc64le:"${VERSION}" \
    "${IMAGE_REPO}"/"${IMAGE_NAME}"-s390x:"${VERSION}"

# push multi-arch manifest
echo "Pushing the multi-arch image manifest for ${IMAGE_REPO}/${IMAGE_NAME}:${RELEASE_VERSION}..."
${CONTAINER_CLI} manifest push "${IMAGE_REPO}"/"${IMAGE_NAME}":"${RELEASE_VERSION}"
echo "Pushing the multi-arch image manifest for ${IMAGE_REPO}/${IMAGE_NAME}:latest..."
${CONTAINER_CLI} manifest push "${IMAGE_REPO}"/"${IMAGE_NAME}":latest
