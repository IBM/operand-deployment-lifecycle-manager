package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CommonServiceConfigSpec defines the desired state of CommonServiceConfig
// +k8s:openapi-gen=true
type CommonServiceConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Services is a list of configuration of service
	// +optional
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

// CommonServiceConfigStatus defines the observed state of CommonServiceConfig
// +k8s:openapi-gen=true
type CommonServiceConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// ServiceStatus defines all the status of a operator
	// +optional
	ServiceStatus map[string]*CrStatus `json:"serviceStatus,omitempty"`
}

// CrStatus defines the status of the custom resource
type CrStatus struct {
	CrStatus map[string]ServicePhase `json:"customeResourceStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommonServiceConfig is the Schema for the commonserviceconfigs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=commonserviceconfigs,shortName=csc,scope=Namespaced
type CommonServiceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status CommonServiceConfigStatus `json:"status,omitempty"`
	Spec   CommonServiceConfigSpec   `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommonServiceConfigList contains a list of CommonServiceConfig
type CommonServiceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CommonServiceConfig `json:"items"`
}

// ServicePhase defines the service status
type ServicePhase string

// Service status
const (
	ServiceRunning ServicePhase = "Running"
	ServiceFailed  ServicePhase = "Failed"
)

func init() {
	SchemeBuilder.Register(&CommonServiceConfig{}, &CommonServiceConfigList{})
}
