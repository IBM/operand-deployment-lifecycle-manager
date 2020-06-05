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
package operandregistry

import (
	"context"
	"testing"

	v1beta2 "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	v1alpha2 "github.com/jenkinsci/kubernetes-operator/pkg/apis/jenkins/v1alpha2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

// TestRegistryController runs ReconcileOperandRegistry.Reconcile() against a
// fake client that tracks a OperandRegistry object.
func TestRegistryBindInfoController(t *testing.T) {
	var (
		name              = "common-service"
		namespace         = "ibm-common-service"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		operatorName      = "ibm-operator-name"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)

	r := getBindInfoReconciler(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace)

	initBindInfoReconcile(t, r, req, operatorName, operatorNamespace)
}

func initBindInfoReconcile(t *testing.T, r ReconcileOperandRegistryBindinfo, req reconcile.Request, operatorName, operatorNamespace string) {
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	registry := &v1alpha1.OperandRegistry{}
	err = r.client.Get(context.TODO(), req.NamespacedName, registry)
	assert.NoError(err)

	bindInfo := &v1alpha1.OperandBindInfo{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: operatorName, Namespace: operatorNamespace}, bindInfo)
	assert.NoError(err)

	// Check the OperandBindInfo status
	assert.NotNil(bindInfo.Status, "init OperandBindInfo status should not be empty")
	assert.Equalf(v1alpha1.BindInfoUpdating, bindInfo.Status.Phase, "Overall OperandBindInfo phase should be 'Updating'")
}

func getBindInfoReconciler(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace string) ReconcileOperandRegistryBindinfo {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initBindInfoClientData(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.objs...)

	// Return a ReconcileOperandRegistry object with the scheme and fake client.
	return ReconcileOperandRegistryBindinfo{
		scheme: s,
		client: client,
	}
}

func initBindInfoClientData(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace string) *DataObj {
	return &DataObj{
		objs: []runtime.Object{
			operandRegistry(name, namespace, operatorNamespace),
			operandRequest(name, namespace, requestName, requestNamespace),
			operandBindInfo(name, namespace, operatorName, operatorNamespace),
			configmap("cm", operatorNamespace),
			secret("secret", operatorNamespace),
		},
	}
}
