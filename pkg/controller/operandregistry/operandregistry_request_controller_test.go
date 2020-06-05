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
func TestRegistryRequestController(t *testing.T) {
	var (
		name              = "common-service"
		namespace         = "ibm-common-service"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)
	r := getRequestReconciler(name, namespace, requestName, requestNamespace, operatorNamespace)

	initRegistryRequestReconcile(t, r, req, requestName, requestNamespace)
}

func initRegistryRequestReconcile(t *testing.T, r ReconcileOperandRegistryRequest, req reconcile.Request, requestName, requestNamespace string) {
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

	request := &v1alpha1.OperandRequest{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: requestName, Namespace: requestNamespace}, request)
	assert.NoError(err)

	// Check the OperandRequest status
	assert.NotNil(request.Status, "init OperandRequest status should not be empty")
	assert.Equalf(v1alpha1.ClusterPhaseUpdating, request.Status.Phase, "Overall OperandRequest phase should be 'Updating'")
}

func getRequestReconciler(name, namespace, requestName, requestNamespace, operatorNamespace string) ReconcileOperandRegistryRequest {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initRequestClientData(name, namespace, requestName, requestNamespace, operatorNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.objs...)

	// Return a ReconcileOperandRegistry object with the scheme and fake client.
	return ReconcileOperandRegistryRequest{
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

func initRequestClientData(name, namespace, requestName, requestNamespace, operatorNamespace string) *DataObj {
	return &DataObj{
		objs: []runtime.Object{
			operandRegistry(name, namespace, operatorNamespace),
			operandRequest(name, namespace, requestName, requestNamespace),
			// operandBindInfo(name, namespace, operatorName, operatorNamespace),
			// configmap("cm", operatorNamespace),
			// secret("secret", operatorNamespace),
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

// Return OperandRequest obj
func operandRequest(name, namespace, requestName, requestNamespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
			Labels: map[string]string{
				namespace + "." + name + "/registry": "true",
			},
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          name,
					RegistryNamespace: namespace,
					Operands: []v1alpha1.Operand{
						{
							Name: "etcd",
						},
						{
							Name: "jenkins",
							Bindings: map[string]v1alpha1.SecretConfigmap{
								"public": {
									Secret:    "secret3",
									Configmap: "cm3",
								},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.OperandRequestStatus{
			Phase: v1alpha1.ClusterPhaseRunning,
		},
	}
}

// Return OperandBindInfo obj
func operandBindInfo(name, namespace, operatorName, operatorNamespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName,
			Namespace: operatorNamespace,
			Labels: map[string]string{
				namespace + "." + name + "/registry": "true",
			},
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:  "jenkins",
			Registry: name,
			Bindings: map[string]v1alpha1.SecretConfigmap{
				"public": {
					Secret:    "secret1",
					Configmap: "cm1",
				},
				"private": {
					Secret:    "secret2",
					Configmap: "cm2",
				},
			},
		},
	}
}

func configmap(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"test": "configmap",
		},
	}
}

func secret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"test": "secret",
		},
	}
}
