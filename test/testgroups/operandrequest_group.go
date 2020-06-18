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

	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/helpers"

	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

// TestOperandRequest is the test group for testing Operand Request
func TestOperandRequest(t *testing.T) {
	t.Run("TestOperandRequestCRUD", TestOperandRequestCRUD)
}

// TestOperandRequestCRUD is for testing the OperandRequest
func TestOperandRequestCRUD(t *testing.T) {
	assert := assert.New(t)
	ctx := test.NewTestCtx(t)
	defer ctx.Cleanup()

	// get global framework variables
	f := test.Global

	// Create 2 namespace for multi Requests test
	err := helpers.CreateNamespace(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	err = helpers.CreateNamespace(f, ctx, config.TestNamespace2)
	assert.NoError(err)

	// Create dependent Registry and Config for Request
	reg, err := helpers.CreateOperandRegistry(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(reg, "regisgry %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace1)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryInit, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.RegistryInit, reg.Status.Phase, "registry(%s/%s) phase should be Initialized", reg.Namespace, reg.Name)

	con, err := helpers.CreateOperandConfig(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(con, "config %s should be created in namespace %s", config.OperandConfigCrName, config.TestNamespace1)

	con, err = helpers.WaitConfigStatus(f, operator.ServiceInit, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.ServiceInit, con.Status.Phase, "config(%s/%s) phase should be Initialized", con.Namespace, con.Name)

	// Create the first Request instance
	req1 := helpers.NewOperandRequestCR1(config.OperandRequestCrName, config.TestNamespace1)
	req1, err = helpers.CreateOperandRequest(f, ctx, req1)
	assert.NoError(err)
	assert.NotNilf(req1, "reqest %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace1)

	req1, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req1.Status.Phase, "request(%s/%s) phase should be Running", req1.Namespace, req1.Name)
	// Manual create BindInfo to mock alm-example
	bi, err := helpers.CreateOperandBindInfo(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(bi, "bindinfo %s should be created in namespace %s", config.OperandBindInfoCrName, config.TestNamespace1)

	bi, err = helpers.WaitBindInfoStatus(f, operator.BindInfoInit, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.BindInfoInit, bi.Status.Phase, "bindinfo(%s/%s) phase should be Initialized", bi.Namespace, bi.Name)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	// Create the second Request instance
	req2 := helpers.NewOperandRequestCR2(config.OperandRequestCrName, config.TestNamespace2)
	req2, err = helpers.CreateOperandRequest(f, ctx, req2)
	assert.NoError(err)
	assert.NotNilf(req2, "request %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace2)

	req2, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace2)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req2.Status.Phase, "request(%s/%s) phase should be Running", req2.Namespace, req2.Name)

	// Check registry status if updated
	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 2, "operator jenkins-operator should have 2 requests")

	bi, err = helpers.WaitBindInfoStatus(f, operator.BindInfoCompleted, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.BindInfoCompleted, bi.Status.Phase, "bindinfo(%s/%s) phase should be Completed", bi.Namespace, bi.Name)

	// Check secret and configmap if copied
	sec, err := helpers.RetrieveSecret(f, "jenkins-operator-credentials-example", config.TestNamespace2)
	assert.NoError(err)
	assert.NotNilf(sec, "secret %s should be copied to namespace %s", "jenkins-operator-credentials-example", config.TestNamespace2)

	cm, err := helpers.RetrieveConfigmap(f, "jenkins-operator-init-configuration-example", config.TestNamespace2)
	assert.NoError(err)
	assert.NotNilf(cm, "configmap %s should be copied to namespace %s", "jenkins-operator-init-configuration-example", config.TestNamespace2)

	// Delete the last operator and related operands from Request 1
	req1, err = helpers.AbsentOperandFromRequest(f, config.TestNamespace1, "jenkins")
	assert.NoError(err)
	assert.Len(req1.Spec.Requests[0].Operands, 1, "the operands number should be equal 1")

	req1, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req1.Status.Phase, "request(%s/%s) phase should be Running", req1.Namespace, req1.Name)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	// Add a operator into Request 1
	req1, err = helpers.PresentOperandFromRequest(f, config.TestNamespace1, "jenkins")
	assert.NoError(err)
	assert.Len(req1.Spec.Requests[0].Operands, 2, "the operands number should be equal 2")

	req1, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req1.Status.Phase, "request(%s/%s) phase should be Running", req1.Namespace, req1.Name)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 2, "the reconcile request number should be equal 2 for operator jenkins")

	// Delete request 2
	err = helpers.DeleteOperandRequest(req2, f)
	assert.NoError(err)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	err = helpers.DeleteOperandRequest(req1, f)
	assert.NoError(err)
}
