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

	"github.com/IBM/operand-deployment-lifecycle-manager/test/helpers"
)

// TestOperandBindInfo is the test group for testing Operand BindInfo
func TestOperandBindInfo(t *testing.T) {
	t.Run("TestOperandBindInfoCRUD", TestOperandBindInfoCRUD)
}

// TestOperandBindInfoCRUD is for testing OperandBindInfo
func TestOperandBindInfoCRUD(t *testing.T) {
	assert := assert.New(t)
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global

	// test create a bindinfo instance
	_, err := helpers.CreateOperandBindInfo(f, ctx)
	assert.NoError(err)

	// test retrieve a bindinfo
	_, err = helpers.RetrieveOperandBindInfo(f, ctx)
	assert.NoError(err)

	// test update bindinfo instance
	bi, err := helpers.UpdateOperandBindInfo(f, ctx)
	assert.NoError(err)
	assert.Equalf("jenkins-operator-base-configuration-example", bi.Spec.Bindings[0].Configmap, "bindinfo(%s/%s) Configmap name should be jenkins-operator-base-configuration-example", bi.Namespace, bi.Name)

	// test delete a bindinfo instance
	err = helpers.DeleteOperandBindInfo(f, bi)
	assert.NoError(err)
}
