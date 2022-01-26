#!/bin/bash
#
# Copyright 2022 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# create zip file containing the bundle to submit for Red Hat certification
# the bundle consists of package.yaml, clusterserviceversion.yaml, crd.yaml
# run as 'scripts/create-bundle.sh'

if [ -d "./bundle" ] 
then
    echo "cleanup bundle directory" 
    rm bundle/*.yaml
    rm bundle/*.zip
else
    echo "create bundle directory"
    mkdir bundle
fi

cp -p deploy/olm-catalog/operand-deployment-lifecycle-manager/operand-deployment-lifecycle-manager.package.yaml bundle/
cp -p deploy/olm-catalog/operand-deployment-lifecycle-manager/"$1"/*.yaml bundle/

cd bundle || exit
zip operand-deployment-lifecycle-manager ./*.yaml 
