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

	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"

	"github.com/operator-framework/operator-sdk/pkg/test"

	"github.com/IBM/operand-deployment-lifecycle-manager/test/helpers"
)

// TestOperandRegistry is the test group for testing Operand Registry
func TestOperandRegistry(t *testing.T) {
	t.Run("TestOperandRegistryCURD", TestOperandRegistryCURD)
}

// TestOperandRegistryCURD is for testing OperandRegistry
func TestOperandRegistryCURD(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateOperandRegistry(f, ctx); err != nil {
		t.Fatal(err)
	}

	ri, err := helpers.GetOperandRegistry(olmClient, f, ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.DeleteOperandRegistry(olmClient, ri, f); err != nil {
		t.Fatal(err)
	}
}
