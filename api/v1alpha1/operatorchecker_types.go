//
// Copyright 2021 IBM Corporation
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OperatorCheckerStatus defines the observed state of OperatorChecker.
type OperatorCheckerStatus struct {
	// Conditions represents the current state of the OperatorChecker Service.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions",xDescriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operand status of the set.
	// +optional
	Members []MemberStatus `json:"members,omitempty"`
	// Phase is the cluster running phase.
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Phase",xDescriptors="urn:alm:descriptor:io.kubernetes.phase"
	// +optional
	Phase OperatorCheckerPhase `json:"phase,omitempty"`
}

// OperatorCheckerPhase defines the operator status.
type OperatorCheckerPhase string

// OperatorChecker phase
const (
	OperatorCheckerReady    OperatorCheckerPhase = "Ready for Deployment"
	OperatorCheckerRunning  OperatorCheckerPhase = "Running"
	OperatorCheckerPending  OperatorCheckerPhase = "Pending"
	OperatorCheckerUpdating OperatorCheckerPhase = "Updating"
	OperatorCheckerFailed   OperatorCheckerPhase = "Failed"
	OperatorCheckerWaiting  OperatorCheckerPhase = "Waiting for CatalogSource being ready"
	OperatorCheckerInit     OperatorCheckerPhase = "Initialized"
	OperatorCheckerNone     OperatorCheckerPhase = ""
)

// OperatorChecker is the Schema for the OperatorChecker API.
type OperatorChecker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status OperatorCheckerStatus `json:"status,omitempty"`
}

// OperatorCheckerList contains a list of OperatorChecker.
type OperatorCheckerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperatorChecker `json:"items"`
}
