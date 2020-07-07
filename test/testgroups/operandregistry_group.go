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

// TestOperandRegistry is the test group for testing Operand Registry
func TestOperandRegistry(t *testing.T) {
	t.Run("TestOperandRegistryCRUD", TestOperandRegistryCRUD)
}

// TestOperandRegistryCRUD is for testing OperandRegistry
func TestOperandRegistryCRUD(t *testing.T) {
	assert := assert.New(t)
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global

	// Create namespace for test
	err := helpers.CreateNamespace(f, ctx, config.TestNamespace1)
	assert.NoError(err)

	reg, err := helpers.CreateOperandRegistry(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(reg, "regisgry %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace1)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryReady, config.TestNamespace1)
	assert.NoError(err)

	err = helpers.UpdateOperandRegistry(f, config.TestNamespace1)
	assert.NoError(err)

	err = helpers.DeleteOperandRegistry(f, reg)
	assert.NoError(err)
}
