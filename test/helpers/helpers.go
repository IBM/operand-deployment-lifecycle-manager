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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// CreateOperandRequest creates a OperandRequest instance
func CreateOperandRequest(f *framework.Framework, ctx *framework.TestCtx, req *v1alpha1.OperandRequest) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- CREATE: OperandRequest Instance")
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), req, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}

	return req, nil
}

// DeleteOperandRequest delete a OperandRequest instance
func DeleteOperandRequest(reqCr *v1alpha1.OperandRequest, f *framework.Framework) error {
	fmt.Println("--- DELETE: OperandRequest Instance")
	// Delete OperandRequest instance
	if err := f.Client.Delete(goctx.TODO(), reqCr); err != nil {
		return err
	}
	return nil
}

// AbsentOperandFormRequest delete an operator and operand from OperandRequest
func AbsentOperandFormRequest(f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	fmt.Println("--- ABSENT: Operator and Operand")
	// Delete first operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: ns}, req); err != nil {
			return false, err
		}
		req.Spec.Requests[0].Operands = req.Spec.Requests[0].Operands[1:]
		if err := f.Client.Update(goctx.TODO(), req); err != nil {
			fmt.Println("    --- Waiting for OperandRequest instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// PresentOperandFormRequest add an operator and operand into OperandRequest
func PresentOperandFormRequest(f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	fmt.Println("--- PRESENT: Operator and Operand [etcd]")
	// Add an operator and related operand
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (done bool, err error) {
		if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: ns}, req); err != nil {
			return false, err
		}
		req.Spec.Requests[0].Operands = append(req.Spec.Requests[0].Operands, v1alpha1.Operand{Name: "etcd"})
		if err := f.Client.Update(goctx.TODO(), req); err != nil {
			fmt.Println("    --- Waiting for OperandRequest instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// WaitRequestStatus wait for request phase is expected phase
func WaitRequestStatus(f *framework.Framework, ctx *framework.TestCtx, expectedPhase v1alpha1.ClusterPhase, ns string) (*v1alpha1.OperandRequest, error) {
	fmt.Println("--- WAITING: OperandRequest")
	req := &v1alpha1.OperandRequest{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: ns}, req); err != nil {
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

// WaitRegistryStatus wait for registry phase is expected phase
func WaitRegistryStatus(f *framework.Framework, ctx *framework.TestCtx, expectedPhase v1alpha1.OperatorPhase, ns string) (*v1alpha1.OperandRegistry, error) {
	fmt.Println("--- WAITING: OperandRegistry")
	reg := &v1alpha1.OperandRegistry{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandRegistry(f, ctx, reg, ns)
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

// WaitConfigStatus wait for config phase is expected phase
func WaitConfigStatus(f *framework.Framework, ctx *framework.TestCtx, expectedPhase v1alpha1.ServicePhase, ns string) (*v1alpha1.OperandConfig, error) {
	fmt.Println("--- WAITING: OperandConfig")
	con := &v1alpha1.OperandConfig{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandConfig(f, ctx, con, ns)
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

// WaitBindInfoStatus wait for bindinfo phase is expected phase
func WaitBindInfoStatus(f *framework.Framework, ctx *framework.TestCtx, expectedPhase v1alpha1.BindInfoPhase, ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- WAITING: OperandBindInfo")
	bi := &v1alpha1.OperandBindInfo{}
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		err := RetrieveOperandBindInfo(f, ctx, bi, ns)
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

// CreateOperandConfig creates a OperandConfig instance
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

// RetrieveOperandConfig gets a OperandConfig instance
func RetrieveOperandConfig(f *framework.Framework, ctx *framework.TestCtx, obj runtime.Object, ns string) error {
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandConfigCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandConfig updates a OperandConfig instance
func UpdateOperandConfig(f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	fmt.Println("--- UPDATE: OperandConfig Instance")
	con := &v1alpha1.OperandConfig{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandConfig(f, ctx, con, ns)
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

// DeleteOperandConfig deletes a OperandConfig instance
func DeleteOperandConfig(f *framework.Framework, ci *v1alpha1.OperandConfig) error {
	fmt.Println("--- DELETE: OperandConfig Instance")
	if err := f.Client.Delete(goctx.TODO(), ci); err != nil {
		return err
	}
	return nil
}

// CreateOperandRegistry creates a OperandRegistry instance
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

// RetrieveOperandRegistry get and update a OperandRegistry instance
func RetrieveOperandRegistry(f *framework.Framework, ctx *framework.TestCtx, obj runtime.Object, ns string) error {
	// Get OperandRegistry instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandRegistry update a OperandRegistry instance
func UpdateOperandRegistry(f *framework.Framework, ctx *framework.TestCtx, ns string) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	reg := &v1alpha1.OperandRegistry{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandRegistry(f, ctx, reg, ns)
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

// DeleteOperandRegistry delete a OperandRegistry instance
func DeleteOperandRegistry(f *framework.Framework, regCr *v1alpha1.OperandRegistry) error {
	fmt.Println("--- DELETE: OperandRegistry Instance")

	// Delete OperandRegistry instance
	err := f.Client.Delete(goctx.TODO(), regCr)
	if err != nil {
		return err
	}

	return nil
}

// CreateOperandBindInfo creates a OperandBindInfo instance
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

// RetrieveOperandBindInfo get and update a OperandBindInfo instance
func RetrieveOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx, obj runtime.Object, ns string) error {
	// Get OperandBindInfo instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandBindInfoCrName, Namespace: ns}, obj)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandBindInfo update a OperandBindInfo instance
func UpdateOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx, ns string) (*v1alpha1.OperandBindInfo, error) {
	fmt.Println("--- UPDATE: OperandBindInfo Instance")
	bi := &v1alpha1.OperandBindInfo{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		err = RetrieveOperandBindInfo(f, ctx, bi, ns)
		if err != nil {
			return false, err
		}
		bi.Spec.Bindings.Public.Configmap = "jenkins-operator-base-configuration-example"
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

// DeleteOperandBindInfo delete a OperandBindInfo instance
func DeleteOperandBindInfo(f *framework.Framework, bi *v1alpha1.OperandBindInfo) error {
	fmt.Println("--- DELETE: OperandBindInfo Instance")

	// Delete OperandBindInfo instance
	if err := f.Client.Delete(goctx.TODO(), bi); err != nil {
		return err
	}

	return nil
}

// CreateNamespace create a new namespace for test
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
	if err != nil {
		return err
	}

	return nil
}

// DeleteNamespace the test namespace
func DeleteNamespace(f *framework.Framework, ctx *framework.TestCtx, name string) error {
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

// RetrieveSecret get a secret
func RetrieveSecret(f *framework.Framework, ctx *framework.TestCtx, name, namespace string) (*corev1.Secret, error) {
	obj := &corev1.Secret{}
	// Get Secret
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// RetrieveConfigmap get a configmap
func RetrieveConfigmap(f *framework.Framework, ctx *framework.TestCtx, name, namespace string) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{}
	// Get ConfigMap
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

//OperandConfig instance
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

// OperandRegistry instance
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
							Bindings: v1alpha1.Binding{
								Public: v1alpha1.SecretConfigmap{
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

func newOperandBindInfoCR(name, namespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "jenkins",
			Registry:          "common-service",
			RegistryNamespace: namespace,

			Bindings: v1alpha1.Binding{
				Public: v1alpha1.SecretConfigmap{
					Secret:    "jenkins-operator-credentials-example",
					Configmap: "jenkins-operator-init-configuration-example",
				},
			},
		},
	}
}
