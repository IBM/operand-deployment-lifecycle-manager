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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The OperandRequestSpec identifies one or more specific operands (from a specific Registry) that should actually be installed
// +k8s:openapi-gen=true
type OperandRequestSpec struct {
	// Requests defines a list of operands installation
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Requests []Request `json:"requests"`
}

// Request identifies a operand detail
type Request struct {
	// Operands deines a list of the OperandRegistry entry for the operand to be deployed
	Operands []Operand `json:"operands"`
	// Specifies the name in which the OperandRegistry reside.
	Registry string `json:"registry"`
	// Specifies the namespace in which the OperandRegistry reside.
	// The default is the current namespace in which the request is defined.
	// +optional
	RegistryNamespace string `json:"registryNamespace,omitempty"`
	// Description is an optional description for the request
	// +optional
	Description string `json:"description,omitempty"`
}

// Operand defines the name and binding information for one operator
type Operand struct {
	// Name of the operand to be deployed
	Name string `json:"name"`
	// Specify a Binding list
	// +listType=set
	// +optional
	Bindings []Binding `json:"bindings,omitempty"`
}

// Binding defines the scope of the operand, and the resources, like Secret, ConfigMap and ServiceAccount
type Binding struct {
	// A scope indicator, either public or private
	// Valid values are:
	// - "private" (default): deployment only request from the containing names;
	// - "public": deployment can be requested from other namespaces;
	// +optional
	Scope scope `json:"scope,omitempty"`
	// Specifies that name of a secret that should contain information shared
	// from the deployed operand if it shares secrets with requesters.
	// +optional
	Secret string `json:"secret,omitempty"`
	// Specifies the name of a configmap that should contain information shared
	// from the deployed operand if it shares configmaps with requesters.
	// +optional
	ConfigMap string `json:"configMap,omitempty"`
	// Specifies the name of the service account that needs to access the created secret
	// and/or configmap (e.g. the service account being used by running pods),
	// so that ODLM can sure that the access rights are set correctly, if needed.
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
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
	ConditionNotFound ConditionType = "NotFound"

	ClusterPhaseNone     ClusterPhase = "Pending"
	ClusterPhaseCreating ClusterPhase = "Creating"
	ClusterPhaseRunning  ClusterPhase = "Running"
	ClusterPhaseFailed   ClusterPhase = "Failed"

	ResourceTypeSub     ResourceType = "subscription"
	ResourceTypeCsv     ResourceType = "csv"
	ResourceTypeOperand ResourceType = "operand"
)

// Condition represents the current state of the Request Service
// A condition might not show up if it is not happening
type Condition struct {
	// Type of condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another
	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition
	// +optional
	Message string `json:"message,omitempty"`
}

// OperandRequestStatus defines the observed state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestStatus struct {
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
	Phase ClusterPhase `json:"phase,omitempty"`
}

// MemberPhase show the phase of the operator and operator instance
type MemberPhase struct {
	// OperatorPhase show the deploy phase of the operator
	// +optional
	OperatorPhase OperatorPhase `json:"operatorPhase,omitempty"`
	// OperandPhase show the deploy phase of the operator instance
	// +optional
	OperandPhase ServicePhase `json:"operandPhase,omitempty"`
}

// MemberStatus show if the Operator is ready
type MemberStatus struct {
	// The member name are the same as the subscription name
	Name string `json:"name"`
	// The operand phase include None, Creating, Running, Failed
	// +optional
	Phase MemberPhase `json:"phase,omitempty"`
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

// SetCreatingCondition creates a new condition status
func (r *OperandRequest) SetCreatingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionCreating, cs, "Creating "+string(rt), "Creating "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetUpdatingCondition updates a condition status
func (r *OperandRequest) SetUpdatingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionUpdating, cs, "Updating "+string(rt), "Updating "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetDeletingCondition delete a condition status
func (r *OperandRequest) SetDeletingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionDeleting, cs, "Deleting "+string(rt), "Deleting "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetNotFoundCondition not found resource
func (r *OperandRequest) SetNotFoundOperatorFromRegistryCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionNotFound, cs, "Not found "+string(rt), "Not found "+string(rt)+" "+name+" from registry")
	r.setCondition(*c)
}

func (r *OperandRequest) setCondition(c Condition) {
	pos, cp := getCondition(&r.Status, c.Type, c.Message)
	if cp != nil {
		r.Status.Conditions[pos] = c
	} else {
		r.Status.Conditions = append(r.Status.Conditions, c)
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

// SetMemberStatus appends a Member status in the Member status list
func (r *OperandRequest) SetMemberStatus(name string, operatorPhase OperatorPhase, operandPhase ServicePhase) {
	m := newMemberStatus(name, operatorPhase, operandPhase)
	pos, mp := getMemberStatus(&r.Status, name)
	if mp != nil {
		if m.Phase.OperatorPhase != mp.Phase.OperatorPhase {
			r.Status.Members[pos].Phase.OperatorPhase = m.Phase.OperatorPhase
		}
		if m.Phase.OperandPhase != mp.Phase.OperandPhase {
			r.Status.Members[pos].Phase.OperandPhase = m.Phase.OperandPhase
		}
	} else {
		r.Status.Members = append(r.Status.Members, m)
	}
}

// CleanMemberStatus deletes a Member status from the Member status list
func (r *OperandRequest) CleanMemberStatus(name string) {
	pos, _ := getMemberStatus(&r.Status, name)
	if pos != -1 {
		r.Status.Members = append(r.Status.Members[:pos], r.Status.Members[pos+1:]...)
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

func newMemberStatus(name string, operatorPhase OperatorPhase, operandPhase ServicePhase) MemberStatus {
	return MemberStatus{
		Name: name,
		Phase: MemberPhase{
			OperatorPhase: operatorPhase,
			OperandPhase:  operandPhase,
		},
	}
}

// SetClusterPhase sets the current Phase status
func (r *OperandRequest) SetClusterPhase(p ClusterPhase) {
	r.Status.Phase = p
}

func (r *OperandRequest) UpdateClusterPhase() {
	clusterStatusStat := struct {
		creatingNum int
		runningNum  int
		failedNum   int
	}{
		creatingNum: 0,
		runningNum:  0,
		failedNum:   0,
	}

	for _, m := range r.Status.Members {
		switch m.Phase.OperatorPhase {
		case OperatorReady:
			clusterStatusStat.creatingNum++
		case OperatorFailed:
			clusterStatusStat.failedNum++
		case OperatorRunning:
			clusterStatusStat.runningNum++
		default:
		}

		switch m.Phase.OperandPhase {
		case ServiceReady:
			clusterStatusStat.creatingNum++
		case ServiceRunning:
			clusterStatusStat.runningNum++
		case ServiceFailed:
			clusterStatusStat.failedNum++
		default:
		}
	}

	var clusterPhase ClusterPhase
	if clusterStatusStat.failedNum > 0 {
		clusterPhase = ClusterPhaseFailed
	} else if clusterStatusStat.creatingNum > 0 {
		clusterPhase = ClusterPhaseCreating
	} else if clusterStatusStat.runningNum > 0 {
		clusterPhase = ClusterPhaseRunning
	} else {
		clusterPhase = ClusterPhaseNone
	}
	r.SetClusterPhase(clusterPhase)
}

// SetDefaultsRequestSpec Set the default value for Request spec
func (r *OperandRequest) SetDefaultsRequestSpec() {
	for i, req := range r.Spec.Requests {
		if req.RegistryNamespace == "" {
			r.Spec.Requests[i].RegistryNamespace = r.Namespace
		}
	}
}

// SetDefaultRequestStatus set the default OperandRquest status
func (r *OperandRequest) SetDefaultRequestStatus() {
	if r.Status.Phase == "" {
		r.Status.Phase = ClusterPhaseNone
	}
}

func init() {
	SchemeBuilder.Register(&OperandRequest{}, &OperandRequestList{})
}
