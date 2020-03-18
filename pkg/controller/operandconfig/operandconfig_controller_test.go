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
package operandconfig

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

// TestConfigController runs ReconcileOperandConfig.Reconcile() against a
// fake client that tracks a OperandConfig object.
func TestConfigController(t *testing.T) {
	assert := assert.New(t)
	var (
		name      = "cs-config"
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

	// A operandconfig resource with metadata and spec.
	config := &v1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandConfigSpec{
			Services: []v1alpha1.ConfigService{
				{
					Name: "jenkins",
					Spec: map[string]runtime.RawExtension{
						"jenkins": {Raw: []byte(`{"service":{"port": 8081}}`)},
					},
				},
				{
					Name: "etcd",
					Spec: map[string]runtime.RawExtension{
						"etcdCluster": {Raw: []byte(`{"size": 1}`)},
					},
				},
			},
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		config,
	}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(v1alpha1.SchemeGroupVersion, config)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileOperandConfig object with the scheme and fake client.
	r := &ReconcileOperandConfig{client: cl, scheme: s}

	_, err := r.Reconcile(req)
	assert.NoError(err)

	err = r.client.Get(context.TODO(), req.NamespacedName, config)
	assert.NoError(err)
	// Check the config init status
	assert.NotNil(config.Status, "init operator status should not be empty")
	assert.Equal(v1alpha1.ServiceReady, config.Status.Phase, "Overall OperandConfig phase should be 'Ready for Deployment'")

	// Create a fake client to mock instance not found.
	cl = fake.NewFakeClient()
	// Create a ReconcileOperandConfig object with the scheme and fake client.
	r = &ReconcileOperandConfig{client: cl, scheme: s}
	_, err = r.Reconcile(req)
	assert.NoError(err)
}
