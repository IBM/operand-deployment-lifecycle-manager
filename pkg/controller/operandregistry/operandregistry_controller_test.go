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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

// TestRegistryController runs ReconcileOperandRegistry.Reconcile() against a
// fake client that tracks a OperandRegistry object.
func TestRegistryController(t *testing.T) {
	var (
		name              = "common-service"
		namespace         = "ibm-common-service"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, operatorNamespace)

	initReconcile(t, r, req)

}

func initReconcile(t *testing.T, r ReconcileOperandRegistry, req reconcile.Request) {
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	registry := &v1alpha1.OperandRegistry{}
	err = r.client.Get(context.TODO(), req.NamespacedName, registry)
	assert.NoError(err)
	// Check the default value
	for _, o := range registry.Spec.Operators {
		assert.Equalf(v1alpha1.ScopePrivate, o.Scope, "default operator(%s) scope should be private", o.Name)
	}
	// Check the registry init status
	assert.NotNil(registry.Status, "init operator status should not be empty")
	assert.Equalf(v1alpha1.OperatorInit, registry.Status.Phase, "Overall OperandRegistry phase should be 'Initialized'")
}

func getReconciler(name, namespace, operatorNamespace string) ReconcileOperandRegistry {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace, operatorNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.objs...)

	// Return a ReconcileOperandRegistry object with the scheme and fake client.
	return ReconcileOperandRegistry{
		scheme: s,
		client: client,
	}
}

// Mock request to simulate Reconcile() being called on an event for a watched resource
func getReconcileRequest(name, namespace string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

type DataObj struct {
	objs []runtime.Object
}

func initClientData(name, namespace, operatorNamespace string) *DataObj {
	return &DataObj{
		objs: []runtime.Object{
			operandRegistry(name, namespace, operatorNamespace),
		},
	}
}

// Return OperandRegistry obj
func operandRegistry(name, namespace, operatorNamespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "etcd",
					Namespace:       operatorNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
				},
				{
					Name:            "jenkins",
					Namespace:       operatorNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
				},
			},
		},
	}
}
