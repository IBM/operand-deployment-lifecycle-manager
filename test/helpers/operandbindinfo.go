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
