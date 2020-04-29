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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"

	operatorv1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1"
)

func (r *ReconcileOperandRequest) updateServiceStatus(configInstance *operatorv1.OperandConfig, operatorName, serviceName string, serviceStatus operatorv1.ServicePhase) error {
	if configInstance.Status.ServiceStatus == nil {
		klog.V(3).Info("Initializing OperandConfig status")
		configInstance.InitConfigServiceStatus()
	}

	klog.V(3).Info("Updating OperandConfig status")

	if err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (done bool, err error) {

		_, ok := configInstance.Status.ServiceStatus[operatorName]

		if !ok {
			configInstance.Status.ServiceStatus[operatorName] = operatorv1.CrStatus{}
		}

		if configInstance.Status.ServiceStatus[operatorName].CrStatus == nil {
			tmp := configInstance.Status.ServiceStatus[operatorName]
			tmp.CrStatus = make(map[string]operatorv1.ServicePhase)
			configInstance.Status.ServiceStatus[operatorName] = tmp
		}

		configInstance.Status.ServiceStatus[operatorName].CrStatus[serviceName] = serviceStatus
		configInstance.UpdateOperandPhase()

		if err := r.client.Status().Update(context.TODO(), configInstance); err != nil {
			klog.V(3).Info("Waiting for OperandConfig instance status ready ...")
			configInstance, _ = r.getConfigInstance(configInstance.Name, configInstance.Namespace)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileOperandRequest) deleteServiceStatus(cr *operatorv1.OperandConfig, operatorName, serviceName string) error {
	klog.V(3).Infof("Deleting custom resource %s from OperandConfig status", serviceName)

	if err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (done bool, err error) {
		configInstance, err := r.getConfigInstance(cr.Name, cr.Namespace)
		if err != nil {
			return false, err
		}
		if configInstance.Status.ServiceStatus[operatorName].CrStatus == nil {
			return true, nil
		}
		for name := range configInstance.Status.ServiceStatus[operatorName].CrStatus {
			if strings.EqualFold(name, serviceName) {
				delete(configInstance.Status.ServiceStatus[operatorName].CrStatus, name)
				configInstance.UpdateOperandPhase()
			}
		}

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
