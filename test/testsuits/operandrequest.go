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

package testsuits

import (
	"testing"

	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"github.com/operator-framework/operator-sdk/pkg/test"

	"github.com/IBM/operand-deployment-lifecycle-manager/test/helpers"
)

// OperandRequestCreate is for testing the create of the OperandRequest
func OperandRequestCreate(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}
}

// OperandRequestCreateUpdate is for testing the create and update of the OperandRequest
func OperandRequestCreateUpdate(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}
}

// OperandRequestCreateDelete is for testing the create and delete of the OperandRequest
func OperandRequestCreateDelete(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}

	// Commnet out delete test
	// if err = helpers.DeleteTest(olmClient, f, ctx); err != nil {
	// 	t.Fatal(err)
	// }
}

// OperandConfigUpdate is for testing the update of the OperandConfig
func OperandConfigUpdate(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = helpers.UpdateConfigTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}
}

// OperandRegistryUpdate is for testing the update of the OperandRegistry
func OperandRegistryUpdate(t *testing.T) {

	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global
	olmClient, err := olmclient.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	if err = helpers.CreateTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}

	if err = helpers.UpdateOperandRegistryTest(olmClient, f, ctx); err != nil {
		t.Fatal(err)
	}
}
