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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

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
