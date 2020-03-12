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

	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

// GetOperandRequest gets an OperandRequest instance
func GetOperandRequest(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandRequest, error) {
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
func AbsentOperandFormRequest(reqCr *operator.OperandRequest, f *framework.Framework) error {
	fmt.Printf("--- ABSENT: Operator and Operand [%s]", reqCr.Spec.Requests[0].Operands[0].Name)
	// Delete first operator and related operand
	reqCr.Spec.Requests[0].Operands = reqCr.Spec.Requests[0].Operands[1:]
	if err := f.Client.Update(goctx.TODO(), reqCr); err != nil {
		return err
	}
	return nil
}

// PresentOperandFormRequest add an operator and operand into OperandRequest
func PresentOperandFormRequest(reqCr *operator.OperandRequest, f *framework.Framework) error {
	fmt.Println("--- PRESENT: Operator and Operand [etcd]")
	// Add an operator and related operand
	reqCr.Spec.Requests[0].Operands = append(reqCr.Spec.Requests[0].Operands, operator.Operand{Name: "etcd"})
	if err := f.Client.Update(goctx.TODO(), reqCr); err != nil {
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

// GetOperandConfig gets a OperandConfig instance
func GetOperandConfig(f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandConfig, error) {
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
func UpdateOperandConfig(ci *operator.OperandConfig, f *framework.Framework) error {
	fmt.Println("--- UPDATE: OperandConfig Instance")
	ci.Spec.Services[0].Spec = map[string]runtime.RawExtension{
		"etcdCluster": {Raw: []byte(`{"size": 3}`)},
	}
	if err := f.Client.Update(goctx.TODO(), ci); err != nil {
		return err
	}
	return nil
}

// DeleteOperandConfig deletes a OperandConfig instance
func DeleteOperandConfig(ci *operator.OperandConfig, f *framework.Framework) error {
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

// GetOperandRegistry get and update a OperandRegistry instance
func GetOperandRegistry(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) (*operator.OperandRegistry, error) {
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
func UpdateOperandRegistry(olmClient *olmclient.Clientset, ri *operator.OperandRegistry, f *framework.Framework) error {
	fmt.Println("--- UPDATE: OperandRegistry Instance")
	ri.Spec.Operators[0].Channel = "clusterwide-alpha"
	if err := f.Client.Update(goctx.TODO(), ri); err != nil {
		return err
	}

	return nil
}

// DeleteOperandRegistry delete a OperandRegistry instance
func DeleteOperandRegistry(olmClient *olmclient.Clientset, ri *operator.OperandRegistry, f *framework.Framework) error {
	fmt.Println("--- DELETE: OperandRegistry Instance")

	// Delete OperandRegistry instance
	err := f.Client.Delete(goctx.TODO(), ri)
	if err != nil {
		return err
	}

	return nil
}

// WaitForSubscriptionDelete waits for the subscription deleted
func WaitForSubscriptionDelete(olmClient *olmclient.Clientset, opt metav1.ObjectMeta) error {
	lastReason := ""
	fmt.Println("Waiting on subscription to be deleted [" + opt.Name + "]")
	waitErr := utilwait.PollImmediate(config.WaitForRetry, config.APITimeout, func() (done bool, err error) {
		_, err = olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Get(opt.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		lastReason = fmt.Sprintf("Waiting on subscription to be deleted" + ", Subscription.Name: " + opt.Name + " Subscription.Namespace: " + opt.Namespace)
		return false, nil
	})
	if waitErr != nil {
		return fmt.Errorf("%v: %s", waitErr, lastReason)
	}
	return nil
}

// ValidateCustomResource check the result of the OperandConfig
func ValidateCustomResource(f *framework.Framework, namespace string) error {
	fmt.Println("Validating custom resources are ready")
	configInstance := &operator.OperandConfig{}
	// Get OperandRequest instance
	err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandConfigCrName, Namespace: namespace}, configInstance)
	if err != nil {
		return err
	}
	lastReason := ""
	waitErr := utilwait.PollImmediate(config.WaitForRetry, config.APITimeout, func() (done bool, err error) {
		for operatorName, operatorState := range configInstance.Status.ServiceStatus {
			for crName, crState := range operatorState.CrStatus {
				if crState == operator.ServiceRunning {
					continue
				} else {
					lastReason = fmt.Sprintf("Waiting on custom resource to be ready" + ", custom resource name: " + crName + " Operator name: " + operatorName)
					return false, nil
				}
			}
		}
		return true, nil
	})
	if waitErr != nil {
		return fmt.Errorf("%v: %s", waitErr, lastReason)
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

// Checking Subscription are ready
func CheckingSub(f *framework.Framework, olmClient *olmclient.Clientset, reqCr *operator.OperandRequest) error {
	fmt.Println("--- CHECKING: Subscription")
	if err := wait.PollImmediate(time.Second*10, time.Minute*5, func() (bool, error) {
		for _, req := range reqCr.Spec.Requests {
			registryInstance, err := getRegistryInstance(f, req.Registry, req.RegistryNamespace)
			if err != nil {
				return false, err
			}
			for _, operand := range req.Operands {
				if registryInstance.Status.OperatorsStatus[operand.Name].Phase != operator.OperatorReady {
					return false, fmt.Errorf("subsciption[%s] phase not ready", operand.Name)
				}
			}
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// Get the OperandRegistry instance with the name and namespace
func getRegistryInstance(f *framework.Framework, name, namespace string) (*operator.OperandRegistry, error) {
	reg := &operator.OperandRegistry{}
	if err := f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, reg); err != nil {
		return nil, err
	}
	return reg, nil
}

func GetReconcileRequest(name, namespace string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}
