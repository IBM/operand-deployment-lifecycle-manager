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

package metaoperatorset

import (
	"context"

	operatorv1alpha1 "github.com/IBM/meta-operator/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileMetaOperatorSet) initServiceStatus(cr *operatorv1alpha1.MetaOperatorConfig) error {

	if cr.Status.ServiceStatus == nil {
		cr.Status.ServiceStatus = make(map[string]operatorv1alpha1.CrStatus)
	}

	for _, operator := range cr.Spec.Services {
		_, ok := cr.Status.ServiceStatus[operator.Name]
		if !ok {
			cr.Status.ServiceStatus[operator.Name] = operatorv1alpha1.CrStatus{}
		}

		if cr.Status.ServiceStatus[operator.Name].CrStatus == nil {
			tmp := cr.Status.ServiceStatus[operator.Name]
			tmp.CrStatus = make(map[string]operatorv1alpha1.ServicePhase)
			cr.Status.ServiceStatus[operator.Name] = tmp
		}
		for service := range operator.Spec {
			if _, ok := cr.Status.ServiceStatus[operator.Name].CrStatus[service]; !ok {
				cr.Status.ServiceStatus[operator.Name].CrStatus[service] = operatorv1alpha1.ServiceReady
			}
		}
	}

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileMetaOperatorSet) updateServiceStatus(cr *operatorv1alpha1.MetaOperatorConfig, operatorName, serviceName string, serviceStatus operatorv1alpha1.ServicePhase) error {

	cr.Status.ServiceStatus[operatorName].CrStatus[serviceName] = serviceStatus
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}
