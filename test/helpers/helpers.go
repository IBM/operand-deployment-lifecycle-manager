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

package helpers

import (
	goctx "context"
	"fmt"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// CreateOperandRequest  is used to create an OperandRequest instance
func CreateOperandRequest(f *framework.Framework, ctx *framework.TestCtx, req *v1alpha1.OperandRequest) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- CREATE: OperandRequest Instance")
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), req, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}

	return req, nil
}

// RetrieveOperandRquest is used to get an OperandRquest instance
func RetrieveOperandRquest(f *framework.Framework, obj runtime.Object, ns string) error {
	// Get OperandRquest instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// DeleteOperandRequest is used to delete an OperandRequest instance
func DeleteOperandRequest(req *v1alpha1.OperandRequest, f *framework.Framework) error {
	fmt.Println("--- DELETE: OperandRequest Instance")
	// Delete OperandRequest instance
	if err := f.Client.Delete(goctx.TODO(), req); err != nil {
		return err
	}

	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandRquest(f, req, req.Namespace)
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

// AbsentOperandFromRequest is used to delete an operator and operand from OperandRequest
func AbsentOperandFromRequest(f *framework.Framework, ns, opName string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- ABSENT: Operator and Operand")
	// Delete the last operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err := RetrieveOperandRquest(f, req, ns); err != nil {
			return false, err
		}

		for index, op := range req.Spec.Requests[0].Operands {
			if op.Name == opName {
				req.Spec.Requests[0].Operands = append(req.Spec.Requests[0].Operands[:index], req.Spec.Requests[0].Operands[index+1:]...)
				break
			}
		}
		if err := f.Client.Update(goctx.TODO(), req); err != nil {
			fmt.Println("    --- Waiting for OperandRequest instance stable ...")
			return false, nil
		}
		if err := RetrieveOperandRquest(f, req, ns); err != nil {
			return false, err
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

// PresentOperandFromRequest is used to add an operator and operand into OperandRequest
func PresentOperandFromRequest(f *framework.Framework, ns, opName string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- PRESENT: Operator and Operand")
	// Add an operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (done bool, err error) {
		if err := RetrieveOperandRquest(f, req, ns); err != nil {
			return false, err
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
			if err := f.Client.Update(goctx.TODO(), req); err != nil {
				fmt.Println("    --- Waiting for OperandRequest instance stable ...")
				return false, nil
			}
		}
		if err := RetrieveOperandRquest(f, req, ns); err != nil {
			return false, err
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

// WaitRequestStatus is wait for request phase become to expected phase
func WaitRequestStatus(f *framework.Framework, expectedPhase v1alpha1.ClusterPhase, ns string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- WAITING: OperandRequest")
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		if err := RetrieveOperandRquest(f, req, ns); err != nil {
			return false, err
		}
		if req.Status.Phase != expectedPhase {
			fmt.Println("    --- Waiting for request phase " + expectedPhase)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return req, nil
}

// WaitRegistryStatus is wait for registry phase become to expected phase
func WaitRegistryStatus(f *framework.Framework, expectedPhase v1alpha1.RegistryPhase, ns string) (*v1alpha1.OperandRegistry, error) {
	fmt.Println("--- WAITING: OperandRegistry")
	reg := &v1alpha1.OperandRegistry{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandRegistry(f, reg, ns)
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

// WaitConfigStatus is wait for config phase become to expected phase
func WaitConfigStatus(f *framework.Framework, expectedPhase v1alpha1.ServicePhase, ns string) (*v1alpha1.OperandConfig, error) {
	fmt.Println("--- WAITING: OperandConfig")
	con := &v1alpha1.OperandConfig{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandConfig(f, con, ns)
		if err != nil {
			return false, err
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

// WaitBindInfoStatus is wait for bindinfo phase become to expected phase
func WaitBindInfoStatus(f *framework.Framework, expectedPhase v1alpha1.BindInfoPhase, ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- WAITING: OperandBindInfo")
	bi := &v1alpha1.OperandBindInfo{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandBindInfo(f, bi, ns)
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

// CreateOperandConfig is used to create an OperandConfig instance
func CreateOperandConfig(f *framework.Framework, ctx *framework.TestCtx, ns string) (*v1alpha1.OperandConfig, error) {
	// Create OperandConfig instance
	fmt.Println("--- CREATE: OperandConfig Instance")
	ci := newOperandConfigCR(config.OperandConfigCrName, ns)
	err := f.Client.Create(goctx.TODO(), ci, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}

	return ci, nil
}

// RetrieveOperandConfig is used to get an OperandConfig instance
func RetrieveOperandConfig(f *framework.Framework, obj runtime.Object, ns string) error {
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandConfigCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandConfig is used to update an OperandConfig instance
func UpdateOperandConfig(f *framework.Framework, ns string) error {
	fmt.Println("--- UPDATE: OperandConfig Instance")
	con := &v1alpha1.OperandConfig{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandConfig(f, con, ns)
		if err != nil {
			return false, err
		}
		con.Spec.Services[0].Spec = map[string]runtime.RawExtension{
			"etcdCluster": {Raw: []byte(`{"size": 3}`)},
		}
		if err := f.Client.Update(goctx.TODO(), con); err != nil {
			fmt.Println("    --- Waiting for OperandConfig instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// DeleteOperandConfig is used to delete an OperandConfig instance
func DeleteOperandConfig(f *framework.Framework, ci *v1alpha1.OperandConfig) error {
	fmt.Println("--- DELETE: OperandConfig Instance")
	if err := f.Client.Delete(goctx.TODO(), ci); err != nil {
		return err
	}
	return nil
}

// CreateOperandRegistry is used to create an OperandRegistry instance
func CreateOperandRegistry(f *framework.Framework, ctx *framework.TestCtx, ns string) (*v1alpha1.OperandRegistry, error) {
	// Create OperandRegistry instance
	fmt.Println("--- CREATE: OperandRegistry Instance")
	ri := newOperandRegistryCR(config.OperandRegistryCrName, ns)
	err := f.Client.Create(goctx.TODO(), ri, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}
	// Get OperandRegistry instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: ns}, ri)
	if err != nil {
		return nil, err
	}

	return ri, nil
}

// RetrieveOperandRegistry is used to get an OperandRegistry instance
func RetrieveOperandRegistry(f *framework.Framework, obj runtime.Object, ns string) error {
	// Get OperandRegistry instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandRegistry is used to update an OperandRegistry instance
func UpdateOperandRegistry(f *framework.Framework, ns string) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	reg := &v1alpha1.OperandRegistry{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandRegistry(f, reg, ns)
		if err != nil {
			return false, err
		}
		reg.Spec.Operators[0].Channel = "clusterwide-alpha"
		if err := f.Client.Update(goctx.TODO(), reg); err != nil {
			fmt.Println("    --- Waiting for OperandRegistry instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// DeleteOperandRegistry is used to delete an OperandRegistry instance
func DeleteOperandRegistry(f *framework.Framework, reg *v1alpha1.OperandRegistry) error {
	fmt.Println("--- DELETE: OperandRegistry Instance")

	// Delete OperandRegistry instance
	err := f.Client.Delete(goctx.TODO(), reg)
	if err != nil {
		return err
	}

	return nil
}

// CreateOperandBindInfo is used to create an OperandBindInfo instance
func CreateOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx, ns string) (*v1alpha1.OperandBindInfo, error) {
	// Create OperandBindInfo instance
	fmt.Println("--- CREATE: OperandBindInfo Instance")
	bi := newOperandBindInfoCR(config.OperandBindInfoCrName, ns)
	err := f.Client.Create(goctx.TODO(), bi, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}
	// Get OperandBindInfo instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandBindInfoCrName, Namespace: ns}, bi)
	if err != nil {
		return nil, err
	}

	return bi, nil
}

// RetrieveOperandBindInfo is used to get an OperandBindInfo instance
func RetrieveOperandBindInfo(f *framework.Framework, obj runtime.Object, ns string) error {
	// Get OperandBindInfo instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandBindInfoCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandBindInfo is used to update an OperandBindInfo instance
func UpdateOperandBindInfo(f *framework.Framework, ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- UPDATE: OperandBindInfo Instance")
	bi := &v1alpha1.OperandBindInfo{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandBindInfo(f, bi, ns)
		if err != nil {
			return false, err
		}
		secretCm := bi.Spec.Bindings["public"]
		secretCm.Configmap = "jenkins-operator-base-configuration-example"
		bi.Spec.Bindings["public"] = secretCm
		if err := f.Client.Update(goctx.TODO(), bi); err != nil {
			fmt.Println("    --- Waiting for OperandBindInfo instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return bi, nil
}

// DeleteOperandBindInfo is used to delete an OperandBindInfo instance
func DeleteOperandBindInfo(f *framework.Framework, bi *v1alpha1.OperandBindInfo) error {
	fmt.Println("--- DELETE: OperandBindInfo Instance")

	// Delete OperandBindInfo instance
	if err := f.Client.Delete(goctx.TODO(), bi); err != nil {
		return err
	}

	return nil
}

// CreateNamespace is used to create a new namespace for test
func CreateNamespace(f *framework.Framework, ctx *framework.TestCtx, name string) error {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := f.Client.Create(goctx.TODO(), obj, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// DeleteNamespace is delete the test namespace
func DeleteNamespace(f *framework.Framework, name string) error {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := f.Client.Delete(goctx.TODO(), obj); err != nil {
		return err
	}

	return nil
}

// RetrieveSecret is get a secret
func RetrieveSecret(f *framework.Framework, name, namespace string) (*corev1.Secret, error) {
	obj := &corev1.Secret{}
	// Get Secret
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
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

// RetrieveConfigmap is get a configmap
func RetrieveConfigmap(f *framework.Framework, name, namespace string) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{}
	// Get ConfigMap
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
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

// newOperandConfigCR is return an OperandRegistry CR object
func newOperandRegistryCR(name, namespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "etcd",
					Namespace:       config.TestNamespace1,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
					Scope:           v1alpha1.ScopePublic,
				},
				{
					Name:            "jenkins",
					Namespace:       config.TestNamespace1,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
				},
			},
		},
	}
}

// NewOperandRequestCR1 is return an OperandRequest CR object
func NewOperandRequestCR1(name, namespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          config.OperandRegistryCrName,
					RegistryNamespace: config.TestNamespace1,
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

// NewOperandRequestCR2 is return an OperandRequest CR object
func NewOperandRequestCR2(name, namespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          config.OperandRegistryCrName,
					RegistryNamespace: config.TestNamespace1,
					Operands: []v1alpha1.Operand{
						{
							Name: "jenkins",
							Bindings: map[string]v1alpha1.SecretConfigmap{
								"public": {
									Secret:    "jenkins-operator-credentials-example",
									Configmap: "jenkins-operator-init-configuration-example",
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
func newOperandBindInfoCR(name, namespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "jenkins",
			Registry:          config.OperandRegistryCrName,
			RegistryNamespace: config.TestNamespace1,

			Bindings: map[string]v1alpha1.SecretConfigmap{
				"public": {
					Secret:    "jenkins-operator-credentials-example",
					Configmap: "jenkins-operator-init-configuration-example",
				},
			},
		},
	}
}
