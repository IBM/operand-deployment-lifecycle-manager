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

	v1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/pkg/constant"
)

// TestConfigController runs ReconcileOperandConfig.Reconcile() against a
// fake client that tracks a OperandConfig object.
func TestConfigController(t *testing.T) {
	var (
		name              = "ibm-cloudpak-name"
		namespace         = "ibm-cloudpak"
		registryName      = "ibm-cloudpak-name"
		registryNamespace = "ibm-cloudpak"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace)

	initReconcile(t, r, req)

}

func initReconcile(t *testing.T, r ReconcileOperandConfig, req reconcile.Request) {
	assert := assert.New(t)

	_, err := r.Reconcile(req)
	assert.NoError(err)

	config := &v1alpha1.OperandConfig{}
	err = r.client.Get(context.TODO(), req.NamespacedName, config)
	assert.NoError(err)
	// Check the config init status
	assert.NotNil(config.Status, "init operator status should not be empty")
	assert.Equal(v1alpha1.ServiceNotReady, config.Status.Phase, "Overall OperandConfig phase should be 'Not Ready'")
	// TODO: Add a test case with CR deployed
}

func getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace string) ReconcileOperandConfig {
	s := scheme.Scheme
	v1alpha1.SchemeBuilder.AddToScheme(s)
	olmv1.SchemeBuilder.AddToScheme(s)
	olmv1alpha1.SchemeBuilder.AddToScheme(s)
	v1beta2.SchemeBuilder.AddToScheme(s)
	v1alpha2.SchemeBuilder.AddToScheme(s)

	initData := initClientData(name, namespace, registryName, registryNamespace, operatorNamespace)

	// Create a fake client to mock API calls.
	client := fake.NewFakeClient(initData.odlmObjs...)

	// Create a fake OLM client to mock OLM API calls.
	olmClient := fakeolmclient.NewSimpleClientset(initData.olmObjs...)

	// Return a ReconcileOperandConfig object with the scheme and fake client.
	return ReconcileOperandConfig{
		scheme:    s,
		client:    client,
		olmClient: olmClient,
	}
}

type DataObj struct {
	odlmObjs []runtime.Object
	olmObjs  []runtime.Object
}

func initClientData(name, namespace, registryName, registryNamespace, operatorNamespace string) *DataObj {
	return &DataObj{
		odlmObjs: []runtime.Object{
			operandRegistry(registryName, registryNamespace, operatorNamespace),
			operandConfig(name, namespace)},
		olmObjs: []runtime.Object{
			sub("etcd", operatorNamespace, "0.0.1"),
			sub("jenkins", operatorNamespace, "0.0.1"),
			csv("etcd-csv.v0.0.1", operatorNamespace, etcdExample),
			csv("jenkins-csv.v0.0.1", operatorNamespace, jenkinsExample),
			ip("etcd-install-plan", operatorNamespace),
			ip("jenkins-install-plan", operatorNamespace)},
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

// Return Subscription obj
func sub(name, namespace, csvVersion string) *olmv1alpha1.Subscription {
	labels := map[string]string{
		constant.OpreqLabel: "true",
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
