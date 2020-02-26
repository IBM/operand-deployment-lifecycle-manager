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
	"testing"

	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TestCatalogController runs ReconcileOperandRegistry.Reconcile() against a
// fake client that tracks a OperandRegistry object.
func TestCatalogController(t *testing.T) {

	var (
		name      = "operand-deployment-lifecycle-manager-catalog"
		namespace = "common-service"
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
	catalog := &v1alpha1.OperandRegistry{
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
					TargetNamespaces: []string{
						"etcd-operator",
					},
				},
				{
					Name:            "jenkins",
					Namespace:       "jenkins-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
					TargetNamespaces: []string{
						"jenkins-operator",
					},
				},
			},
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		catalog,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, catalog)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileOperandRegistry object with the scheme and fake client.
	r := &ReconcileOperandRegistry{client: cl, scheme: s}

	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile requeue which is not expected")
	}

	// Create a fake client to mock instance not found.
	cl = fake.NewFakeClient()
	// Create a ReconcileOperandRegistry object with the scheme and fake client.
	r = &ReconcileOperandRegistry{client: cl, scheme: s}

	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile requeue which is not expected")
	}
}
