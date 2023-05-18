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

package e2e

import (
	"time"
)

const (
	// APIRetry defines the frequency at which we check for updates against the
	// k8s api when waiting for a specific condition to be true.
	APIRetry = time.Second * 5

	// APITimeout defines the amount of time we should spend querying the k8s api
	// when waiting for a specific condition to be true.
	APITimeout = time.Minute * 5

	// CleanupRetry is the interval at which test framework attempts cleanup
	CleanupRetry = time.Second * 10

	// CleanupTimeout is the wait time for test framework cleanup
	CleanupTimeout = time.Second * 180

	// WaitForTimeout is the wait time for cluster resource
	WaitForTimeout = time.Minute * 5

	// WaitForRetry is the the interval at checking cluster resource
	WaitForRetry = time.Second * 10

	// TestOperatorName specifies the name of the operator being tested
	TestOperatorName = "operand-deployment-lifecycle-manager"

	// OperandRequestCrName specifies the name of the custom resource of the OperandRequest
	OperandRequestCrName = "ibm-cloudpak-name"

	// OperandRegistryCrName specifies the name of the custom resource of the OperandRegistry
	OperandRegistryCrName = "common-service"

	// OperandConfigCrName specifies the name of the custom resource of the OperandConfig
	OperandConfigCrName = "common-service"

	// OperandBindInfoCrName specifies the name of the custom resource of the OperandBindInfo
	OperandBindInfoCrName = "mongodb-public-bindinfo"

	// OperandRequestNamespace1 specifies the namespace of the OperandRequest
	OperandRequestNamespace1 = "ibm-cloudpak-1"

	// OperandRequestNamespace2 specifies the namespace of the OperandRequest
	OperandRequestNamespace2 = "ibm-cloudpak-2"

	// OperandRegistryNamespace specifies the namespace of the OperandRegistry
	OperandRegistryNamespace = "ibm-common-services"

	// OperatorNamespace specifies the namespace of the generated operators
	OperatorNamespace = "ibm-operators"
)
