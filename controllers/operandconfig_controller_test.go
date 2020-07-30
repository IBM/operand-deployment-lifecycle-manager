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
package controllers

// import (
// 	"context"
// 	"testing"

// 	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
// 	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
// 	"github.com/stretchr/testify/assert"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/runtime"
// 	"k8s.io/apimachinery/pkg/types"
// 	"k8s.io/client-go/kubernetes/scheme"
// 	"sigs.k8s.io/controller-runtime/pkg/client/fake"
// 	"sigs.k8s.io/controller-runtime/pkg/reconcile"

// 	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
// 	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
// )

// // TestConfigController runs OperandConfigReconciler.Reconcile() against a
// // fake client that tracks a OperandConfig object.
// func TestConfigController(t *testing.T) {
// 	var (
// 		name              = "ibm-cloudpak-name"
// 		namespace         = "ibm-cloudpak"
// 		registryName      = "ibm-cloudpak-name"
// 		registryNamespace = "ibm-cloudpak"
// 		operatorNamespace = "ibm-operators"
// 	)

// 	req := getReconcileRequest(name, namespace)
// 	r := getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace)

// 	initReconcile(t, r, req)

// }

// func initReconcile(t *testing.T, r OperandConfigReconciler, req reconcile.Request) {
// 	assert := assert.New(t)

// 	_, err := r.Reconcile(req)
// 	assert.NoError(err)

// 	config := &v1alpha1.OperandConfig{}
// 	err = r.Get(context.TODO(), req.NamespacedName, config)
// 	assert.NoError(err)
// 	// Check the config init status
// 	assert.NotNil(config.Status, "init operator status should not be empty")
// 	assert.Equal(v1alpha1.ServiceNotReady, config.Status.Phase, "Overall OperandConfig phase should be 'Not Ready'")
// 	// TODO: Add a test case with CR deployed
// }

// func getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace string) OperandConfigReconciler {
// 	s := scheme.Scheme
// 	v1alpha1.SchemeBuilder.AddToScheme(s)

// 	initData := initClientData(name, namespace, registryName, registryNamespace, operatorNamespace)

// 	// Create a fake client to mock API calls.
// 	client := fake.NewFakeClient(initData.odlmObjs...)

// 	// Return a OperandConfigReconciler object with the scheme and fake client.
// 	return OperandConfigReconciler{
// 		Scheme: s,
// 		Client: client,
// 	}
// }

// type DataObj struct {
// 	odlmObjs []runtime.Object
// }

// func initClientData(name, namespace, registryName, registryNamespace, operatorNamespace string) *DataObj {
// 	return &DataObj{
// 		odlmObjs: []runtime.Object{
// 			operandRegistry(registryName, registryNamespace, operatorNamespace),
// 			operandConfig(name, namespace)},
// 	}
// }

// // Mock request to simulate Reconcile() being called on an event for a watched resource
// func getReconcileRequest(name, namespace string) reconcile.Request {
// 	return reconcile.Request{
// 		NamespacedName: types.NamespacedName{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 	}
// }

// // Return OperandConfig obj
// func operandConfig(name, namespace string) *v1alpha1.OperandConfig {
// 	return &v1alpha1.OperandConfig{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Spec: v1alpha1.OperandConfigSpec{
// 			Services: []v1alpha1.ConfigService{
// 				{
// 					Name: "etcd",
// 					Spec: map[string]runtime.RawExtension{
// 						"etcdCluster": {Raw: []byte(`{"size": 3}`)},
// 					},
// 				},
// 				{
// 					Name: "jenkins",
// 					Spec: map[string]runtime.RawExtension{
// 						"jenkins": {Raw: []byte(`{"service":{"port": 8081}}`)},
// 					},
// 				},
// 			},
// 		},
// 	}
// }

// // Return OperandRegistry obj
// func operandRegistry(name, namespace, operatorNamespace string) *v1alpha1.OperandRegistry {
// 	return &v1alpha1.OperandRegistry{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Spec: v1alpha1.OperandRegistrySpec{
// 			Operators: []v1alpha1.Operator{
// 				{
// 					Name:            "etcd",
// 					Namespace:       operatorNamespace,
// 					SourceName:      "community-operators",
// 					SourceNamespace: "openshift-marketplace",
// 					PackageName:     "etcd",
// 					Channel:         "singlenamespace-alpha",
// 				},
// 				{
// 					Name:            "jenkins",
// 					Namespace:       operatorNamespace,
// 					SourceName:      "community-operators",
// 					SourceNamespace: "openshift-marketplace",
// 					PackageName:     "jenkins-operator",
// 					Channel:         "alpha",
// 				},
// 			},
// 		},
// 	}
// }
