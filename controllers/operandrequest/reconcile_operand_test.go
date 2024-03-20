//
// Copyright 2022 IBM Corporation
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

package operandrequest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

type MockReader struct {
	mock.Mock
}

func (m *MockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	args := m.Called(ctx, key, obj)
	return args.Error(0)
}
func (m *MockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func TestSetOwnerReferences(t *testing.T) {
	// Create a fake client
	client := fake.NewClientBuilder().Build()

	// Create a mock reader
	reader := &MockReader{}

	// Create a reconciler instance
	r := &Reconciler{
		ODLMOperator: &deploy.ODLMOperator{
			Client: client,
			Reader: reader,
		},
	}

	// Create the controlled resource
	controlledRes := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "test-namespace",
			},
		},
	}

	// Create the owner references
	ownerReferences := []operatorv1alpha1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Deployment",
			Name:       "test-deployment",
			Controller: pointer.BoolPtr(true),
		},
		{
			APIVersion: "v1",
			Kind:       "Service",
			Name:       "test-service",
			Controller: pointer.BoolPtr(false),
		},
	}

	// Mock the Get method of the reader
	reader.On("Get", mock.Anything, types.NamespacedName{Name: "test-deployment", Namespace: "test-namespace"}, mock.AnythingOfType("*unstructured.Unstructured")).Return(nil)
	reader.On("Get", mock.Anything, types.NamespacedName{Name: "test-service", Namespace: "test-namespace"}, mock.AnythingOfType("*unstructured.Unstructured")).Return(nil)

	// Call the setOwnerReferences function
	err := r.setOwnerReferences(context.Background(), controlledRes, &ownerReferences)

	// Assert that there are no errors
	assert.NoError(t, err)

	// Assert that the owner references are set correctly
	expectedOwnerReferences := []metav1.OwnerReference{
		{
			APIVersion:         "v1",
			Kind:               "Deployment",
			Name:               "test-deployment",
			Controller:         pointer.BoolPtr(true),
			BlockOwnerDeletion: pointer.BoolPtr(true),
		},
		{
			APIVersion: "v1",
			Kind:       "Service",
			Name:       "test-service",
		},
	}
	assert.Equal(t, expectedOwnerReferences, controlledRes.GetOwnerReferences())
}
