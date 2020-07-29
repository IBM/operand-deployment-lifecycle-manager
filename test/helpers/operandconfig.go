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
