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

package v1alpha1

import (
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OperandRequestSpec defines the desired state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// +listType=set
	Services []SetService `json:"services"`
}

// SetService is the service configuration for a common service
type SetService struct {
	Name    string `json:"name"`
	Channel string `json:"channel,omitempty"`
	State   string `json:"state"`
}

// ConditionType is the condition of a service
type ConditionType string

// ClusterPhase is the phase of the installation
type ClusterPhase string

// Constants are used for state
const (
	ConditionInstall ConditionType = "Install"
	ConditionUpdate  ConditionType = "Update"
	ConditionDelete  ConditionType = "Delete"

	ClusterPhaseNone     ClusterPhase = ""
	ClusterPhasePending  ClusterPhase = "Pending"
	ClusterPhaseCreating ClusterPhase = "Creating"
	ClusterPhaseUpdating ClusterPhase = "Updating"
	ClusterPhaseDeleting ClusterPhase = "Deleting"
	ClusterPhaseRunning  ClusterPhase = "Running"
	ClusterPhaseFailed   ClusterPhase = "Failed"
)

// Condition defines the current state of operator deploy
type Condition struct {
	Name           string        `json:"name,omitempty"`
	Type           ConditionType `json:"type,omitempty"`
	Status         string        `json:"status,omitempty"`
	LastUpdateTime string        `json:"lastUpdateTime,omitempty"`
	Reason         string        `json:"reason,omitempty"`
	Message        string        `json:"message,omitempty"`
}

// OperandRequestStatus defines the observed state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Conditions represents the current state of the Set Service
	// +optional
	// +listType=set
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operand status of the set
	// +optional
	Members []MemberStatus `json:"member,omitempty"`
	// Phase is the cluster running phase
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase ClusterPhase `json:"phase"`
}

// MemberPhase show the phase of the operator and operator instance
type MemberPhase struct {
	OperatorPhase olmv1alpha1.ClusterServiceVersionPhase `json:"operatorPhase,omitempty"`
	OperandPhase  ServicePhase                           `json:"operandPhase,omitempty"`
}

// MemberStatus show if the Operator is ready
type MemberStatus struct {
	// The member name are the same as the subscription name
	Name string `json:"name"`
	// The operand phase include None, Creating, Running, Failed
	Phase MemberPhase `json:"phase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequest is the Schema for the operandrequests API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandrequests,shortName=opreq,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandRequest"
type OperandRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandRequestSpec   `json:"spec,omitempty"`
	Status OperandRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequestList contains a list of OperandRequest
type OperandRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperandRequest{}, &OperandRequestList{})
}
