//
// Copyright 2022 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package testutil

import (
	"time"
)

const (
	Timeout  = time.Second * 300
	Interval = time.Second * 5
)

const JaegerExample string = `
[
	{
	  "apiVersion": "jaegertracing.io/v1",
	  "kind": "Jaeger",
	  "metadata": {
		"name": "my-jaeger"
	  },
	  "spec": {
		"strategy": "allinone"
	  }
	}
]
`
const MongodbExample string = `
[
	{
	  "apiVersion": "atlas.mongodb.com/v1",
	  "kind": "AtlasDeployment",
	  "metadata": {
		"name": "my-atlas-deployment"
	  },
	  "spec": {
		"deploymentSpec": {
			"name": "test-deployment",
			"providerSettings": {
			  "instanceSizeName": "M10",
			  "providerName": "AWS",
			  "regionName": "US_EAST_1"
			}
		},
		"projectRef": {"name": "my-project"}
	  }
	}
]
`
