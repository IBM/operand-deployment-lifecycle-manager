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

	"github.com/operator-framework/operator-sdk/pkg/test"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"github.com/stretchr/testify/assert"

	apis "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis"
	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/testgroups"
)

func TestODLM(t *testing.T) {
	assert := assert.New(t)
	requestList := &operator.OperandRequestList{}
	registryList := &operator.OperandRegistryList{}
	configList := &operator.OperandConfigList{}
	bindinfoList := &operator.OperandBindInfoList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, requestList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, registryList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, configList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, bindinfoList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	t.Parallel()
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	err := deployOperator(t, ctx)
	assert.NoError(err)

	// Run group test
	t.Run("TestOperandRegistry", testgroups.TestOperandRegistry)
	t.Run("TestOperandConfig", testgroups.TestOperandConfig)
	t.Run("TestOperandBindInfo", testgroups.TestOperandBindInfo)
	t.Run("TestOperandRequest", testgroups.TestOperandRequest)
}

func deployOperator(t *testing.T, ctx *test.TestCtx) error {
	err := ctx.InitializeClusterResources(
		&test.CleanupOptions{
			TestContext:   ctx,
			Timeout:       config.CleanupTimeout,
			RetryInterval: config.CleanupRetry,
		},
	)
	if err != nil {
		t.Fatalf("failed to initialize cluster resources")
		return err
	}

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatalf("failed to get the namespace")
		return err
	}

	return e2eutil.WaitForOperatorDeployment(
		t,
		test.Global.KubeClient,
		namespace,
		config.TestOperatorName,
		1,
		config.APIRetry,
		config.APITimeout,
	)
}
