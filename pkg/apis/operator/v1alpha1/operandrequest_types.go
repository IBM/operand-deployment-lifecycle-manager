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
	"time"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

	// Services defines a list of operator installation states
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Services []RequestService `json:"services"`
}

// RequestService is the service configuration for a common service
type RequestService struct {
	Name    string `json:"name"`
	Channel string `json:"channel,omitempty"`
	State   string `json:"state"`
}

// ConditionType is the condition of a service
type ConditionType string

// ClusterPhase is the phase of the installation
type ClusterPhase string

// ResourceType is the type of condition use
type ResourceType string

// Constants are used for state
const (
	ConditionCreating ConditionType = "Creating"
	ConditionUpdating ConditionType = "Updating"
	ConditionDeleting ConditionType = "Deleting"

	ClusterPhaseNone     ClusterPhase = ""
	ClusterPhaseCreating ClusterPhase = "Creating"
	ClusterPhaseRunning  ClusterPhase = "Running"
	ClusterPhaseFailed   ClusterPhase = "Failed"

	ResourceTypeSub     ResourceType = "subscription"
	ResourceTypeCsv     ResourceType = "csv"
	ResourceTypeOperand ResourceType = "operand"
)

// Conditions represents the current state of the Request Service
// A condition might not show up if it is not happening.
type Condition struct {
	// Type of condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// OperandRequestStatus defines the observed state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Conditions represents the current state of the Request Service
	// +optional
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operand status of the set
	// +optional
	// +listType=set
	Members []MemberStatus `json:"members,omitempty"`
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

func (rs *OperandRequestStatus) SetCreatingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionCreating, corev1.ConditionTrue, "Creating "+string(rt), "Creating "+string(rt)+" "+name)
	rs.setCondition(*c)
}

func (rs *OperandRequestStatus) SetUpdatingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionUpdating, corev1.ConditionTrue, "Updating "+string(rt), "Updating "+string(rt)+" "+name)
	rs.setCondition(*c)
}

func (rs *OperandRequestStatus) SetDeletingCondition(name string, rt ResourceType) {
	c := newCondition(ConditionDeleting, corev1.ConditionTrue, "Deleting "+string(rt), "Deleting "+string(rt)+" "+name)
	rs.setCondition(*c)
}

func (rs *OperandRequestStatus) setCondition(c Condition) {
	pos, cp := getCondition(rs, c.Type, c.Message)
	if cp != nil {
		rs.Conditions[pos] = c
	} else {
		rs.Conditions = append(rs.Conditions, c)
	}
}

func getCondition(status *OperandRequestStatus, t ConditionType, msg string) (int, *Condition) {
	for i, c := range status.Conditions {
		if t == c.Type && msg == c.Message {
			return i, &c
		}
	}
	return -1, nil
}

func newCondition(condType ConditionType, status corev1.ConditionStatus, reason, message string) *Condition {
	now := time.Now().Format(time.RFC3339)
	return &Condition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

func (rs *OperandRequestStatus) SetMemberStatus(name string, operatorPhase olmv1alpha1.ClusterServiceVersionPhase, operandPhase ServicePhase) {
	m := newMemberStatus(name, operatorPhase, operandPhase)
	pos, mp := getMemberStatus(rs, name)
	if mp != nil {
		if m.Phase.OperatorPhase != mp.Phase.OperatorPhase {
			rs.Members[pos].Phase.OperatorPhase = m.Phase.OperatorPhase
		}
		if m.Phase.OperandPhase != mp.Phase.OperandPhase {
			rs.Members[pos].Phase.OperandPhase = m.Phase.OperandPhase
		}
	} else {
		rs.Members = append(rs.Members, m)
	}
}

func (rs *OperandRequestStatus) CleanMemberStatus(name string) {
	pos, _ := getMemberStatus(rs, name)
	if pos != -1 {
		rs.Members = append(rs.Members[:pos], rs.Members[pos+1:]...)
	}
}

func getMemberStatus(status *OperandRequestStatus, name string) (int, *MemberStatus) {
	for i, m := range status.Members {
		if name == m.Name {
			return i, &m
		}
	}
	return -1, nil
}

func newMemberStatus(name string, operatorPhase olmv1alpha1.ClusterServiceVersionPhase, operandPhase ServicePhase) MemberStatus {
	return MemberStatus{
		Name: name,
		Phase: MemberPhase{
			OperatorPhase: operatorPhase,
			OperandPhase:  operandPhase,
		},
	}
}

func (rs *OperandRequestStatus) SetClusterPhase(p ClusterPhase) {
	rs.Phase = p
}

func init() {
	SchemeBuilder.Register(&OperandRequest{}, &OperandRequestList{})
}
