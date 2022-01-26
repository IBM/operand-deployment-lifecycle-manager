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

echo ">>> Installing opm"
version=v1.14.0 # latest stable version
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
curl -L https://github.com/operator-framework/operator-registry/releases/download/${version}/linux-${target_os}-${arch} -o opm

# move opm to /usr/local/opm
chmod +x opm
sudo mv opm /usr/local/

export PATH=$PATH:/usr/local/opm
