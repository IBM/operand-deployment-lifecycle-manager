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

package commonserviceset

import (
	"testing"

	// olmfake "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	operatorv1alpha1 "github.com/IBM/common-service-operator/pkg/apis/operator/v1alpha1"
)

func buildScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	return scheme, nil
}
func TestCommonServiceSetController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	var (
		name      = "common-service"
		namespace = "common-service-operator"
	)

	instanceSet := &operatorv1alpha1.CommonServiceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorv1alpha1.CommonServiceSetSpec{
			Services: []operatorv1alpha1.SetService{
				{
					Name:  "etcd",
					State: "present",
				},
				{
					Name:  "jenkins",
					State: "present",
				},
			},
		},
	}

	instanceMetaOperator := &operatorv1alpha1.MetaOperator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorv1alpha1.MetaOperatorSpec{
			Operators: []operatorv1alpha1.Operator{
				{
					Name:            "jenkins",
					Namespace:       "jenkins-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
				},
				{
					Name:            "etcd",
					Namespace:       "etcd-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
				},
			},
		},
	}

	instanceConfig := &operatorv1alpha1.CommonServiceConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operatorv1alpha1.CommonServiceConfigSpec{
			Services: []operatorv1alpha1.ConfigService{
				{
					Name: "jenkins",
					Spec: map[string]runtime.RawExtension{
						"jenkins": runtime.RawExtension{Raw: []byte(`{"service":{"port": 8081}}`)},
					},
				},
				{
					Name: "etcd",
					Spec: map[string]runtime.RawExtension{
						"etcdCluster": runtime.RawExtension{Raw: []byte(`{"size": 1}`)},
					},
				},
			},
		},
	}

	objs := []runtime.Object{
		instanceSet,
		instanceMetaOperator,
		instanceConfig,
	}

	// Register operator types with the runtime scheme.
	s, err := buildScheme()
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Create a fake client to mock API calls.
	cl := k8sfake.NewFakeClientWithScheme(s, objs...)
	// get global framework variables
	//olmcl := olmfake.NewSimpleClientset(objs...)
	// Create a ReconcileCommonServiceSet object with the scheme and fake client.
	r := &ReconcileCommonServiceSet{client: cl, scheme: s}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if !res.Requeue {
		t.Error("reconcile did not requeue request as expected")
	}

}
