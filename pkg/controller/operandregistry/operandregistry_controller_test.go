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

	v1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1"
)

// TestRegistryController runs ReconcileOperandRegistry.Reconcile() against a
// fake client that tracks a OperandRegistry object.
func TestRegistryController(t *testing.T) {
	var (
		name              = "common-service"
		namespace         = "ibm-common-service"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		operatorName      = "ibm-operator-name"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace)

	initReconcile(t, r, req, requestName, requestNamespace, operatorName, operatorNamespace)

}

func initReconcile(t *testing.T, r ReconcileOperandRegistry, req reconcile.Request, requestName, requestNamespace, operatorName, operatorNamespace string) {
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	registry := &v1.OperandRegistry{}
	err = r.client.Get(context.TODO(), req.NamespacedName, registry)
	assert.NoError(err)
	// Check the default value
	for _, o := range registry.Spec.Operators {
		assert.Equalf(v1.ScopePrivate, o.Scope, "default operator(%s) scope should be private", o.Name)
	}
	// Check the registry init status
	assert.NotNil(registry.Status, "init operator status should not be empty")
	assert.Equalf(v1.OperatorInit, registry.Status.Phase, "Overall OperandRegistry phase should be 'Initialized'")

	bindInfo := &v1.OperandBindInfo{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: operatorName, Namespace: operatorNamespace}, bindInfo)
	assert.NoError(err)

	// Check the OperandBindInfo status
	assert.NotNil(bindInfo.Status, "init OperandBindInfo status should not be empty")
	assert.Equalf(v1.BindInfoUpdating, bindInfo.Status.Phase, "Overall OperandBindInfo phase should be 'Updating'")

	request := &v1.OperandRequest{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: requestName, Namespace: requestNamespace}, request)
	assert.NoError(err)

	// Check the OperandRequest status
	assert.NotNil(request.Status, "init OperandRequest status should not be empty")
	assert.Equalf(v1.ClusterPhaseUpdating, request.Status.Phase, "Overall OperandRequest phase should be 'Updating'")
}

func getReconciler(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace string) ReconcileOperandRegistry {
	s := scheme.Scheme
	v1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace)

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

func initClientData(name, namespace, requestName, requestNamespace, operatorName, operatorNamespace string) *DataObj {
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

// Return OperandRegistry obj
func operandRegistry(name, namespace, operatorNamespace string) *v1.OperandRegistry {
	return &v1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.OperandRegistrySpec{
			Operators: []v1.Operator{
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
func operandRequest(name, namespace, requestName, requestNamespace string) *v1.OperandRequest {
	return &v1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
			Labels: map[string]string{
				namespace + "." + name + "/registry": "true",
			},
		},
		Spec: v1.OperandRequestSpec{
			Requests: []v1.Request{
				{
					Registry:          name,
					RegistryNamespace: namespace,
					Operands: []v1.Operand{
						{
							Name: "etcd",
						},
						{
							Name: "jenkins",
							Bindings: map[string]v1.SecretConfigmap{
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
	}
}

// Return OperandBindInfo obj
func operandBindInfo(name, namespace, operatorName, operatorNamespace string) *v1.OperandBindInfo {
	return &v1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName,
			Namespace: operatorNamespace,
			Labels: map[string]string{
				namespace + "." + name + "/registry": "true",
			},
		},
		Spec: v1.OperandBindInfoSpec{
			Operand:  "jenkins",
			Registry: name,
			Bindings: map[string]v1.SecretConfigmap{
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
