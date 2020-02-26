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
package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"

	apis "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis"
	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/testgroups"
)

func TestOperandRequest(t *testing.T) {
	servicesetList := &operator.OperandRequestList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, servicesetList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("meta-operator", func(t *testing.T) {
		t.Run("Operator", testgroups.MetaOperator)
	})
}
