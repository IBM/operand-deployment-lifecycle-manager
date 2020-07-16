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

// TestOperandConfig is the test group for testing Operand Config
func TestOperandConfig(t *testing.T) {
	t.Run("TestOperandConfigCRUD", TestOperandConfigCRUD)
}

// TestOperandConfigCRUD is for testing OperandConfig
// 1. Create namespace e2e-test-ns-1.
// 2. Create OperandConfig e2e-test-ns-1/common-service and ensure its status being Initialized.
// 3. Update OperandConfig e2e-test-ns-1/common-service
// 4. Delete OperandConfig e2e-test-ns-1/common-service
func TestOperandConfigCRUD(t *testing.T) {
	assert := assert.New(t)
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global

	// Step1: Create namespace for testing
	err := helpers.CreateNamespace(f, ctx, config.TestNamespace1)
	assert.NoError(err)

	// Step2: Create OperandConfig
	con, err := helpers.CreateOperandConfig(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(con, "config %s should be created in namespace %s", config.OperandConfigCrName, config.TestNamespace1)

	con, err = helpers.WaitConfigStatus(f, operator.ServiceInit, config.TestNamespace1)
	assert.NoError(err)

	// Step3: Update OperandConfig
	err = helpers.UpdateOperandConfig(f, config.TestNamespace1)
	assert.NoError(err)

	// Step4: Delete OperandConfig
	err = helpers.DeleteOperandConfig(f, con)
	assert.NoError(err)
}
