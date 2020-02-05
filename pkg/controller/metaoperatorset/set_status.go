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

	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/IBM/meta-operator/pkg/apis/operator/v1alpha1"
)

// States of common services
const (
	Absent  = "absent"
	Present = "present"
)

type deployState struct {
	Type    operatorv1alpha1.ConditionType
	Status  string
	Reason  string
	Message string
}

// InstallSuccessed defines the desired state of install subscription successful
var InstallSuccessed = &deployState{
	Type:    "Install",
	Status:  "Successed",
	Reason:  "Install subscription successful",
	Message: "Install subscription successful",
}

// InstallFailed defines the desired state of install subscription failed
var InstallFailed = &deployState{
	Type:    "Install",
	Status:  "Failed",
	Reason:  "Install subscription Failed",
	Message: "Install subscription Failed",
}

// UpdateSuccessed defines the desired state of update subscription successful
var UpdateSuccessed = &deployState{
	Type:    "Update",
	Status:  "Successed",
	Reason:  "Update subscription successful",
	Message: "Update subscription successful",
}

// UpdateFailed defines the desired state of update subscription failed
var UpdateFailed = &deployState{
	Type:    "Update",
	Status:  "Failed",
	Reason:  "Update subscription Failed",
	Message: "Update subscription Failed",
}

// DeleteSuccessed defines the desired state of delete subscription successful
var DeleteSuccessed = &deployState{
	Type:    "Delete",
	Status:  "Successed",
	Reason:  "Delete subscription successful",
	Message: "Delete subscription successful",
}

// DeleteFailed defines the desired state of delete subscription failed
var DeleteFailed = &deployState{
	Type:    "Delete",
	Status:  "Failed",
	Reason:  "Delete subscription Failed",
	Message: "Delete subscription Failed",
}

func newCondition(name string, ds *deployState) operatorv1alpha1.Condition {
	now := time.Now().Format(time.RFC3339)
	return operatorv1alpha1.Condition{
		Name:           name,
		Type:           ds.Type,
		Status:         ds.Status,
		LastUpdateTime: now,
		Reason:         ds.Reason,
		Message:        ds.Message,
	}
}

func getCondition(cr *operatorv1alpha1.MetaOperatorSet, name string, t operatorv1alpha1.ConditionType) (int, *operatorv1alpha1.Condition) {
	for i, c := range cr.Status.Conditions {
		if name == c.Name && t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func setCondition(cr *operatorv1alpha1.MetaOperatorSet, c operatorv1alpha1.Condition) {
	pos, cp := getCondition(cr, c.Name, c.Type)
	if cp != nil {
		if cp.Status == c.Status && cp.Reason == c.Reason && cp.Message == c.Message {
			return
		}
		cr.Status.Conditions[pos] = c
	} else {
		cr.Status.Conditions = append(cr.Status.Conditions, c)
	}
}

func (r *ReconcileMetaOperatorSet) updateConditionStatus(cr *operatorv1alpha1.MetaOperatorSet, name string, ds *deployState) error {
	c := newCondition(name, ds)
	setCondition(cr, c)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileMetaOperatorSet) updatePhaseStatus(cr *operatorv1alpha1.MetaOperatorSet, phase operatorv1alpha1.ClusterPhase) error {
	cr.Status.Phase = phase
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileMetaOperatorSet) updateMemberStatus(cr *operatorv1alpha1.MetaOperatorSet) error {
	subs, err := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/mos-control",
	})
	if err != nil {
		return err
	}
	members := operatorv1alpha1.MembersStatus{}
	for _, s := range subs.Items {
		if s.Status.InstalledCSV != "" {
			members.Ready = append(members.Ready, s.Name)
		} else {
			members.Unready = append(members.Unready, s.Name)
		}
	}
	cr.Status.Members = members
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}
