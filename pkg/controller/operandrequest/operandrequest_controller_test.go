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
package operandrequest

import (
	"context"
	"testing"
	"time"

	v1beta2 "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	fakeolmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
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

// TestRequestController runs ReconcileOperandRequest.Reconcile() against a
// fake client that tracks a OperandRequest object.
func TestRequestController(t *testing.T) {
	var (
		name      = "common-service"
		namespace = "ibm-common-service"
	)
	assert := assert.New(t)
	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	// Retrieve OperandRequest instance
	requestInstance := &v1alpha1.OperandRequest{}
	err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.NoError(err)

	// Absent an operator from request
	requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
	err = r.client.Update(context.TODO(), requestInstance)
	assert.NoError(err)
	_, err = r.Reconcile(req)
	assert.NoError(err)

	// Check if operator absent from the request
	err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.NoError(err)
	assert.Equal(1, len(requestInstance.Status.Members), "operands member list should have only one element")

	// // TBD... Present an operator into request, due to fakeolmclient cannot mock subscription status
	// requestInstance.Spec.Requests[0].Operands = append(requestInstance.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
	// err = r.client.Update(context.TODO(), requestInstance)
	// _, err = r.Reconcile(req)
	// assert.NoError(err)

	// // Check if operator present into the request
	// err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	// assert.NoError(err)
	// assert.Equal(2, len(requestInstance.Spec.Requests[0].Operands), "operands list should have two elements")

	// requestInstance.Spec.Requests[0].Operands = append(requestInstance.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
	// err = r.client.Update(context.TODO(), requestInstance)
	// registryInstance, err := r.getRegistryInstance(name, namespace)
	// assert.NoError(err)
	// opt := r.getOperatorFromRegistryInstance("etcd", registryInstance)
	// err = r.createSubscription(requestInstance, opt)
	// assert.NoError(err)
	// err = r.updateMemberStatus(requestInstance)
	// assert.NoError(err)
	// err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	// assert.NoError(err)
	// assert.Equal(2, len(requestInstance.Status.Members), "operands member list should have two elements")

	// Mock delete OperandRequest instance, mark OperandRequest instance as delete state
	deleteTime := metav1.NewTime(time.Now())
	requestInstance.SetDeletionTimestamp(&deleteTime)
	err = r.client.Update(context.TODO(), requestInstance)
	assert.NoError(err)
	_, err = r.Reconcile(req)
	assert.NoError(err)
	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions(namespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Empty(subs.Items, "all subscriptions should be deleted")

	// Check if OperandRequest instance deleted
	err = r.client.Delete(context.TODO(), requestInstance)
	assert.NoError(err)
	err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.True(errors.IsNotFound(err), "retrieve operand request should be return an error of type is 'NotFound'")
}

func getReconcileRequest(name, namespace string) reconcile.Request {
	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

type DataObj struct {
	odlmObjs []runtime.Object
	olmObjs  []runtime.Object
}

func initClientData(name, namespace string) *DataObj {
	return &DataObj{
		odlmObjs: []runtime.Object{
			operandRegistry(name, namespace),
			operandRequest(name, namespace),
			operandConfig(name, namespace)},
		olmObjs: []runtime.Object{
			sub("etcd", namespace, "0.0.1"),
			sub("jenkins", namespace, "0.0.1"),
			csv("etcd-csv.v0.0.1", namespace, etcdExample),
			csv("jenkins-csv.v0.0.1", namespace, jenkinsExample),
			ip("etcd-install-plan", namespace),
			ip("jenkins-install-plan", namespace)},
	}
}

func getReconciler(name, namespace string) ReconcileOperandRequest {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	olmv1.SchemeBuilder.AddToScheme(s)
	olmv1alpha1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.odlmObjs...)

	// Create a fake OLM client to mock OLM API calls.
	olmClient := fakeolmclient.NewSimpleClientset(initData.olmObjs...)

	// Return a ReconcileOperandRequest object with the scheme and fake client.
	return ReconcileOperandRequest{
		scheme:    s,
		client:    client,
		olmClient: olmClient,
	}
}

// Return OperandRegistry obj
func operandRegistry(name, namespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
			OperatorsStatus: map[string]v1alpha1.OperatorStatus{
				"etcd": {
					Phase: v1alpha1.OperatorReady,
				},
				"jenkins": {
					Phase: v1alpha1.OperatorReady,
				},
			},
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
						"etcdCluster": {Raw: []byte(`{"size": 1}`)},
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
		Status: v1alpha1.OperandConfigStatus{
			ServiceStatus: map[string]v1alpha1.CrStatus{
				"etcd": {
					CrStatus: map[string]v1alpha1.ServicePhase{
						"etcdCluster": v1alpha1.ServiceReady,
					},
				},
				"jenkins": {
					CrStatus: map[string]v1alpha1.ServicePhase{
						"jenkins": v1alpha1.ServiceReady,
					},
				},
			},
		},
	}
}

// Return OperandRequest obj
func operandRequest(name, namespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

// Return Subscription obj
func sub(name, namespace, csvVersion string) *olmv1alpha1.Subscription {
	labels := map[string]string{
		"operator.ibm.com/opreq-control": "true",
	}
	return &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                "alpha",
			Package:                name,
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
		},
		Status: olmv1alpha1.SubscriptionStatus{
			CurrentCSV:   name + "-csv.v" + csvVersion,
			InstalledCSV: name + "-csv.v" + csvVersion,
			Install: &olmv1alpha1.InstallPlanReference{
				APIVersion: "operators.coreos.com/v1alpha1",
				Kind:       "InstallPlan",
				Name:       name + "-install-plan",
				UID:        types.UID("install-plan-uid"),
			},
			InstallPlanRef: &corev1.ObjectReference{
				APIVersion: "operators.coreos.com/v1alpha1",
				Kind:       "InstallPlan",
				Name:       name + "-install-plan",
				Namespace:  namespace,
				UID:        types.UID("install-plan-uid"),
			},
		},
	}
}

// Return CSV obj
func csv(name, namespace, example string) *olmv1alpha1.ClusterServiceVersion {
	return &olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"alm-examples": example,
			},
		},
		Spec: olmv1alpha1.ClusterServiceVersionSpec{},
	}
}

// Return InstallPlan obj
func ip(name, namespace string) *olmv1alpha1.InstallPlan {
	return &olmv1alpha1.InstallPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: olmv1alpha1.InstallPlanSpec{},
		Status: olmv1alpha1.InstallPlanStatus{
			Phase: olmv1alpha1.InstallPlanPhaseComplete,
		},
	}
}

const etcdExample string = `
[
	{
	  "apiVersion": "etcd.database.coreos.com/v1beta2",
	  "kind": "EtcdCluster",
	  "metadata": {
		"name": "example"
	  },
	  "spec": {
		"size": 3,
		"version": "3.2.13"
	  }
	}
]
`
const jenkinsExample string = `
[
	{
	  "apiVersion": "etcd.database.coreos.com/v1beta2",
	  "kind": "Jenkins",
	  "metadata": {
		"name": "example"
	  },
	  "spec": {
		"service": {"port": 8081}
	  }
	}
]
`
