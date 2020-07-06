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

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/pkg/constant"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/pkg/controller/common"
)

// TestRequestController runs ReconcileOperandRequest.Reconcile() against a
// fake client that tracks a OperandRequest object.
func TestRequestController(t *testing.T) {
	var (
		name              = "ibm-cloudpak-name"
		namespace         = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-service"
		operatorNamespace = "ibm-operators"
	)

	req := getReconcileRequest(name, namespace)
	r := getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace)
	requestInstance := &v1alpha1.OperandRequest{}

	initReconcile(t, r, req, requestInstance, registryName, registryNamespace, operatorNamespace)

	// absentOperand(t, r, req, requestInstance, operatorNamespace)

	// presentOperand(t, r, req, requestInstance, operatorNamespace)

	updateOperandCustomResource(t, r, req, registryName, registryNamespace)

	// absentOperandCustomResource(t, r, req, registryName, registryNamespace)

	// presentOperandCustomResource(t, r, req, registryName, registryNamespace)

	// deleteOperandRequest(t, r, req, requestInstance, operatorNamespace)
}

// Init reconcile the OperandRequest
func initReconcile(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, registryName, registryNamespace, operatorNamespace string) {
	assert := assert.New(t)
	res, err := r.Reconcile(req)
	if res.Requeue {
		t.Error("Reconcile requeued request as not expected")
	}
	assert.NoError(err)

	// Retrieve OperandRegistry
	registryInstance, err := fetch.FetchOperandRegistry(r.client, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
	assert.NoError(err)
	// for k, v := range registryInstance.Status.OperatorsStatus {
	// 	assert.Equalf(v1alpha1.OperatorRunning, v.Phase, "operator(%s) phase should be Running", k)
	// 	assert.NotEqualf(-1, registryInstance.GetReconcileRequest(k, req), "reconcile requests should be include %s", req.NamespacedName)
	// }
	// assert.Equalf(v1alpha1.OperatorRunning, registryInstance.Status.Phase, "registry(%s/%s) phase should be Running", registryInstance.Namespace, registryInstance.Name)

	// Retrieve OperandConfig
	// configInstance, err := r.getConfigInstance(registryName, registryNamespace)
	// assert.NoError(err)
	// for k, v := range configInstance.Status.ServiceStatus {
	// 	for crK, crV := range v.CrStatus {
	// 		assert.Equalf(v1alpha1.ServiceRunning, crV, "operator(%s) cr(%s) phase should be Running", k, crK)
	// 	}
	// }
	// assert.Equalf(v1alpha1.ServiceRunning, configInstance.Status.Phase, "config(%s/%s) phase should be Running", configInstance.Namespace, configInstance.Name)

	// Retrieve OperandRequest
	retrieveOperandRequest(t, r, req, requestInstance, 2, operatorNamespace)

	err = checkOperandCustomResource(t, r, registryInstance, 3, 8081, "3.2.13")
	assert.NoError(err)
}

// Retrieve OperandRequest instance
func retrieveOperandRequest(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, expectedMemNum int, operatorNamespace string) {
	assert := assert.New(t)
	err := r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
	assert.NoError(err)
	assert.Equal(expectedMemNum, len(requestInstance.Status.Members), "operandrequest member list should have two elements")

	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions(operatorNamespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Equalf(expectedMemNum, len(subs.Items), "subscription list should have %s Subscriptions", expectedMemNum)

	csvs, err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(operatorNamespace).List(metav1.ListOptions{})
	assert.NoError(err)
	assert.Equalf(expectedMemNum, len(csvs.Items), "csv list should have %s ClusterServiceVersions", expectedMemNum)

	for _, m := range requestInstance.Status.Members {
		assert.Equalf(v1alpha1.OperatorRunning, m.Phase.OperatorPhase, "operator(%s) phase should be Running", m.Name)
		assert.Equalf(v1alpha1.ServiceRunning, m.Phase.OperandPhase, "operand(%s) phase should be Running", m.Name)
	}
	assert.Equalf(v1alpha1.ClusterPhaseRunning, requestInstance.Status.Phase, "request(%s/%s) phase should be Running", requestInstance.Namespace, requestInstance.Name)
}

// // Absent an operator from request
// func absentOperand(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
// 	err := r.client.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	retrieveOperandRequest(t, r, req, requestInstance, 1, operatorNamespace)
// }

// // Present an operator from request
// func presentOperand(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	// Present an operator into request
// 	requestInstance.Spec.Requests[0].Operands = append(requestInstance.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
// 	err := r.client.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)

// 	err = r.createSubscription(requestInstance, &v1alpha1.Operator{
// 		Name:            "etcd",
// 		Namespace:       operatorNamespace,
// 		SourceName:      "community-operators",
// 		SourceNamespace: "openshift-marketplace",
// 		PackageName:     "etcd",
// 		Channel:         "singlenamespace-alpha",
// 	})
// 	assert.NoError(err)
// 	err = r.reconcileOperator(requestInstance, req)
// 	assert.NoError(err)
// 	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(operatorNamespace).UpdateStatus(sub("etcd", operatorNamespace, "0.0.1"))
// 	assert.NoError(err)
// 	_, err = r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(operatorNamespace).Create(csv("etcd-csv.v0.0.1", operatorNamespace, etcdExample))
// 	assert.NoError(err)
// 	multiErr := r.reconcileOperand(requestInstance)
// 	assert.Empty(multiErr.Errors, "all the operands reconcile should not be error")
// 	// err = r.updateMemberStatus(requestInstance)
// 	// assert.NoError(err)
// 	retrieveOperandRequest(t, r, req, requestInstance, 2, operatorNamespace)
// }

// // Mock delete OperandRequest instance, mark OperandRequest instance as delete state
// func deleteOperandRequest(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, requestInstance *v1alpha1.OperandRequest, operatorNamespace string) {
// 	assert := assert.New(t)
// 	deleteTime := metav1.NewTime(time.Now())
// 	requestInstance.SetDeletionTimestamp(&deleteTime)
// 	err := r.client.Update(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions(operatorNamespace).List(metav1.ListOptions{})
// 	assert.NoError(err)
// 	assert.Empty(subs.Items, "all the Subscriptions should be deleted")

// 	csvs, err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(operatorNamespace).List(metav1.ListOptions{})
// 	assert.NoError(err)
// 	assert.Empty(csvs.Items, "all the ClusterServiceVersions should be deleted")

// 	// Check if OperandRequest instance deleted
// 	err = r.client.Delete(context.TODO(), requestInstance)
// 	assert.NoError(err)
// 	err = r.client.Get(context.TODO(), req.NamespacedName, requestInstance)
// 	assert.True(errors.IsNotFound(err), "retrieve operand request should be return an error of type is 'NotFound'")
// }

func getReconciler(name, namespace, registryName, registryNamespace, operatorNamespace string) ReconcileOperandRequest {
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

// Update an operand custom resource from config
func updateOperandCustomResource(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, registryName, registryNamespace string) {
	assert := assert.New(t)
	// Retrieve OperandConfig
	configInstance, err := fetch.FetchOperandConfig(r.client, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
	assert.NoError(err)
	for id, operator := range configInstance.Spec.Services {
		if operator.Name == "etcd" {
			configInstance.Spec.Services[id].Spec = map[string]runtime.RawExtension{
				"etcdCluster": {Raw: []byte(`{"size": 1}`)},
			}
		}
	}
	err = r.client.Update(context.TODO(), configInstance)
	assert.NoError(err)
	res, err := r.Reconcile(req)
	if res.Requeue {
		t.Error("Reconcile requeued request as not expected")
	}
	assert.NoError(err)
	configInstance, err = fetch.FetchOperandConfig(r.client, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
	assert.NoError(err)
	assert.Equal(v1alpha1.ServiceRunning, configInstance.Status.ServiceStatus["etcd"].CrStatus["etcdCluster"], "The status of etcdCluster should be running in the OperandConfig")
	updatedRequestInstance := &v1alpha1.OperandRequest{}
	err = r.client.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
	assert.NoError(err)
	for _, operator := range updatedRequestInstance.Status.Members {
		if operator.Name == "etcd" {
			assert.Equal(v1alpha1.ServiceRunning, operator.Phase.OperandPhase, "The etcd phase status should be running in the OperandRequest")
		}
	}
	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")

	registryInstance, err := fetch.FetchOperandRegistry(r.client, types.NamespacedName{Name: registryName, Namespace: registryNamespace})
	assert.NoError(err)
	err = checkOperandCustomResource(t, r, registryInstance, 1, 8081, "3.2.13")
	assert.NoError(err)
}

// // Absent an operand custom resource from config
// func absentOperandCustomResource(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, registryName, registryNamespace string) {
// 	assert := assert.New(t)
// 	// Retrieve OperandConfig
// 	configInstance, err := r.getConfigInstance(registryName, registryNamespace)
// 	assert.NoError(err)
// 	for id, operator := range configInstance.Spec.Services {
// 		if operator.Name == "etcd" {
// 			configInstance.Spec.Services[id].Spec = make(map[string]runtime.RawExtension)
// 		}
// 	}
// 	err = r.client.Update(context.TODO(), configInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	configInstance, err = r.getConfigInstance(registryName, registryNamespace)
// 	assert.NoError(err)
// 	assert.Equal(v1alpha1.ServiceNone, configInstance.Status.ServiceStatus["etcd"].CrStatus["etcdCluster"], "The status of etcdCluster should be cleaned up in the OperandConfig")
// 	updatedRequestInstance := &v1alpha1.OperandRequest{}
// 	err = r.client.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
// 	assert.NoError(err)
// 	for _, operator := range updatedRequestInstance.Status.Members {
// 		if operator.Name == "etcd" {
// 			assert.Equal(v1alpha1.ServiceNone, operator.Phase.OperandPhase, "The etcd phase status should be none in the OperandRequest")
// 		}
// 	}
// 	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")
// }

// // Present an operand custom resource from config
// func presentOperandCustomResource(t *testing.T, r ReconcileOperandRequest, req reconcile.Request, registryName, registryNamespace string) {
// 	assert := assert.New(t)
// 	// Retrieve OperandConfig
// 	configInstance := operandConfig(registryName, registryNamespace)
// 	err := r.client.Update(context.TODO(), configInstance)
// 	assert.NoError(err)
// 	res, err := r.Reconcile(req)
// 	if res.Requeue {
// 		t.Error("Reconcile requeued request as not expected")
// 	}
// 	assert.NoError(err)
// 	configInstance, err = r.getConfigInstance(registryName, registryNamespace)
// 	assert.NoError(err)
// 	assert.Equal(v1alpha1.ServiceRunning, configInstance.Status.ServiceStatus["etcd"].CrStatus["etcdCluster"], "The status of etcdCluster should be running in the OperandConfig")
// 	updatedRequestInstance := &v1alpha1.OperandRequest{}
// 	err = r.client.Get(context.TODO(), req.NamespacedName, updatedRequestInstance)
// 	assert.NoError(err)
// 	for _, operator := range updatedRequestInstance.Status.Members {
// 		if operator.Name == "etcd" {
// 			assert.Equal(v1alpha1.ServiceRunning, operator.Phase.OperandPhase, "The etcd phase status should be none in the OperandRequest")
// 		}
// 	}
// 	assert.Equal(v1alpha1.ClusterPhaseRunning, updatedRequestInstance.Status.Phase, "The cluster phase status should be running in the OperandRequest")
// }

func checkOperandCustomResource(t *testing.T, r ReconcileOperandRequest, registryInstance *v1alpha1.OperandRegistry, etcdSize, jenkinsPort int, etcdVersion string) error {
	assert := assert.New(t)
	for _, operator := range registryInstance.Spec.Operators {
		if operator.Name == "etcd" {
			etcdCluster := &v1beta2.EtcdCluster{}
			err := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      "example",
				Namespace: operator.Namespace,
			}, etcdCluster)
			if err != nil {
				return err
			}
			assert.Equalf(etcdCluster.Spec.Size, etcdSize, "The size of etcdCluster should be %d, but it is %d", etcdSize, etcdCluster.Spec.Size)
			assert.Equalf(etcdCluster.Spec.Version, etcdVersion, "The version of etcdCluster should be %s, but it is %s", etcdVersion, etcdCluster.Spec.Version)
		}
		if operator.Name == "jenkins" {
			jenkins := &v1alpha2.Jenkins{}
			err := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      "example",
				Namespace: operator.Namespace,
			}, jenkins)
			if err != nil {
				return err
			}
			assert.Equalf(jenkins.Spec.Service.Port, int32(jenkinsPort), "The post of jenkins service should be %d, but it is %d", int32(jenkinsPort), jenkins.Spec.Service.Port)
		}
	}
	return nil
}

type DataObj struct {
	odlmObjs []runtime.Object
	olmObjs  []runtime.Object
}

func initClientData(name, namespace, registryName, registryNamespace, operatorNamespace string) *DataObj {
	return &DataObj{
		odlmObjs: []runtime.Object{
			operandRegistry(registryName, registryNamespace, operatorNamespace),
			operandRequest(name, namespace, registryName, registryNamespace),
			operandConfig(registryName, registryNamespace)},
		olmObjs: []runtime.Object{
			sub("etcd", operatorNamespace, "0.0.1"),
			sub("jenkins", operatorNamespace, "0.0.1"),
			csv("etcd-csv.v0.0.1", operatorNamespace, etcdExample),
			csv("jenkins-csv.v0.0.1", operatorNamespace, jenkinsExample),
			ip("etcd-install-plan", operatorNamespace),
			ip("jenkins-install-plan", operatorNamespace)},
	}
}

// Return OperandRegistry obj
func operandRegistry(registryName, registryNamespace, operatorNamespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryName,
			Namespace: registryNamespace,
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

// Return OperandConfig obj
func operandConfig(configName, configNamespace string) *v1alpha1.OperandConfig {
	return &v1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configName,
			Namespace: configNamespace,
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
func operandRequest(name, namespace, registryName, registryNamespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
