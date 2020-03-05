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

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	operator "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// CreateTest creates a OperandRequest instance
func CreateTest(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	// create.OperandConfig custom resource
	fmt.Println("--- CREATE: OperandConfigCR Instance")
	configInstance := newOperandConfigCR(config.OperandConfigCrName, namespace)
	err = f.Client.Create(goctx.TODO(), configInstance, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}

	// create OperandRegistry custom resource
	fmt.Println("--- CREATE: OperandRegistry Instance")
	operandRegistryInstance := newOperandRegistryCR(config.OperandRegistryCrName, namespace)
	err = f.Client.Create(goctx.TODO(), operandRegistryInstance, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}

	// create OperandRequest custom resource
	requestInstance := &operator.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.OperandRequestCrName,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          "common-service",
					RegistryNamespace: "ibm-common-services",
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

	fmt.Println("--- CREATE: OperandRequest Instance")
	// use TestCtx's create helper to create the object and add a cleanup function for the new object
	err = f.Client.Create(goctx.TODO(), requestInstance, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil {
		return err
	}
	// wait for all the csv ready
	optMap, err := GetOperators(f, namespace)
	if err != nil {
		return err
	}

	for _, req := range requestInstance.Spec.Requests {
		for _, operand := range req.Operands {
			opt := optMap[operand.Name]
			err = WaitForSubCsvReady(olmClient, metav1.ObjectMeta{Name: opt.Name, Namespace: opt.Namespace})
			if err != nil {
				return err
			}
		}
	}

	err = ValidatecustomResource(f, namespace)
	if err != nil {
		return err
	}
	return nil
}

// UpdateTest updates a OperandRequest instance
// func UpdateTest(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) error {
// 	namespace, err := ctx.GetNamespace()
// 	if err != nil {
// 		return err
// 	}
// 	requestInstance := &operator.OperandRequest{}
// 	fmt.Println("--- UPDATE: subscription")
// 	// Get OperandRequest instance
// 	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: namespace}, requestInstance)
// 	if err != nil {
// 		return err
// 	}

// 	requestInstance.Spec.Requests[0].Channel = "clusterwide-alpha"
// 	err = f.Client.Update(goctx.TODO(), requestInstance)
// 	if err != nil {
// 		return err
// 	}

// 	// wait for updated csv ready
// 	optMap, err := GetOperators(f, namespace)
// 	if err != nil {
// 		return err
// 	}
// 	opt := optMap[requestInstance.Spec.Services[0].Name]
// 	err = WaitForSubCsvReady(olmClient, metav1.ObjectMeta{Name: opt.Name, Namespace: opt.Namespace})
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

//DeleteTest delete a OperandRequest instance
func DeleteTest(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}
	requestInstance := &operator.OperandRequest{}
	fmt.Println("--- DELETE: subscription")
	// Get OperandRequest instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRequestCrName, Namespace: namespace}, requestInstance)
	if err != nil {
		return err
	}
	// Delete first operator
	operandName := requestInstance.Spec.Requests[0].Operands[0].Name
	requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
	err = f.Client.Update(goctx.TODO(), requestInstance)
	if err != nil {
		return err
	}

	optMap, err := GetOperators(f, namespace)
	if err != nil {
		return err
	}
	opt := optMap[operandName]
	// Waiting for subscription deleted
	err = WaitForSubscriptionDelete(olmClient, metav1.ObjectMeta{Name: opt.Name, Namespace: opt.Namespace})
	if err != nil {
		return err
	}
	return nil
}

// UpdateConfigTest updates a OperandConfig instance
func UpdateConfigTest(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return err
	}
	configInstance := &operator.OperandConfig{}
	fmt.Println("--- UPDATE: custom resource")
	// Get OperandRequest instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandConfigCrName, Namespace: namespace}, configInstance)
	if err != nil {
		return err
	}

	configInstance.Spec.Services[0].Spec = map[string]runtime.RawExtension{
		"etcdCluster": {Raw: []byte(`{"size": 3}`)},
	}
	err = f.Client.Update(goctx.TODO(), configInstance)
	if err != nil {
		return err
	}

	err = ValidatecustomResource(f, namespace)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperandRegistryTest updates a OperandRegistry instance
func UpdateOperandRegistryTest(olmClient *olmclient.Clientset, f *framework.Framework, ctx *framework.TestCtx) error {
	namespace, err := ctx.GetNamespace()
	if err != nil {
		return err
	}
	operandRegistryInstance := &operator.OperandRegistry{}
	fmt.Println("--- UPDATE: operandRegistry Instance")

	// Get OperandRegistry instance
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: namespace}, operandRegistryInstance)
	if err != nil {
		return err
	}

	operandRegistryInstance.Spec.Operators[0].Channel = "clusterwide-alpha"
	err = f.Client.Update(goctx.TODO(), operandRegistryInstance)
	if err != nil {
		return err
	}

	// wait for updated csv ready
	optMap, err := GetOperators(f, namespace)
	if err != nil {
		return err
	}
	opt := optMap[operandRegistryInstance.Spec.Operators[0].Name]
	err = WaitForSubCsvReady(olmClient, metav1.ObjectMeta{Name: opt.Name, Namespace: opt.Namespace})
	if err != nil {
		return err
	}
	return nil
}

// GetOperators get a operator list waiting for being installed
func GetOperators(f *framework.Framework, namespace string) (map[string]operator.Operator, error) {
	registryInstance := &operator.OperandRegistry{}
	lastReason := ""
	waitErr := utilwait.PollImmediate(config.WaitForRetry, config.APITimeout, func() (done bool, err error) {
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: config.OperandRegistryCrName, Namespace: namespace}, registryInstance)
		if err != nil {
			if errors.IsNotFound(err) {
				lastReason = fmt.Sprintf("Waiting on odlm instance to be created [operand-deployment-lifecycle-manager]")
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if waitErr != nil {
		return nil, fmt.Errorf("%v: %s", waitErr, lastReason)
	}
	optMap := make(map[string]operator.Operator)
	for _, v := range registryInstance.Spec.Operators {
		optMap[v.Name] = v
	}
	return optMap, nil
}

// WaitForSubCsvReady waits for the subscription and csv create success
func WaitForSubCsvReady(olmClient *olmclient.Clientset, opt metav1.ObjectMeta) error {
	lastReason := ""
	sub := &olmv1alpha1.Subscription{}
	fmt.Println("Waiting for Subscription created [" + opt.Name + "]")
	waitErr := utilwait.PollImmediate(config.WaitForRetry, config.APITimeout, func() (done bool, err error) {
		foundSub, err := olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Get(opt.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				lastReason = fmt.Sprintf("Waiting on subscription to be created" + ", Subscription.Name: " + opt.Name + " Subscription.Namespace: " + opt.Namespace)
				return false, nil
			}
			return false, err
		}
		if foundSub.Status.InstalledCSV == "" {
			lastReason = fmt.Sprintf("Waiting on CSV to be installed" + ", Subscription.Name: " + opt.Name + " Subscription.Namespace: " + opt.Namespace)
			return false, nil
		}
		sub = foundSub
		return true, nil
	})
	if waitErr != nil {
		return fmt.Errorf("%v: %s", waitErr, lastReason)
	}

	fmt.Println("Waiting for CSV status succeeded [" + sub.Status.InstalledCSV + "]")
	waitErr = utilwait.PollImmediate(config.WaitForRetry, config.APITimeout, func() (done bool, err error) {
		csv, err := olmClient.OperatorsV1alpha1().ClusterServiceVersions(opt.Namespace).Get(sub.Status.InstalledCSV, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				lastReason = fmt.Sprintf("Waiting on CSV to be created" + ", CSV.Name: " + sub.Status.InstalledCSV + " CSV.Namespace: " + opt.Namespace)
				return false, nil
			}
			return false, err
		}

		// New csv found and phase is succeeeded
		if sub.Status.InstalledCSV == csv.Name && csv.Status.Phase == "Succeeded" {
			return true, nil
		}
		lastReason = fmt.Sprintf("Waiting on CSV status succeeded" + ", CSV.Name: " + sub.Status.InstalledCSV + " CSV.Namespace: " + opt.Namespace)
		return false, nil
	})
	if waitErr != nil {
		return fmt.Errorf("%v: %s", waitErr, lastReason)
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

// ValidatecustomResource check the result of the OperandConfig
func ValidatecustomResource(f *framework.Framework, namespace string) error {
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
					Namespace:       "etcd-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "singlenamespace-alpha",
					TargetNamespaces: []string{
						"etcd-operator",
					},
				},
				{
					Name:            "jenkins",
					Namespace:       "jenkins-operator",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
					TargetNamespaces: []string{
						"jenkins-operator",
					},
				},
			},
		},
	}
}
