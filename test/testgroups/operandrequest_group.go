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
	assert.NotNilf(reg, "registry %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace1)

	_, err = helpers.WaitRegistryStatus(f, operator.RegistryReady, config.TestNamespace1)
	assert.NoError(err)

	con, err := helpers.CreateOperandConfig(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(con, "config %s should be created in namespace %s", config.OperandConfigCrName, config.TestNamespace1)

	_, err = helpers.WaitConfigStatus(f, operator.ServiceInit, config.TestNamespace1)
	assert.NoError(err)

	// Create the first Request instance
	req1 := helpers.NewOperandRequestCR1(config.OperandRequestCrName, config.TestNamespace1)
	req1, err = helpers.CreateOperandRequest(f, ctx, req1)
	assert.NoError(err)
	assert.NotNilf(req1, "request %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace1)

	_, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)

	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "etcd", "openshift-operators", false)
	assert.NoError(err)

	// Manual create BindInfo to mock alm-example
	bi, err := helpers.CreateOperandBindInfo(f, ctx, config.TestNamespace1)
	assert.NoError(err)
	assert.NotNilf(bi, "bindinfo %s should be created in namespace %s", config.OperandBindInfoCrName, config.TestNamespace1)

	bindinfo, err := helpers.WaitBindInfoStatus(f, operator.BindInfoCompleted, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(bindinfo.Status.RequestNamespaces, 0, "bindinfo for jenkins should have 0 requests")

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	con, err = helpers.WaitConfigStatus(f, operator.ServiceRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equal(con.Status.ServiceStatus["etcd"].CrStatus["EtcdCluster"], operator.ServiceRunning, "The status of EtcdCluster should be running")
	assert.Equal(con.Status.ServiceStatus["jenkins"].CrStatus["Jenkins"], operator.ServiceRunning, "The status of Jenkins should be running")

	// Create the second Request instance
	req2 := helpers.NewOperandRequestCR2(config.OperandRequestCrName, config.TestNamespace2)
	req2, err = helpers.CreateOperandRequest(f, ctx, req2)
	assert.NoError(err)
	assert.NotNilf(req2, "request %s should be created in namespace %s", config.OperandRegistryCrName, config.TestNamespace2)

	_, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace2)
	assert.NoError(err)

	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "jaeger", config.TestNamespace2, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "etcd", "openshift-operators", false)
	assert.NoError(err)

	// Check registry status if updated
	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 2, "operator jenkins-operator should have 2 requests")

	con, err = helpers.WaitConfigStatus(f, operator.ServiceRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equal(con.Status.ServiceStatus["etcd"].CrStatus["EtcdCluster"], operator.ServiceRunning, "The status of EtcdCluster should be running")
	assert.Equal(con.Status.ServiceStatus["jenkins"].CrStatus["Jenkins"], operator.ServiceRunning, "The status of Jenkins should be running")

	// Check secret and configmap if copied
	sec, err := helpers.RetrieveSecret(f, "jenkins-operator-credentials-example", config.TestNamespace2, false)
	assert.NoError(err)
	assert.NotNilf(sec, "secret %s should be copied to namespace %s", "jenkins-operator-credentials-example", config.TestNamespace2)

	cm, err := helpers.RetrieveConfigmap(f, "jenkins-operator-init-configuration-example", config.TestNamespace2, false)
	assert.NoError(err)
	assert.NotNilf(cm, "configmap %s should be copied to namespace %s", "jenkins-operator-init-configuration-example", config.TestNamespace2)

	bindinfo, err = helpers.WaitBindInfoStatus(f, operator.BindInfoCompleted, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(bindinfo.Status.RequestNamespaces, 1, "bindinfo for jenkins should have 1 requests")

	// Delete the last operator and related operands from Request 1
	req1, err = helpers.AbsentOperandFromRequest(f, config.TestNamespace1, "jenkins")
	assert.NoError(err)
	assert.Len(req1.Spec.Requests[0].Operands, 1, "the operands number should be equal 1")

	req1, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req1.Status.Phase, "request(%s/%s) phase should be Running", req1.Namespace, req1.Name)

	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "jaeger", config.TestNamespace2, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "etcd", "openshift-operators", false)
	assert.NoError(err)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	bindinfo, err = helpers.WaitBindInfoStatus(f, operator.BindInfoCompleted, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(bindinfo.Status.RequestNamespaces, 1, "bindinfo for jenkins should have 1 requests")

	// Add a operator into Request 1
	req1, err = helpers.PresentOperandFromRequest(f, config.TestNamespace1, "jenkins")
	assert.NoError(err)
	assert.Len(req1.Spec.Requests[0].Operands, 2, "the operands number should be equal 2")

	req1, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace1)
	assert.NoError(err)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 2, "the reconcile request number should be equal 2 for operator jenkins")

	// Delete the request 1
	err = helpers.DeleteOperandRequest(req1, f)
	assert.NoError(err)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	bindinfo, err = helpers.WaitBindInfoStatus(f, operator.BindInfoCompleted, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(bindinfo.Status.RequestNamespaces, 1, "bindinfo for jenkins should have 1 requests")

	// Check if etcd subscription are deleted
	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "jaeger", config.TestNamespace2, false)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "etcd", "openshift-operators", true)
	assert.NoError(err)

	// Update OperandConfig
	err = helpers.UpdateOperandConfig(f, config.TestNamespace1)
	assert.NoError(err)

	_, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace2)
	assert.NoError(err)

	con, err = helpers.WaitConfigStatus(f, operator.ServiceRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Equal(con.Status.ServiceStatus["jenkins"].CrStatus["Jenkins"], operator.ServiceRunning, "The status of Jenkins should be running")

	// Delete the jenkin operator and its operands from Request 2
	req2, err = helpers.AbsentOperandFromRequest(f, config.TestNamespace2, "jenkins")
	assert.NoError(err)
	assert.Len(req2.Spec.Requests[0].Operands, 1, "the operands number should be equal 1")

	req2, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace2)
	assert.NoError(err)
	assert.Equalf(operator.ClusterPhaseRunning, req2.Status.Phase, "request(%s/%s) phase should be Running", req2.Namespace, req2.Name)

	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, true)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "jaeger", config.TestNamespace2, false)
	assert.NoError(err)

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 0, "the reconcile request number should be equal 0 for operator jenkins")

	// TODO: When secret and configmap are deleted, the copies should be copied as well
	// // Check if secret and configmap are deleted
	// _, err = helpers.RetrieveSecret(f, "jenkins-operator-credentials-example", config.TestNamespace2, true)
	// assert.NoError(err)

	// _, err = helpers.RetrieveConfigmap(f, "jenkins-operator-init-configuration-example", config.TestNamespace2, true)
	// assert.NoError(err)

	// Add jenkins operator into Request 1
	_, err = helpers.PresentOperandFromRequest(f, config.TestNamespace2, "jenkins")
	assert.NoError(err)

	req2, err = helpers.WaitRequestStatus(f, operator.ClusterPhaseRunning, config.TestNamespace2)
	assert.NoError(err)
	assert.Len(req2.Status.Members, 2, "the operands number should be equal 2")

	reg, err = helpers.WaitRegistryStatus(f, operator.RegistryRunning, config.TestNamespace1)
	assert.NoError(err)
	assert.Len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests, 1, "the reconcile request number should be equal 1 for operator jenkins")

	// Check if secret and configmap are copied
	_, err = helpers.RetrieveSecret(f, "jenkins-operator-credentials-example", config.TestNamespace2, false)
	assert.NoError(err)

	_, err = helpers.RetrieveConfigmap(f, "jenkins-operator-init-configuration-example", config.TestNamespace2, false)
	assert.NoError(err)

	err = helpers.DeleteOperandRequest(req2, f)
	assert.NoError(err)

	// Check if subscriptions are deleted
	_, err = helpers.RetrieveSubscription(f, "jenkins", config.TestNamespace1, true)
	assert.NoError(err)
	_, err = helpers.RetrieveSubscription(f, "jaeger", config.TestNamespace2, true)
	assert.NoError(err)

	// Check if secret and configmap are deleted
	_, err = helpers.RetrieveSecret(f, "jenkins-operator-credentials-example", config.TestNamespace2, true)
	assert.NoError(err)

	_, err = helpers.RetrieveConfigmap(f, "jenkins-operator-init-configuration-example", config.TestNamespace2, true)
	assert.NoError(err)
}
