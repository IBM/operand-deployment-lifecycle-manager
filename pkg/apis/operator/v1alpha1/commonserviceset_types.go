package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CommonServiceSetSpec defines the desired state of CommonServiceSet
// +k8s:openapi-gen=true
type CommonServiceSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Services []SetService `json:"services"`
}

type SetService struct {
	Name        string `json:"name"`
	Channel     string `json:"channel,omitempty"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
}

type ConditionType string
type ClusterPhase string

const (
	ConditionInstall ConditionType = "Install"
	ConditionUpdate  ConditionType = "Update"
	ConditionDelete  ConditionType = "Delete"

	ClusterPhaseNone     ClusterPhase = ""
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

// CommonServiceSetStatus defines the observed state of CommonServiceSet
// +k8s:openapi-gen=true
type CommonServiceSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Conditions represents the current state of the Set Service
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operators of the set
	// +optional
	Members MembersStatus `json:"members,omitempty"`
	// Phase is the cluster running phase
	Phase ClusterPhase `json:"phase"`
}
type MembersStatus struct {
	// Ready are the operator members that are ready
	// The member names are the same as the subscription
	Ready []string `json:"ready,omitempty"`
	// Unready are the etcd members not ready
	Unready []string `json:"unready,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommonServiceSet is the Schema for the commonservicesets API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=commonservicesets,shortName=css,scope=Namespaced
type CommonServiceSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommonServiceSetSpec   `json:"spec,omitempty"`
	Status CommonServiceSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommonServiceSetList contains a list of CommonServiceSet
type CommonServiceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CommonServiceSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CommonServiceSet{}, &CommonServiceSetList{})
}
