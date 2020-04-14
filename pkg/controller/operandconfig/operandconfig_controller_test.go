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

// TestConfigController runs ReconcileOperandConfig.Reconcile() against a
// fake client that tracks a OperandConfig object.
func TestConfigController(t *testing.T) {
	var (
		name             = "common-service"
		namespace        = "ibm-common-service"
		requestName      = "ibm-cloudpak-name"
		requestNamespace = "ibm-cloudpak"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, requestName, requestNamespace)

	initReconcile(t, r, req, requestName, requestNamespace)

}

func initReconcile(t *testing.T, r ReconcileOperandConfig, req reconcile.Request, requestName, requestNamespace string) {
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	config := &v1alpha1.OperandConfig{}
	err = r.client.Get(context.TODO(), req.NamespacedName, config)
	assert.NoError(err)
	// Check the config init status
	assert.NotNil(config.Status, "init operator status should not be empty")
	assert.Equal(v1alpha1.ServiceInit, config.Status.Phase, "Overall OperandConfig phase should be 'Initialized'")

	request := &v1alpha1.OperandRequest{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: requestName, Namespace: requestNamespace}, request)
	assert.NoError(err)

	// Check the OperandRequest status
	assert.NotNil(request.Status, "init OperandRequest status should not be empty")
	assert.Equalf(v1alpha1.ClusterPhaseUpdating, request.Status.Phase, "Overall OperandRequest phase should be 'Updating'")
}

func getReconciler(name, namespace, requestName, requestNamespace string) ReconcileOperandConfig {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	corev1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace, requestName, requestNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.objs...)

	// Return a ReconcileOperandConfig object with the scheme and fake client.
	return ReconcileOperandConfig{
		scheme: s,
		client: client,
	}
}

type DataObj struct {
	objs []runtime.Object
}

func initClientData(name, namespace, requestName, requestNamespace string) *DataObj {
	return &DataObj{
		objs: []runtime.Object{
			operandRequest(name, namespace, requestName, requestNamespace),
			operandConfig(name, namespace)},
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

// Return OperandConfig obj
func operandConfig(name, namespace string) *v1alpha1.OperandConfig {
	return &v1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandConfigSpec{
			Services: []v1alpha1.ConfigService{
				{
					Name: "etcd",
					Spec: map[string]runtime.RawExtension{
						"etcdCluster": {Raw: []byte(`{"size": 3}`)},
					},
				},
				{
					Name: "jenkins",
					Spec: map[string]runtime.RawExtension{
						"jenkins": {Raw: []byte(`{"service":{"port": 8081}}`)},
					},
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
				namespace + "." + name + "/config": "true",
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
						},
					},
				},
			},
		},
	}
}
