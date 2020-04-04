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

// OperandBindInfoSpec defines the desired state of OperandBindInfo
type OperandBindInfoSpec struct {
	// The deployed service identifies itself with its operand.
	// This must match the name in the OperandRegistry in the current namespace.
	Operand string `json:"operand"`
	// The registry identifies the name of the name of the OperandRegistry CR from which this operand deployment is being requested.
	Registry string `json:"registry"`
	// +optional
	Description string `json:"description,omitempty"`
	// The bindings section is used to specify information about the access/configuration data that is to be shared.
	// +optional
	Bindings []Binding `json:"bindings,omitempty"`
}

// Binding defines the scope of the operand, and the resources, like Secret and ConfigMap.
type Binding struct {
	// The scope identifier determines whether the referenced information can be shared with requests in other namespaces (pubic) or only in this namespace (private). An OperandBindInfo CR can have at MOST one public scoped binding and one private scope binding.
	// +optional
	Scope scope `json:"scope,omitempty"`
	// The secret field names an existing secret, if any, that has been created and holds information that is to be shared with the adopter.
	// +optional
	Secret string `json:"secret,omitempty"`
	// The configmap field identifies a configmap object, if any, that should be shared with the adopter/requestor
	// +optional
	Configmap string `json:"configmap,omitempty"`
}

// OperandBindInfoStatus defines the observed state of OperandBindInfo
type OperandBindInfoStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandBindInfo is the Schema for the operandbindinfos API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandbindinfos,shortName=opbi,scope=Namespaced
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandBindInfo"
type OperandBindInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandBindInfoSpec   `json:"spec,omitempty"`
	Status OperandBindInfoStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandBindInfoList contains a list of OperandBindInfo
type OperandBindInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandBindInfo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperandBindInfo{}, &OperandBindInfoList{})
}
