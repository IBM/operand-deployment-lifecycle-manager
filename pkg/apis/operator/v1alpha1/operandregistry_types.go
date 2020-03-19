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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Operator defines the desired state of Operators
type Operator struct {
	// A unique name for the operator whose operand may be deployed
	Name string `json:"name"`
	// A scope indicator, either public or private
	// Valid values are:
	// - "private" (default): deployment only request from the containing names;
	// - "public": deployment can be requested from other namespaces;
	// +optional
	Scope scope `json:"scope,omitempty"`
	// The namespace in which operator's operand should be deployed
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Name of a CatalogSource that defines where and how to find the channel
	SourceName string `json:"sourceName"`
	// The Kubernetes namespace where the CatalogSource used is located
	SourceNamespace string `json:"sourceNamespace"`
	// The target namespace of the OperatorGroup
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`
	// Name of the package that defines the application
	PackageName string `json:"packageName"`
	// Name of the channel to track
	Channel string `json:"channel"`
	// Description of a common service
	// +optional
	Description string `json:"description,omitempty"`
	// Approval mode for emitted InstallPlans
	// +optional
	InstallPlanApproval string `json:"installPlanApproval,omitempty"`
}

// +kubebuilder:validation:Enum=public;private
type scope string

const (
	//ScopePrivate means the operand resource can only
	//be used within the namespace.
	ScopePrivate scope = "private"
	//ScopePublic means the operand resource can only
	//be used in the cluster.
	ScopePublic scope = "public"
)

// OperandRegistrySpec defines the desired state of OperandRegistry
// +k8s:openapi-gen=true
type OperandRegistrySpec struct {
	// Operators is a list of operator OLM definition
	// +optional
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Operators []Operator `json:"operators,omitempty"`
}

// OperandRegistryStatus defines the observed state of OperandRegistry
// +k8s:openapi-gen=true
type OperandRegistryStatus struct {
	// Phase describes the overall phase of operators in the OperandRegistry
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase OperatorPhase `json:"phase,omitempty"`
	// OperatorsStatus defines operators status and the number of reconcile request
	// +listType=set
	// +optional
	OperatorsStatus map[string]OperatorStatus `json:"operatorsStatus,omitempty"`
}

// OperatorStatus defines operators status and the number of reconcile request
type OperatorStatus struct {
	// Phase is the state of operator
	// +optional
	Phase OperatorPhase `json:"phase,omitempty"`
	// RecondileRequests store the namespace/name of all the requests
	// +optional
	ReconcileRequests []ReconcileRequest `json:"reconcileRequests,omitempty"`
}

// ReconcileRequest records the information of the operandRequest
type ReconcileRequest struct {
	// Name defines the name of request
	Name string `json:"name"`
	// Namespace defines the namespace of request
	Namespace string `json:"namespace"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRegistry is the Schema for the operandregistries API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandregistries,shortName=opreg,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandRegistry"
type OperandRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandRegistrySpec   `json:"spec,omitempty"`
	Status OperandRegistryStatus `json:"status,omitempty"`
}

// OperatorPhase defines the operator status
type OperatorPhase string

// Operator status
const (
	OperatorReady   OperatorPhase = "Ready for Deployment"
	OperatorRunning OperatorPhase = "Running"
	OperatorFailed  OperatorPhase = "Failed"
	OperatorInit    OperatorPhase = "Initialized"
	OperatorNone    OperatorPhase = ""
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRegistryList contains a list of OperandRegistry
type OperandRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRegistry `json:"items"`
}

// SetDefaultsRegistry Set the default value for Registry spec
func (r *OperandRegistry) SetDefaultsRegistry() {
	for i, o := range r.Spec.Operators {
		if o.Scope == "" {
			r.Spec.Operators[i].Scope = ScopePrivate
		}
	}
}

// InitRegistryStatus Init Phase in the OperandRegistry status
func (r *OperandRegistry) InitRegistryStatus() {
	if (reflect.DeepEqual(r.Status, OperandRegistryStatus{})) {
		r.Status.Phase = OperatorInit
	}
}

// InitRegistryOperatorStatus Init Operators status in the OperandRegistry instance
func (r *OperandRegistry) InitRegistryOperatorStatus() {
	r.Status.OperatorsStatus = make(map[string]OperatorStatus)
	for _, opt := range r.Spec.Operators {
		opratorStatus := OperatorStatus{
			Phase: OperatorReady,
		}
		r.Status.OperatorsStatus[opt.Name] = opratorStatus
	}
	r.UpdateOperatorPhase()
}

// GetReconcileRequest gets the position of request from OperandRegistry status
func (r *OperandRegistry) GetReconcileRequest(name string, reconcileRequest reconcile.Request) int {
	s := r.Status.OperatorsStatus[name]
	for pos, r := range s.ReconcileRequests {
		if r.Name == reconcileRequest.Name && r.Namespace == reconcileRequest.Namespace {
			return pos
		}
	}
	return -1
}

// SetOperatorStatus sets the operator status in the operandRegistry
func (r *OperandRegistry) SetOperatorStatus(name string, phase OperatorPhase, request reconcile.Request) {
	s := r.Status.OperatorsStatus[name]
	if s.Phase != phase {
		s.Phase = phase
	}

	if pos := r.GetReconcileRequest(name, request); pos == -1 {
		s.ReconcileRequests = append(s.ReconcileRequests, ReconcileRequest{Name: request.Name, Namespace: request.Namespace})
	}
	r.Status.OperatorsStatus[name] = s
	r.UpdateOperatorPhase()
}

// CleanOperatorStatus remove the operator status in the operandRegistry
func (r *OperandRegistry) CleanOperatorStatus(name string, request reconcile.Request) {
	s := r.Status.OperatorsStatus[name]
	if pos := r.GetReconcileRequest(name, request); pos != -1 {
		s.ReconcileRequests = append(s.ReconcileRequests[:pos], s.ReconcileRequests[pos+1:]...)
	}
	if len(s.ReconcileRequests) == 0 {
		s.Phase = OperatorReady
	}
	r.Status.OperatorsStatus[name] = s
	r.UpdateOperatorPhase()
}

func init() {
	SchemeBuilder.Register(&OperandRegistry{}, &OperandRegistryList{})
}

// UpdateOperatorPhase sets the current Phase status
func (r *OperandRegistry) UpdateOperatorPhase() {
	operatorStatusStat := struct {
		readyNum   int
		runningNum int
		failedNum  int
	}{
		readyNum:   0,
		runningNum: 0,
		failedNum:  0,
	}
	for _, operator := range r.Status.OperatorsStatus {
		switch operator.Phase {
		case OperatorReady:
			operatorStatusStat.readyNum++
		case OperatorRunning:
			operatorStatusStat.runningNum++
		case OperatorFailed:
			operatorStatusStat.failedNum++
		}
	}
	if operatorStatusStat.failedNum > 0 {
		r.Status.Phase = OperatorFailed
	} else if operatorStatusStat.runningNum > 0 {
		r.Status.Phase = OperatorRunning
	} else if operatorStatusStat.readyNum > 0 {
		r.Status.Phase = OperatorReady
	} else {
		r.Status.Phase = OperatorNone
	}
}
