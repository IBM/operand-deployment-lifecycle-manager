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

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	fakeolmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

// TestSetController runs ReconcileOperandRequest.Reconcile() against a
// fake client that tracks a OperandRequest object.
func TestInitSet(t *testing.T) {

	req := getRequest()
	r := getReconcilerWithoutMOS()

	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("reconcile requeue which is not expected")
	}
}

func TestUpdateSetChannel(t *testing.T) {
	req := getRequest()
	r := getReconcilerWithMOS()
	assert := assert.New(t)
	// Update default channel from alpha to beta
	mos := mos(req.Name, req.Namespace, "present", "beta")

	err := r.client.Update(context.TODO(), mos)
	assert.NoError(err)

	_, err = r.Reconcile(req)
	assert.NoError(err)
}

func TestUpdateSetState(t *testing.T) {
	req := getRequest()
	r := getReconcilerWithMOS()
	assert := assert.New(t)
	// Update default state from present to absent
	mos := mos(req.Name, req.Namespace, "absent", "")

	err := r.client.Update(context.TODO(), mos)
	assert.NoError(err)

	_, err = r.Reconcile(req)
	assert.NoError(err)

	found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(req.Name).List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/mos-control",
	})
	assert.NoError(err)
	assert.Nil(found.Items, "The subscription list should be empty.")
}

func TestDeleteSet(t *testing.T) {
	req := getRequest()
	r := getReconcilerWithMOS()
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	mos := &v1alpha1.OperandRequest{}
	err = r.client.Get(context.TODO(), req.NamespacedName, mos)
	assert.NoError(err)

	// Mark OperandRequest Cr as delete state
	deleteTime := metav1.NewTime(time.Now())
	mos.SetDeletionTimestamp(&deleteTime)

	err = r.client.Update(context.TODO(), mos)
	assert.NoError(err)

	_, err = r.Reconcile(req)
	assert.NoError(err)
}

// Return OperandRegistry obj
func mog(name, namespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
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
					Channel:         "alpha",
				},
			},
		},
	}
}

// Return OperandConfig obj
func moc(name, namespace string) *v1alpha1.OperandConfig {
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
			},
		},
	}
}

// Return OperandRequest obj
func mos(name, namespace, state, channel string) *v1alpha1.OperandRequest {
	if channel == "" {
		channel = "alpha"
	}
	if state == "" {
		state = "present"
	}
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Services: []v1alpha1.SetService{
				{
					Name:    "etcd",
					Channel: channel,
					State:   state,
				},
			},
		},
	}
}

// Return Subscription obj
func sub(name, namespace, csvVersion string) *olmv1alpha1.Subscription {
	labels := map[string]string{
		"operator.ibm.com/mos-control": "true",
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
func csv(name, namespace string) *olmv1alpha1.ClusterServiceVersion {
	return &olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

func getRequest() reconcile.Request {
	var (
		name      = "common-service"
		namespace = "operand-deployment-lifecycle-manager"
	)
	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func getReconcilerWithoutMOS() ReconcileOperandRequest {
	var (
		name      = "common-service"
		namespace = "operand-deployment-lifecycle-manager"
	)
	metaObjs := []runtime.Object{
		mog(name, namespace),
		moc(name, namespace)}
	olmObjs := []runtime.Object{
		sub("etcd", "etcd-operator", "0.0.1"),
		csv("etcd-csv.v0.0.1", "etcd-operator"),
		ip("etcd-install-plan", "etcd-operator")}
	return getReconciler(metaObjs, olmObjs)
}

func getReconcilerWithMOS() ReconcileOperandRequest {
	var (
		name      = "common-service"
		namespace = "operand-deployment-lifecycle-manager"
	)
	metaObjs := []runtime.Object{
		mog(name, namespace),
		mos(name, namespace, "", ""),
		moc(name, namespace)}
	olmObjs := []runtime.Object{
		sub("etcd", "etcd-operator", "0.0.1"),
		csv("etcd-csv.v0.0.1", "etcd-operator"),
		ip("etcd-install-plan", "etcd-operator")}
	return getReconciler(metaObjs, olmObjs)
}

func getReconciler(metaObjs, olmObjs []runtime.Object) ReconcileOperandRequest {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	olmv1.SchemeBuilder.AddToScheme(s)
	olmv1alpha1.SchemeBuilder.AddToScheme(s)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(metaObjs...)

	// Create a fake OLM client to mock OLM API calls.
	olmClient := fakeolmclient.NewSimpleClientset(olmObjs...)

	// Return a ReconcileOperandRequest object with the scheme and fake client.
	return ReconcileOperandRequest{
		scheme:    s,
		client:    client,
		olmClient: olmClient,
	}
}
