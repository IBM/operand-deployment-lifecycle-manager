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
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// CreateOperandRequest creates a OperandRequest instance
func CreateOperandRequest(f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	fmt.Println("--- CREATE: OperandRequest Instance")
	requestInstance := newOperandRequestCR(config.OperandRequestCrName, namespace)
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), requestInstance, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}

	return nil
}

// RetrieveOperandRequest gets an OperandRequest instance
func RetrieveOperandRequest(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandRequest, error) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return nil, err
	}
	reqCr := &operator.OperandRequest{}
	fmt.Println("--- GET: OperandRequest Instance")
	// Get OperandRequest instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: namespace}, reqCr)
	if err != nil {
		return nil, err
	}

	return reqCr, nil
}

// DeleteOperandRequest delete a OperandRequest instance
func DeleteOperandRequest(reqCr *operator.OperandRequest, f *framework.Framework) error {
	fmt.Println("--- DELETE: OperandRequest Instance")
	// Delete OperandRequest instance
	if err := f.Client.Delete(goctx.TODO(), reqCr); err != nil {
		return err
	}
	return nil
}

// AbsentOperandFormRequest delete an operator and operand from OperandRequest
func AbsentOperandFormRequest(f *framework.Framework, ctx *framework.TestCtx) error {
	fmt.Println("--- ABSENT: Operator and Operand")
	// Delete first operator and related operand
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		reqCr, err := RetrieveOperandRequest(f, ctx)
		if err != nil {
			return false, err
		}
		reqCr.Spec.Requests[0].Operands = reqCr.Spec.Requests[0].Operands[1:]
		if err := f.Client.Update(goctx.TODO(), reqCr); err != nil {
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
func PresentOperandFormRequest(f *framework.Framework, ctx *framework.TestCtx) error {
	fmt.Println("--- PRESENT: Operator and Operand [etcd]")
	// Add an operator and related operand
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (done bool, err error) {
		reqCr, err := RetrieveOperandRequest(f, ctx)
		if err != nil {
			return false, err
		}
		reqCr.Spec.Requests[0].Operands = append(reqCr.Spec.Requests[0].Operands, operator.Operand{Name: "etcd"})
		if err := f.Client.Update(goctx.TODO(), reqCr); err != nil {
			fmt.Println("    --- Waiting for OperandRequest instance stable ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// CheckingClusterState checking cluster phase if running
func CheckingClusterState(f *framework.Framework, ctx *framework.TestCtx) error {
	fmt.Println("--- CHECKING: Cluster Phase")
	if err := utilwait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		reqCr, err := RetrieveOperandRequest(f, ctx)
		if err != nil {
			return false, err
		}
		if reqCr.Status.Phase != operator.ClusterPhaseRunning {
			fmt.Println("    --- Waiting for cluster ready ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// CreateOperandConfig creates a OperandConfig instance
func CreateOperandConfig(f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	// Create OperandConfig instance
	fmt.Println("--- CREATE: OperandConfig Instance")
	ci := newOperandConfigCR(config.OperandConfigCrName, namespace)
	err = f.Client.Create(goctx.TODO(), ci, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}

	return nil
}

// RetrieveOperandConfig gets a OperandConfig instance
func RetrieveOperandConfig(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandConfig, error) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return nil, err
	}
	ci := &operator.OperandConfig{}
	fmt.Println("--- GET: OperandConfig Instance")
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandConfigCrName, Namespace: namespace}, ci)
	if err != nil {
		return nil, err
	}

	return ci, nil
}

// UpdateOperandConfig updates a OperandConfig instance
func UpdateOperandConfig(f *framework.Framework, ctx *framework.TestCtx) error {
	fmt.Println("--- UPDATE: OperandConfig Instance")
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		conCr, err := RetrieveOperandConfig(f, ctx)
		if err != nil {
			return false, err
		}
		conCr.Spec.Services[0].Spec = map[string]runtime.RawExtension{
			"etcdCluster": {Raw: []byte(`{"size": 3}`)},
		}
		if err := f.Client.Update(goctx.TODO(), conCr); err != nil {
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
func DeleteOperandConfig(f *framework.Framework, ci *operator.OperandConfig) error {
	fmt.Println("--- DELETE: OperandConfig Instance")
	if err := f.Client.Delete(goctx.TODO(), ci); err != nil {
		return err
	}
	return nil
}

// CreateOperandRegistry creates a OperandRegistry instance
func CreateOperandRegistry(f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	// Create OperandRegistry instance
	fmt.Println("--- CREATE: OperandRegistry Instance")
	ri := newOperandRegistryCR(config.OperandRegistryCrName, namespace)
	err = f.Client.Create(goctx.TODO(), ri, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}
	// Get OperandRegistry instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: namespace}, ri)
	if err != nil {
		return err
	}

	return nil
}

// RetrieveOperandRegistry get and update a OperandRegistry instance
func RetrieveOperandRegistry(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandRegistry, error) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("could not get namespace: %v", err)
	}
	ri := &operator.OperandRegistry{}
	fmt.Println("--- GET: OperandRegistry Instance")

	// Get OperandRegistry instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: namespace}, ri)
	if err != nil {
		return nil, err
	}

	return ri, nil
}

// UpdateOperandRegistry update a OperandRegistry instance
func UpdateOperandRegistry(f *framework.Framework, ctx *framework.TestCtx) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		regCr, err := RetrieveOperandRegistry(f, ctx)
		if err != nil {
			return false, err
		}
		regCr.Spec.Operators[0].Channel = "clusterwide-alpha"
		if err := f.Client.Update(goctx.TODO(), regCr); err != nil {
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
func DeleteOperandRegistry(f *framework.Framework, regCr *operator.OperandRegistry) error {
	fmt.Println("--- DELETE: OperandRegistry Instance")

	// Delete OperandRegistry instance
	err := f.Client.Delete(goctx.TODO(), regCr)
	if err != nil {
		return err
	}

	return nil
}

// AssertNoError confirms the error returned is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// CreateOperandBindInfo creates a OperandBindInfo instance
func CreateOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandBindInfo, error) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("could not get namespace: %v", err)
	}

	// Create OperandBindInfo instance
	fmt.Println("--- CREATE: OperandBindInfo Instance")
	bi := newOperandBindInfoCR(config.OperandBindInfoCrName, namespace)
	err = f.Client.Create(goctx.TODO(), bi, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return nil, err
	}
	// Get OperandBindInfo instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandBindInfoCrName, Namespace: namespace}, bi)
	if err != nil {
		return nil, err
	}

	return bi, nil
}

// RetrieveOperandBindInfo get and update a OperandBindInfo instance
func RetrieveOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandBindInfo, error) {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("could not get namespace: %v", err)
	}
	bi := &operator.OperandBindInfo{}
	fmt.Println("--- GET: OperandBindInfo Instance")

	// Get OperandBindInfo instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandBindInfoCrName, Namespace: namespace}, bi)
	if err != nil {
		return nil, err
	}

	return bi, nil
}

// UpdateOperandBindInfo update a OperandBindInfo instance
func UpdateOperandBindInfo(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandBindInfo, error) {
	fmt.Println("--- UPDATE: OperandBindInfo Instance")
	bi := &operator.OperandBindInfo{}
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		bi, err = RetrieveOperandBindInfo(f, ctx)
		if err != nil {
			return false, err
		}
		bi.Spec.Bindings[0].Configmap = "jenkins-operator-base-configuration-example"
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
func DeleteOperandBindInfo(f *framework.Framework, bi *operator.OperandBindInfo) error {
	fmt.Println("--- DELETE: OperandBindInfo Instance")

	// Delete OperandBindInfo instance
	if err := f.Client.Delete(goctx.TODO(), bi); err != nil {
		return err
	}

	return nil
}

//OperandConfig instance
func newOperandConfigCR(name, namespace string) *operator.OperandConfig {
	return &operator.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operator.OperandConfigSpec{
			Services: []operator.ConfigService{
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
func newOperandRegistryCR(name, namespace string) *operator.OperandRegistry {
	return &operator.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operator.OperandRegistrySpec{
			Operators: []operator.Operator{
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

func newOperandRequestCR(name, namespace string) *operator.OperandRequest {
	return &operator.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          "common-service",
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

func newOperandBindInfoCR(name, namespace string) *operator.OperandBindInfo {
	return &operator.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "jenkins-operator",
			Registry:          "common-service",
			RegistryNamespace: namespace,

			Bindings: []v1alpha1.Binding{
				{
					Scope:     "public",
					Secret:    "jenkins-operator-credentials-example",
					Configmap: "jenkins-operator-init-configuration-example",
				},
			},
		},
	}
}
