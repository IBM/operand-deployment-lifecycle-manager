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

package operandbindinfo

import (
	"context"
	"testing"

	v1beta2 "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	v1alpha2 "github.com/jenkinsci/kubernetes-operator/pkg/apis/jenkins/v1alpha2"
	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

// TestBindInfoController runs ReconcileOperandBindInfo.Reconcile() against a
// fake client that tracks a OperandBindInfo object.
func TestBindInfoController(t *testing.T) {
	var (
		name              = "ibm-operators-bindinfo"
		namespace         = "ibm-operators"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-services"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, registryName, registryNamespace, requestName, requestNamespace)

	initReconcile(t, r, req, requestNamespace)

}

// Init reconcile the OperandBindInfo
func initReconcile(t *testing.T, r ReconcileOperandBindInfo, req reconcile.Request, requestNamespace string) {
	assert := assert.New(t)
	res, err := r.Reconcile(req)
	if res.Requeue {
		t.Error("Reconcile requeued request as not expected")
	}
	assert.NoError(err)

	// Retrieve configmap and secret
	configmap1 := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "cm3", Namespace: requestNamespace}, configmap1)
	assert.NoError(err)

	secret1 := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "secret3", Namespace: requestNamespace}, secret1)
	assert.NoError(err)

	configmap2 := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "cm2", Namespace: requestNamespace}, configmap2)
	assert.True(errors.IsNotFound(err))

	secret2 := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "secret2", Namespace: requestNamespace}, secret2)
	assert.True(errors.IsNotFound(err))
}

func getReconciler(name, namespace, registryName, registryNamespace, requestName, requestNamespace string) ReconcileOperandBindInfo {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)
	olmv1.SchemeBuilder.AddToScheme(s)
	olmv1alpha1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace, registryName, registryNamespace, requestName, requestNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.objs...)

	// Return a ReconcileOperandBindInfo object with the scheme and fake client.
	return ReconcileOperandBindInfo{
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

func initClientData(name, namespace, registryName, registryNamespace, requestName, requestNamespace string) *DataObj {
	return &DataObj{
		objs: []runtime.Object{
			operandRegistry(namespace, registryName, registryNamespace, requestName, requestNamespace),
			operandRequest(registryName, registryNamespace, requestName, requestNamespace),
			operandBindInfo(name, namespace, registryName, registryNamespace),
			configmap1("cm1", namespace),
			configmap2("cm2", namespace),
			secret1("secret1", namespace),
			secret2("secret2", namespace),
		},
	}
}

// Return OperandRegistry obj
func operandRegistry(namespace, registryName, registryNamespace, requestName, requestNamespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryName,
			Namespace: registryNamespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "etcd",
					Namespace:       namespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
				},
				{
					Name:            "jenkins",
					Namespace:       namespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
				},
			},
		},
		Status: v1alpha1.OperandRegistryStatus{
			Phase: v1alpha1.RegistryRunning,
			OperatorsStatus: map[string]v1alpha1.OperatorStatus{
				"jenkins": {
					Phase: v1alpha1.OperatorRunning,
					ReconcileRequests: []v1alpha1.ReconcileRequest{
						{
							Name:      requestName,
							Namespace: requestNamespace,
						},
					},
				},
			},
		},
	}
}

// Return OperandRequest obj
func operandRequest(registryName, registryNamespace, requestName, requestNamespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          registryName,
					RegistryNamespace: registryNamespace,
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
	}
}

// Return OperandBindInfo obj
func operandBindInfo(name, namespace, registryName, registryNamespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "jenkins",
			Registry:          registryName,
			RegistryNamespace: registryNamespace,
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

func configmap1(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"test": "configmap1",
		},
	}
}

func secret1(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"test": "secret1",
		},
	}
}

func configmap2(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"test": "configmap1",
		},
	}
}

func secret2(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"test": "secret2",
		},
	}
}
