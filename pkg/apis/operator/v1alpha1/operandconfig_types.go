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
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OperandConfigSpec defines the desired state of OperandConfig
// +k8s:openapi-gen=true
type OperandConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Services is a list of configuration of service
	// +optional
	// +listType=set
	Services []ConfigService `json:"services,omitempty"`
}

// ConfigService defines the configuration of the service
type ConfigService struct {
	// Name is the subscription name
	Name string `json:"name"`
	// Spec is the map of configuration of custome service
	Spec map[string]runtime.RawExtension `json:"spec"`
	// State is a flag to enable or disable service
	State string `json:"state,omitempty"`
}

// OperandConfigStatus defines the observed state of OperandConfig
// +k8s:openapi-gen=true
type OperandConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// ServiceStatus defines all the status of a operator
	// +optional
	ServiceStatus map[string]CrStatus `json:"serviceStatus,omitempty"`
}

// CrStatus defines the status of the custom resource
type CrStatus struct {
	CrStatus map[string]ServicePhase `json:"customeResourceStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandConfig is the Schema for the operandconfigs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandconfigs,shortName=oc,scope=Namespaced
type OperandConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandConfigSpec   `json:"spec,omitempty"`
	Status OperandConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandConfigList contains a list of OperandConfig
type OperandConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandConfig `json:"items"`
}

// ServicePhase defines the service status
type ServicePhase string

// Service status
const (
	ServiceReady   ServicePhase = "Ready for Deployment"
	ServiceRunning ServicePhase = "Running"
	ServiceFailed  ServicePhase = "Failed"
)

func init() {
	SchemeBuilder.Register(&OperandConfig{}, &OperandConfigList{})
}
