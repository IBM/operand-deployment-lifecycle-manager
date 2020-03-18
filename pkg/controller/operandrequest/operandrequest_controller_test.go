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

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace)
	requestInstance := &v1alpha1.OperandRequest{}

	initReconcile(t, r, req, requestInstance)

	absentOperand(t, r, req, requestInstance)

	presentOperand(t, r, req, requestInstance)

	deleteOperandRequest(t, r, req, requestInstance)
}

// Init reconcile the OperandRequest
func initReconcile(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest) {
	assert := assert.New(t)
	_, err := r.Reconcile(req)
	assert.NoError(err)

	// Retrieve OperandRegistry
	registryInstance, err := r.getRegistryInstance(req.Name, req.Namespace)
	assert.NoError(err)
	for k, v := range registryInstance.Status.OperatorsStatus {
		assert.Equalf(v1alpha1.OperatorRunning, v.Phase, "operator(%s) phase should be Running", k)
		assert.NotEqualf(-1, registryInstance.GetReconcileRequest(k, req), "reconcile requests should be include %s", req.NamespacedName)
	}
	assert.Equalf(v1alpha1.OperatorRunning, registryInstance.Status.Phase, "registry(%s/%s) phase should be Running", registryInstance.Namespace, registryInstance.Name)

	// Retrieve OperandConfig
	configInstance, err := r.getConfigInstance(req.Name, req.Namespace)
	assert.NoError(err)
	for k, v := range configInstance.Status.ServiceStatus {
		for crK, crV := range v.CrStatus {
			assert.Equalf(v1alpha1.ServiceRunning, crV, "operator(%s) cr(%s) phase should be Running", k, crK)
		}
	}
	assert.Equalf(v1alpha1.ServiceRunning, configInstance.Status.Phase, "config(%s/%s) phase should be Running", configInstance.Namespace, configInstance.Name)

	// Retrieve OperandRequest
	retrieveOperandRequest(t, r, req, requestInstance, 2)

}

// Retrieve OperandRequest instance
func retrieveOperandRequest(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, expectedMemNum int) {
	assert := assert.New(t)
	err := r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.NoError(err)
	assert.Equal(expectedMemNum, len(requestInstance.Status.Members), "operandrequest member list should have two elements")

	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions(req.Namespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Equalf(expectedMemNum, len(subs.Items), "subscription list should have %s Subscriptions", expectedMemNum)

	csvs, err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(req.Namespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Equalf(expectedMemNum, len(csvs.Items), "csv list should have %s ClusterServiceVersions", expectedMemNum)

	for _, m := range requestInstance.Status.Members {
		assert.Equalf(v1alpha1.OperatorRunning, m.Phase.OperatorPhase, "operator(%s) phase should be Running", m.Name)
		assert.Equalf(v1alpha1.ServiceRunning, m.Phase.OperandPhase, "operand(%s) phase should be Running", m.Name)
	}
	assert.Equalf(v1alpha1.ClusterPhaseRunning, requestInstance.Status.Phase, "request(%s/%s) phase should be Running", requestInstance.Namespace, requestInstance.Name)
}

// Absent an operator from request
func absentOperand(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest) {
	assert := assert.New(t)
	requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
	err := r.client.Update(context.TODO(), requestInstance)
	assert.NoError(err)
	_, err = r.Reconcile(req)
	assert.NoError(err)
	retrieveOperandRequest(t, r, req, requestInstance, 1)
}

// Present an operator from request
func presentOperand(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest) {
	assert := assert.New(t)
	// Present an operator into request
	requestInstance.Spec.Requests[0].Operands = append(requestInstance.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
	err := r.client.Update(context.TODO(), requestInstance)
	assert.NoError(err)

	err = r.createSubscription(requestInstance, &v1alpha1.Operator{
		Name:            "etcd",
		Namespace:       req.Namespace,
		SourceName:      "community-operators",
		SourceNamespace: "openshift-marketplace",
		PackageName:     "etcd",
		Channel:         "singlenamespace-alpha",
	})
	assert.NoError(err)
	err = r.reconcileOperator(requestInstance, req)
	assert.NoError(err)
	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(req.Namespace).UpdateStatus(sub("etcd", req.Namespace, "0.0.1"))
	assert.NoError(err)
	_, err = r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(req.Namespace).Create(csv("etcd-csv.v0.0.1", req.Namespace, etcdExample))
	assert.NoError(err)
	err = r.waitForInstallPlan(requestInstance, req)
	assert.NoError(err)
	multiErr := r.reconcileOperand(requestInstance)
	assert.Empty(multiErr.errors, "all the operands reconcile should not be error")
	err = r.UpdateMemberStatus(requestInstance)
	assert.NoError(err)
	retrieveOperandRequest(t, r, req, requestInstance, 2)
}

// Mock delete OperandRequest instance, mark OperandRequest instance as delete state
func deleteOperandRequest(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest) {
	assert := assert.New(t)
	deleteTime := metav1.NewTime(time.Now())
	requestInstance.SetDeletionTimestamp(&deleteTime)
	err := r.client.Update(context.TODO(), requestInstance)
	assert.NoError(err)
	_, err = r.Reconcile(req)
	assert.NoError(err)
	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions(req.Namespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Empty(subs.Items, "all the Subscriptions should be deleted")

	csvs, err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(req.Namespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Empty(csvs.Items, "all the ClusterServiceVersions should be deleted")

	// Check if OperandRequest instance deleted
	err = r.client.Delete(context.TODO(), requestInstance)
	assert.NoError(err)
	err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.True(errors.IsNotFound(err), "retrieve operand request should be return an error of type is 'NotFound'")
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
		Status: olmv1alpha1.ClusterServiceVersionStatus{
			Phase: olmv1alpha1.CSVPhaseSucceeded,
		},
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
	  "apiVersion": "jenkins.io/v1alpha2",
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
