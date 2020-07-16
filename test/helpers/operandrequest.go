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
	"k8s.io/apimachinery/pkg/api/errors"
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

// RetrieveOperandRequest is used to get an OperandRequest instance
func RetrieveOperandRequest(f *framework.Framework, obj runtime.Object, ns string) error {
	// Get OperandRequest instance
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
		err = RetrieveOperandRequest(f, req, req.Namespace)
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
		if err := RetrieveOperandRequest(f, req, ns); err != nil {
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
		if err := RetrieveOperandRequest(f, req, ns); err != nil {
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
		if err := RetrieveOperandRequest(f, req, ns); err != nil {
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
		if err := RetrieveOperandRequest(f, req, ns); err != nil {
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
		if err := RetrieveOperandRequest(f, req, ns); err != nil {
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
