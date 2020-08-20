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

echo ">>> Installing kubebuilder"
version=2.3.1 # latest stable version
arch=$(uname -m | sed 's/x86_64/amd64/')
local_os=$(uname)
if [[ $local_os == "Linux" ]]; then
    target_os="linux"
elif [[ $local_os == "Darwin" ]]; then
    target_os="darwin"
else
    echo "This system's OS $local_os isn't recognized/supported"
fi

# download the release
curl -L "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${version}/kubebuilder_${version}_${target_os}_${arch}.tar.gz" -o /tmp/kubebuilder_${version}_${target_os}_${arch}.tar.gz

# extract the archive
tar -zxvf /tmp/kubebuilder_${version}_${target_os}_${arch}.tar.gz -C /tmp
mv /tmp/kubebuilder_${version}_${target_os}_${arch} /tmp/kubebuilder && sudo mv /tmp/kubebuilder /usr/local/

# update your PATH to include /usr/local/kubebuilder/bin
export PATH=$PATH:/usr/local/kubebuilder/bin
