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

func (r *ReconcileMetaOperatorSet) updateOperatorStatus(cr *operatorv1alpha1.MetaOperatorCatalog, operatorName string, operatorStatus operatorv1alpha1.OperatorPhase) error {

	if cr.Status.OperatorsStatus == nil {
		cr.Status.OperatorsStatus = make(map[string]operatorv1alpha1.OperatorPhase)
	}

	cr.Status.OperatorsStatus[operatorName] = operatorStatus

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileMetaOperatorSet) deleteOperatorStatus(cr *operatorv1alpha1.MetaOperatorCatalog, operatorName string) error {

	if cr.Status.OperatorsStatus != nil {
		delete(cr.Status.OperatorsStatus, operatorName)
	}

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}

	return nil
}
