//
// Copyright 2022 IBM Corporation
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

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
)

func createTestNamespace(namespace string) {
	log.Info("Creating namespace", "name", namespace)
	Eventually(func() error {
		if _, err := clientset.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}).Should(Succeed())
}

func deleteTestNamespace(namespace string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
	if ns != nil {
		log.Info("Removing namespace", "name", namespace)
		err = clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())
		log.Info("Waiting for namespace to be removed", "timeout", APITimeout)
		err := wait.Poll(APIRetry, APITimeout, func() (bool, error) {
			od, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			}
			if od != nil {
				return false, nil
			}
			return true, nil
		})
		Expect(err).ToNot(HaveOccurred())
	}
}

func createOperandRequest(req *v1alpha1.OperandRequest) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- CREATE: OperandRequest Instance")
	err := k8sClient.Create(context.TODO(), req)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func retrieveOperandRequest(obj client.Object, ns string) error {
	// Get OperandRequest instance
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandRequestCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// deleteOperandRequest is used to delete the OperandRequest
func deleteOperandRequest(req *v1alpha1.OperandRequest) error {
	fmt.Println("--- DELETE: OperandRequest Instance")
	// Delete OperandRequest instance
	if err := k8sClient.Delete(context.TODO(), req); err != nil {
		return err
	}

	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		err = retrieveOperandRequest(req, req.Namespace)
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		fmt.Println("    --- Waiting for OperandRequest deleted ...")
		return false, err
	}); err != nil {
		return err
	}
	return nil
}

// absentOperandFromRequest disables an Operand from the OperandRequest
func absentOperandFromRequest(ns, opName string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- ABSENT: Operator and Operand")
	// Delete the last operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err := retrieveOperandRequest(req, ns); err != nil {
			return false, client.IgnoreNotFound(err)
		}

		for index, op := range req.Spec.Requests[0].Operands {
			if op.Name == opName {
				req.Spec.Requests[0].Operands = append(req.Spec.Requests[0].Operands[:index], req.Spec.Requests[0].Operands[index+1:]...)
				break
			}
		}
		if err := k8sClient.Update(context.TODO(), req); err != nil {
			fmt.Println("    --- Waiting for OperandRequest instance stable ...")
			return false, nil
		}
		if err := retrieveOperandRequest(req, ns); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		for _, opstatus := range req.Status.Members {
			if opstatus.Name == opName {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return req, nil
}

// presentOperandFromRequest enables an Operand from the OperandRequest
func presentOperandFromRequest(ns, opName string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- PRESENT: Operator and Operand")
	// Add an operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := wait.PollImmediate(APIRetry, APITimeout, func() (done bool, err error) {
		if err := retrieveOperandRequest(req, ns); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		createOp := true
		for _, op := range req.Spec.Requests[0].Operands {
			if op.Name == opName {
				createOp = false
				break
			}
		}
		if createOp {
			req.Spec.Requests[0].Operands = append(req.Spec.Requests[0].Operands, v1alpha1.Operand{Name: opName})
			if err := k8sClient.Update(context.TODO(), req); err != nil {
				fmt.Println("    --- Waiting for OperandRequest instance stable ...")
				return false, nil
			}
		}
		if err := retrieveOperandRequest(req, ns); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		for _, opstatus := range req.Status.Members {
			if opstatus.Name == opName {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return nil, err
	}
	return req, nil
}

// waitRequestStatus is wait for request phase become to expected phase
func waitRequestStatusRunning(ns string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- WAITING: OperandRequest")
	req := &v1alpha1.OperandRequest{}
	if err := wait.PollImmediate(APIRetry, APITimeout, func() (bool, error) {
		if err := retrieveOperandRequest(req, ns); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if req.Status.Phase != v1alpha1.ClusterPhaseRunning {
			fmt.Println("    --- Waiting for request phase " + v1alpha1.ClusterPhaseRunning)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return req, nil
}

// waitRegistryStatus is wait for registry phase become to expected phase
func waitRegistryStatus(expectedPhase v1alpha1.RegistryPhase) (*v1alpha1.OperandRegistry, error) {
	fmt.Println("--- WAITING: OperandRegistry")
	reg := &v1alpha1.OperandRegistry{}
	if err := wait.PollImmediate(APIRetry, APITimeout, func() (bool, error) {
		err := retrieveOperandRegistry(reg, OperandRegistryNamespace)
		if err != nil {
			return false, err
		}
		if reg.Status.Phase != expectedPhase {
			fmt.Println("    --- Waiting for registry phase " + expectedPhase)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return reg, nil
}

// waitConfigStatus is wait for config phase become to expected phase
func waitConfigStatus(expectedPhase v1alpha1.ServicePhase, ns string) (*v1alpha1.OperandConfig, error) {
	fmt.Println("--- WAITING: OperandConfig")
	con := &v1alpha1.OperandConfig{}
	if err := wait.PollImmediate(APIRetry, APITimeout, func() (bool, error) {
		err := retrieveOperandConfig(con, ns)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if con.Status.Phase != expectedPhase {
			fmt.Println("    --- Waiting for config phase " + expectedPhase)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return con, nil
}

// waitBindInfoStatus is wait for bindinfo phase become to expected phase
func waitBindInfoStatus(expectedPhase v1alpha1.BindInfoPhase, ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- WAITING: OperandBindInfo")
	bi := &v1alpha1.OperandBindInfo{}
	if err := wait.PollImmediate(APIRetry, APITimeout, func() (bool, error) {
		err := retrieveOperandBindInfo(bi, ns)
		if err != nil {
			return false, err
		}
		if bi.Status.Phase != expectedPhase {
			fmt.Println("    --- Waiting for bindinfo phase " + expectedPhase)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return bi, nil
}

// createOperandConfig is used to create an OperandConfig instance
func createOperandConfig(ns string) (*v1alpha1.OperandConfig, error) {
	// Create OperandConfig instance
	fmt.Println("--- CREATE: OperandConfig Instance")
	ci := newOperandConfigCR(OperandConfigCrName, ns)
	err := k8sClient.Create(context.TODO(), ci)
	if err != nil {
		return nil, err
	}

	// Get OperandConfig instance
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandConfigCrName, Namespace: ns}, ci)
	if err != nil {
		return nil, err
	}

	return ci, nil
}

// retrieveOperandConfig is used to get an OperandConfig instance
func retrieveOperandConfig(obj client.Object, ns string) error {
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandConfigCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// updateJaegerReplicas is used to update an OperandConfig instance
func updateJaegerStrategy(ns string) error {
	fmt.Println("--- UPDATE: OperandConfig Instance")
	con := &v1alpha1.OperandConfig{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		err = retrieveOperandConfig(con, ns)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		con.Spec.Services[0].Spec = map[string]v1alpha1.ExtensionWithMarker{
			"jaeger": {
				RawExtension: runtime.RawExtension{Raw: []byte(`{"strategy": "allinone"}`)},
			},
		}
		if err := k8sClient.Update(context.TODO(), con); err != nil {
			fmt.Println("    --- Waiting for OperandConfig instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// deleteOperandConfig is used to delete an OperandConfig instance
func deleteOperandConfig(ci *v1alpha1.OperandConfig) error {
	fmt.Println("--- DELETE: OperandConfig Instance")
	if err := k8sClient.Delete(context.TODO(), ci); err != nil {
		return err
	}
	return nil
}

// createOperandRegistry is used to create an OperandRegistry instance
func createOperandRegistry(ns, OperatorNamespace string) (*v1alpha1.OperandRegistry, error) {
	// Create OperandRegistry instance
	fmt.Println("--- CREATE: OperandRegistry Instance")
	var ri *v1alpha1.OperandRegistry
	if isRunningOnKind() {
		ri = newOperandRegistryCRforKind(OperandRegistryCrName, ns, OperatorNamespace)
	} else {
		ri = newOperandRegistryCR(OperandRegistryCrName, ns, OperatorNamespace)
	}
	err := k8sClient.Create(context.TODO(), ri)
	if err != nil {
		return nil, err
	}
	// Get OperandRegistry instance
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandRegistryCrName, Namespace: ns}, ri)
	if err != nil {
		return nil, err
	}

	return ri, nil
}

// retrieveOperandRegistry is used to get an OperandRegistry instance
func retrieveOperandRegistry(obj client.Object, ns string) error {
	// Get OperandRegistry instance
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandRegistryCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// updateJaegerChannel is used to update the channel for the jaeger operator
func updateJaegerChannel(ns string) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	reg := &v1alpha1.OperandRegistry{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		err = retrieveOperandRegistry(reg, ns)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		reg.Spec.Operators[0].Channel = "stable"
		reg.Spec.Operators[0].InstallMode = "cluster"
		if err := k8sClient.Update(context.TODO(), reg); err != nil {
			fmt.Println("    --- Waiting for OperandRegistry instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	if err := checkNameSpaceandOperatorGroup("openshift-operators"); err != nil {
		return err
	}
	return nil
}

// checkNameSpaceandOperatorGroup makes
func checkNameSpaceandOperatorGroup(ns string) error {
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		createTestNamespace(ns)
		existOG := &olmv1.OperatorGroupList{}
		if err := k8sClient.List(context.TODO(), existOG, &client.ListOptions{Namespace: ns}); err != nil {
			return false, err
		}
		if len(existOG.Items) == 0 {
			og := &olmv1.OperatorGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "olm",
					Namespace: ns,
				},
				Spec: olmv1.OperatorGroupSpec{
					TargetNamespaces: []string{},
				},
			}
			if err := k8sClient.Create(context.TODO(), og); err != nil && !errors.IsAlreadyExists(err) {
				return false, err
			}
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// updateMongodbScope is used to update the channel for the Mongodb operator
func updateMongodbScope(ns string) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	reg := &v1alpha1.OperandRegistry{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		err = retrieveOperandRegistry(reg, ns)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		reg.Spec.Operators[1].Scope = v1alpha1.ScopePublic
		if err := k8sClient.Update(context.TODO(), reg); err != nil {
			fmt.Println("    --- Waiting for OperandRegistry instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// deleteOperandRegistry is used to delete an OperandRegistry instance
func deleteOperandRegistry(reg *v1alpha1.OperandRegistry) error {
	fmt.Println("--- DELETE: OperandRegistry Instance")

	// Delete OperandRegistry instance
	err := k8sClient.Delete(context.TODO(), reg)
	if err != nil {
		return err
	}

	return nil
}

// createOperandBindInfo is used to create an OperandBindInfo instance
func createOperandBindInfo(ns, RegistryNamespace string) (*v1alpha1.OperandBindInfo, error) {
	// Create OperandBindInfo instance
	fmt.Println("--- CREATE: OperandBindInfo Instance")
	bi := newOperandBindInfoCR(OperandBindInfoCrName, ns, RegistryNamespace)
	err := k8sClient.Create(context.TODO(), bi)
	if err != nil {
		return nil, err
	}
	// Get OperandBindInfo instance
	err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandBindInfoCrName, Namespace: ns}, bi)
	if err != nil {
		return nil, err
	}

	return bi, nil
}

// retrieveOperandBindInfo is used to get an OperandBindInfo instance
func retrieveOperandBindInfo(obj client.Object, ns string) error {
	// Get OperandBindInfo instance
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: OperandBindInfoCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// updateOperandBindInfo is used to update an OperandBindInfo instance
func updateOperandBindInfo(ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- UPDATE: OperandBindInfo Instance")
	bi := &v1alpha1.OperandBindInfo{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		err = retrieveOperandBindInfo(bi, ns)
		if err != nil {
			return false, err
		}
		secretCm := bi.Spec.Bindings["public"]
		secretCm.Configmap = "mongodb-second-configmap"
		bi.Spec.Bindings["public"] = secretCm
		if err := k8sClient.Update(context.TODO(), bi); err != nil {
			fmt.Println("    --- Waiting for OperandBindInfo instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return bi, nil
}

// deleteOperandBindInfo is used to delete an OperandBindInfo instance
func deleteOperandBindInfo(bi *v1alpha1.OperandBindInfo) error {
	fmt.Println("--- DELETE: OperandBindInfo Instance")

	// Delete OperandBindInfo instance
	if err := k8sClient.Delete(context.TODO(), bi); err != nil {
		return err
	}

	return nil
}

// retrieveSecret is get a secret
func retrieveSecret(name, namespace string) (*corev1.Secret, error) {
	obj := &corev1.Secret{}
	// Get Secret
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("    --- Waiting for Secret copied ...")
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// retrieveConfigmap is get a configmap
func retrieveConfigmap(name, namespace string) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{}
	// Get ConfigMap
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("    --- Waiting for ConfigMap copied ...")
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// retrieveSubscription is get a subscription
func retrieveSubscription(name, namespace string) (*olmv1alpha1.Subscription, error) {
	obj := &olmv1alpha1.Subscription{}
	// Get subscription
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("    --- Waiting for Subscription created ...")
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// retrieveMongodb is get a custom resource
func retrieveMongodb(name, namespace string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "atlas.mongodb.com", Version: "v1", Kind: "AtlasDeployment"})
	// Get subscription
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("    --- Waiting for custom resource AtlasDeployment created ...")
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// retrieveJaeger is get a custom resource
func retrieveJaeger(name, namespace string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "jaegertracing.io", Version: "v1", Kind: "Jaeger"})
	// Get subscription
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("    --- Waiting for custom resource Jaeger created ...")
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// newOperandConfigCR is return an OperandConfig CR object
func newOperandConfigCR(name, namespace string) *v1alpha1.OperandConfig {
	return &v1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandConfigSpec{
			Services: []v1alpha1.ConfigService{
				{
					Name: "jaeger",
					Spec: map[string]v1alpha1.ExtensionWithMarker{
						"jaeger": {
							RawExtension: runtime.RawExtension{Raw: []byte(`{"strategy": "streaming"}`)}},
					},
				},
				{
					Name: "mongodb-atlas-kubernetes",
					Spec: map[string]v1alpha1.ExtensionWithMarker{
						"atlasDeployment": {
							RawExtension: runtime.RawExtension{Raw: []byte(`{"deploymentSpec":{"name": "test-deployment"}}`)},
						},
					},
					Resources: []v1alpha1.ConfigResource{
						{
							Name:       "mongodb-configmap",
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Labels: map[string]string{
								"mongodb": "configmap",
							},
							Annotations: map[string]string{
								"mongodb": "configmap",
							},
							Data: &runtime.RawExtension{
								Raw: []byte(`{"data": {"port": "8081"}}`),
							},
							Force: false,
						},
						{
							Name:       "mongodb-second-configmap",
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Labels: map[string]string{
								"mongodb": "configmap",
							},
							Annotations: map[string]string{
								"mongodb": "configmap",
							},
							Data: &runtime.RawExtension{
								Raw: []byte(`{"data": {"port": "8080"}}`),
							},
							Force: false,
						},
						{
							Name:       "mongodb-secret",
							APIVersion: "v1",
							Kind:       "Secret",
							Labels: map[string]string{
								"mongodb": "secret",
							},
							Annotations: map[string]string{
								"mongodb": "secret",
							},
							Data: &runtime.RawExtension{
								Raw: []byte(`{"type": "Opaque", "data": {"password": "UyFCXCpkJHpEc2I9", "username": "YWRtaW4="}}`),
							},
							Force: false,
						},
					},
				},
			},
		},
	}
}

// newOperandConfigCR is return an OperandRegistry CR object
func newOperandRegistryCR(name, namespace, OperatorNamespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "jaeger",
					Namespace:       OperatorNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jaeger",
					Channel:         "stable",
					Scope:           v1alpha1.ScopePublic,
				},
				{
					Name:            "mongodb-atlas-kubernetes",
					Namespace:       OperatorNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "mongodb-atlas-kubernetes",
					Channel:         "stable",
				},
			},
		},
	}
}

// newOperandConfigCRforKind is return an OperandRegistry CR object
func newOperandRegistryCRforKind(name, namespace, OperatorNamespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "jaeger",
					Namespace:       OperatorNamespace,
					SourceName:      "operatorhubio-catalog",
					SourceNamespace: "olm",
					PackageName:     "jaeger",
					Channel:         "stable",
					Scope:           v1alpha1.ScopePublic,
				},
				{
					Name:            "mongodb-atlas-kubernetes",
					Namespace:       OperatorNamespace,
					SourceName:      "operatorhubio-catalog",
					SourceNamespace: "olm",
					PackageName:     "mongodb-atlas-kubernetes",
					Channel:         "stable",
				},
			},
		},
	}
}

// newOperandRequestWithoutBindinfo is return an OperandRequest CR object
func newOperandRequestWithoutBindinfo(name, namespace, RegistryNamespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          OperandRegistryCrName,
					RegistryNamespace: RegistryNamespace,
					Operands: []v1alpha1.Operand{
						{
							Name: "jaeger",
						},
						{
							Name: "mongodb-atlas-kubernetes",
						},
					},
				},
			},
		},
	}
}

// newOperandRequestWithBindinfo is return an OperandRequest CR object
func newOperandRequestWithBindinfo(name, namespace, RegistryNamespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          OperandRegistryCrName,
					RegistryNamespace: RegistryNamespace,
					Operands: []v1alpha1.Operand{
						{
							Name: "mongodb-atlas-kubernetes",
							Bindings: map[string]v1alpha1.Bindable{
								"public": {
									Secret:    "mongodb-secret",
									Configmap: "mongodb-configmap",
								},
							},
						},
					},
				},
			},
		},
	}
}

// newOperandBindInfoCR is return an OperandBindInfo CR object
func newOperandBindInfoCR(name, namespace, RegistryNamespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "mongodb-atlas-kubernetes",
			Registry:          OperandRegistryCrName,
			RegistryNamespace: RegistryNamespace,

			Bindings: map[string]v1alpha1.Bindable{
				"public": {
					Secret:    "mongodb-secret",
					Configmap: "mongodb-configmap",
				},
			},
		},
	}
}

func waitConfigmapDeletion(name, namespace string) error {
	obj := &corev1.ConfigMap{}
	if err := wait.PollImmediate(WaitForRetry, WaitForTimeout, func() (done bool, err error) {
		if err = k8sClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		fmt.Println("    --- Waiting for ConfigMap deleted ...")
		return false, nil
	}); err != nil {
		return err
	}
	return nil
}
