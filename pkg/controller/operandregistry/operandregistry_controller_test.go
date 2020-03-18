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

	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestRegistryController runs ReconcileOperandRegistry.Reconcile() against a
// fake client that tracks a OperandRegistry object.
func TestRegistryController(t *testing.T) {
	assert := assert.New(t)
	var (
		name      = "cs-registry"
		namespace = "ibm-common-service"
	)
	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	// A operandregistry resource with metadata and spec.
	registry := &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "etcd",
					Namespace:       "etcd-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
				},
				{
					Name:            "jenkins",
					Namespace:       "jenkins-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
				},
			},
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		registry,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, registry)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileOperandRegistry object with the scheme and fake client.
	r := &ReconcileOperandRegistry{client: cl, scheme: s}

	_, err := r.Reconcile(req)
	assert.NoError(err)

	err = r.client.Get(context.TODO(), req.NamespacedName, registry)
	assert.NoError(err)
	// Check the default value
	for _, o := range registry.Spec.Operators {
		assert.Equalf(v1alpha1.ScopePrivate, o.Scope, "default operator(%s) scope should be private", o.Name)
	}
	// Check the registry init status
	assert.NotNil(registry.Status, "init operator status should not be empty")
	assert.Equalf(v1alpha1.OperatorReady, registry.Status.Phase, "Overall OperandRegistry phase should be 'Ready for Deployment'")

	// Create a fake client to mock instance not found.
	cl = fake.NewFakeClient()
	// Create a ReconcileOperandRegistry object with the scheme and fake client.
	r = &ReconcileOperandRegistry{client: cl, scheme: s}

	_, err = r.Reconcile(req)
	assert.NoError(err)

}
