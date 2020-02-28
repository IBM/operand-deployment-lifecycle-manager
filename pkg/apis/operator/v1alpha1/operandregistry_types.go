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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Channel string `json:"channel,omitempty"`
	// Description of a common service
	Description string `json:"description,omitempty"`
	// Approval mode for emitted InstallPlans
	InstallPlanApproval string `json:"installPlanApproval,omitempty"`
	// State is a flag to enable or disable service
	State string `json:"state,omitempty"`
}

// +kubebuilder:validation:Enum=public;private
type scope string

const (
	ScopePrivate scope = "private"
	ScopePublic  scope = "public"
)

// OperandRegistrySpec defines the desired state of OperandRegistry
// +k8s:openapi-gen=true
type OperandRegistrySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Operators is a list of operator OLM definition
	// +optional
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Operators []Operator `json:"operators,omitempty"`
}

// OperandRegistryStatus defines the observed state of OperandRegistry
// +k8s:openapi-gen=true
type OperandRegistryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// OperatorsStatus defines operator running state
	// +optional
	OperatorsStatus map[string]OperatorPhase `json:"operatorsStatus,omitempty"`
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
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRegistryList contains a list of OperandRegistry
type OperandRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRegistry `json:"items"`
}

// Set the default value for Registry spec
func (r *OperandRegistrySpec) SetDefaults() {
	for i, o := range r.Operators {
		if o.Scope == "" {
			r.Operators[i].Scope = ScopePrivate
		}
	}
}

func init() {
	SchemeBuilder.Register(&OperandRegistry{}, &OperandRegistryList{})
}
