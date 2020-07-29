//
// Copyright 2020 IBM Corporation
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

package testgroups

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"

	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/helpers"
)

// TestOperandBindInfo is the test group for testing Operand BindInfo
func TestOperandBindInfo(t *testing.T) {
	t.Run("TestOperandBindInfoCRUD", TestOperandBindInfoCRUD)
}

// TestOperandBindInfoCRUD is for testing OperandBindInfo
// 1. Create namespace e2e-test-ns-1.
// 2. Create OperandBindInfo e2e-test-ns-1/jenkins-public-bindinfo and ensure its status being Initialized.
// 3. Update OperandBindInfo e2e-test-ns-1/jenkins-public-bindinfo
// 4. Delete OperandBindInfo e2e-test-ns-1/jenkins-public-bindinfo
func TestOperandBindInfoCRUD(t *testing.T) {
	assert := assert.New(t)
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global

	// Step1: Create namespace for testing
	err := helpers.CreateNamespace(f, ctx, config.TestNamespace1)
	assert.NoError(err)

	// Step2: Create OperandBindInfo
	bi, err := helpers.CreateOperandBindInfo(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(bi, "bindinfo %s should be created in namespace %s", config.OperandBindInfoCrName, config.TestNamespace1)

	_, err = helpers.WaitBindInfoStatus(f, operator.BindInfoInit, config.TestNamespace1)
	assert.NoError(err)

	// Step3: Update OperandBindInfo
	bi, err = helpers.UpdateOperandBindInfo(f, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf("jenkins-operator-base-configuration-example", bi.Spec.Bindings["public"].Configmap, "bindinfo(%s/%s) Configmap name should be jenkins-operator-base-configuration-example", bi.Namespace, bi.Name)

	// Step4: Delete OperandBindInfo
	err = helpers.DeleteOperandBindInfo(f, bi)
	assert.NoError(err)
}
