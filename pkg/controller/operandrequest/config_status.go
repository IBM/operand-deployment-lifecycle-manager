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

package operandrequest

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) updateServiceStatus(cr *operatorv1alpha1.OperandConfig, operatorName, serviceName string, serviceStatus operatorv1alpha1.ServicePhase) error {
	klog.V(3).Info("Updating OperandConfig status")

	if err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (done bool, err error) {
		configInstance, err := r.getConfigInstance(cr.Name, cr.Namespace)
		if err != nil {
			return false, err
		}

		configInstance.Status.ServiceStatus[operatorName].CrStatus[serviceName] = serviceStatus
		if err := r.client.Status().Update(context.TODO(), configInstance); err != nil {
			klog.V(3).Info("Waiting for OperandConfig instance status ready ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileOperandRequest) deleteServiceStatus(cr *operatorv1alpha1.OperandConfig, operatorName, serviceName string) error {
	klog.V(3).Infof("Deleting custom resource %s from OperandConfig status", serviceName)

	if err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (done bool, err error) {
		configInstance, err := r.getConfigInstance(cr.Name, cr.Namespace)
		if err != nil {
			return false, err
		}

		delete(configInstance.Status.ServiceStatus[operatorName].CrStatus, serviceName)

		if err := r.client.Status().Update(context.TODO(), configInstance); err != nil {
			klog.V(3).Info("Waiting for OperandConfig instance status ready ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}

	return nil
}
