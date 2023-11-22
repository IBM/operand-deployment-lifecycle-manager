#!/usr/bin/env bash

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

# This script needs to inputs
# The CSV version that is currently in dev

CURRENT_DEV_CSV=$1
NEW_DEV_CSV=$2
PREVIOUS_DEV_CSV=$3

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
	# Linux OS
    # Update bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "/olm.skipRange/s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "s/operand-deployment-lifecycle-manager.v$CURRENT_DEV_CSV/operand-deployment-lifecycle-manager.v$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "s/odlm:$CURRENT_DEV_CSV/odlm:$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "s/version: $CURRENT_DEV_CSV/version: $NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "s/$PREVIOUS_DEV_CSV/$CURRENT_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    echo "Updated the bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml"

    # Update config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "/olm.skipRange/s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "s/odlm:$CURRENT_DEV_CSV/odlm:$NEW_DEV_CSV/g" config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    echo "Updated the config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml"

    sed -i "s/OPERATOR_VERSION ?= $CURRENT_DEV_CSV/OPERATOR_VERSION ?= $NEW_DEV_CSV/g" Makefile
    echo "Updated the Makefile"
    sed -i "s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" version/version.go
    echo "Updated the version/version.go"

elif [[ "$OSTYPE" == "darwin"* ]]; then
    # Mac OSX
    # Update bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "/olm.skipRange/s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "s/operand-deployment-lifecycle-manager.v$CURRENT_DEV_CSV/operand-deployment-lifecycle-manager.v$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "s/odlm:$CURRENT_DEV_CSV/odlm:$NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "s/version: $CURRENT_DEV_CSV/version: $NEW_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "s/$PREVIOUS_DEV_CSV/$CURRENT_DEV_CSV/g" bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    echo "Updated the bundle/manifests/operand-deployment-lifecycle-manager.clusterserviceversion.yaml"

    # Update config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "/olm.skipRange/s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    sed -i "" "s/odlm:$CURRENT_DEV_CSV/odlm:$NEW_DEV_CSV/g" config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml
    echo "Updated the config/manifests/bases/operand-deployment-lifecycle-manager.clusterserviceversion.yaml"

    sed -i "" "s/OPERATOR_VERSION ?= $CURRENT_DEV_CSV/OPERATOR_VERSION ?= $NEW_DEV_CSV/g" Makefile
    echo "Updated the Makefile"
    sed -i "" "s/$CURRENT_DEV_CSV/$NEW_DEV_CSV/g" version/version.go
    echo "Updated the version/version.go"

else
    echo "Not support on other operating system"
fi
