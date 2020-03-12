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

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) updateMemberStatus(cr *operatorv1alpha1.OperandRequest) error {
	klog.V(3).Info("Updating OperandRequest member status")
	for _, req := range cr.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}

		for _, ro := range req.Operands {
			opt := r.getOperatorFromRegistryInstance(ro.Name, registryInstance)
			operatorPhase, err := r.getOperatorPhase(opt.Name, opt.Namespace)
			operandPhase := getOperandPhase(configInstance.Status.ServiceStatus[ro.Name].CrStatus)
			if err != nil {
				return err
			}
			cr.SetMemberStatus(ro.Name, operatorPhase, operandPhase)
		}
	}
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) getOperatorPhase(name, namespace string) (olmv1alpha1.ClusterServiceVersionPhase, error) {
	klog.V(3).Info("Get OperatorPhase for OperandRequest member status")
	sub, err := r.olmClient.OperatorsV1alpha1().Subscriptions(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	var operatorPhase olmv1alpha1.ClusterServiceVersionPhase
	csvName := sub.Status.InstalledCSV
	if csvName != "" {
		csv, err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(namespace).Get(csvName, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return operatorPhase, err
		}
		if csv.Status.Phase == "" {
			operatorPhase = "Pending"
		} else {
			operatorPhase = csv.Status.Phase
		}
	}
	return operatorPhase, nil
}

func getOperandPhase(sp map[string]operatorv1alpha1.ServicePhase) operatorv1alpha1.ServicePhase {
	klog.V(3).Info("Get OperandPhase for OperandRequest member status")
	operandStatusStat := struct {
		readyNum   int
		runningNum int
		failedNum  int
	}{
		readyNum:   0,
		runningNum: 0,
		failedNum:  0,
	}
	for _, v := range sp {
		switch v {
		case operatorv1alpha1.ServiceReady:
			operandStatusStat.readyNum++
		case operatorv1alpha1.ServiceRunning:
			operandStatusStat.runningNum++
		case operatorv1alpha1.ServiceFailed:
			operandStatusStat.failedNum++
		default:
		}
	}
	operandPhase := operatorv1alpha1.ServiceReady
	if operandStatusStat.failedNum > 0 {
		operandPhase = operatorv1alpha1.ServiceFailed
	} else if operandStatusStat.readyNum > 0 {
		operandPhase = operatorv1alpha1.ServiceReady
	} else if operandStatusStat.runningNum > 0 {
		operandPhase = operatorv1alpha1.ServiceRunning

	}
	return operandPhase
}

func (r *ReconcileOperandRequest) updateClusterPhase(cr *operatorv1alpha1.OperandRequest) error {
	klog.V(3).Info("Update OperandRequest phase status")
	clusterStatusStat := struct {
		creatingNum int
		runningNum  int
		failedNum   int
	}{
		creatingNum: 0,
		runningNum:  0,
		failedNum:   0,
	}

	for _, m := range cr.Status.Members {
		switch m.Phase.OperatorPhase {
		case olmv1alpha1.CSVPhasePending, olmv1alpha1.CSVPhaseInstalling,
			olmv1alpha1.CSVPhaseInstallReady, olmv1alpha1.CSVPhaseReplacing,
			olmv1alpha1.CSVPhaseDeleting:
			clusterStatusStat.creatingNum++
		case olmv1alpha1.CSVPhaseFailed, olmv1alpha1.CSVPhaseUnknown:
			clusterStatusStat.failedNum++
		case olmv1alpha1.CSVPhaseSucceeded:
			clusterStatusStat.runningNum++
		default:
		}

		switch m.Phase.OperandPhase {
		case operatorv1alpha1.ServiceReady:
			clusterStatusStat.creatingNum++
		case operatorv1alpha1.ServiceRunning:
			clusterStatusStat.runningNum++
		case operatorv1alpha1.ServiceFailed:
			clusterStatusStat.failedNum++
		default:
		}
	}

	var clusterPhase operatorv1alpha1.ClusterPhase
	if clusterStatusStat.failedNum > 0 {
		clusterPhase = operatorv1alpha1.ClusterPhaseFailed
	} else if clusterStatusStat.creatingNum > 0 {
		clusterPhase = operatorv1alpha1.ClusterPhaseCreating
	} else if clusterStatusStat.runningNum > 0 {
		clusterPhase = operatorv1alpha1.ClusterPhaseRunning
	} else {
		clusterPhase = operatorv1alpha1.ClusterPhaseNone
	}
	cr.SetClusterPhase(clusterPhase)

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}
