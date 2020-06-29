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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BindInfoPhase defines the BindInfo status
type BindInfoPhase string

// BindInfo status
const (
	BindInfoCompleted BindInfoPhase = "Completed"
	BindInfoFailed    BindInfoPhase = "Failed"
	BindInfoInit      BindInfoPhase = "Initialized"
	BindInfoUpdating  BindInfoPhase = "Updating"
	BindInfoWaiting   BindInfoPhase = "Waiting for Secret and/or Configmap from provider"
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
	// Specifies the namespace in which the OperandRegistry reside.
	// The default is the current namespace in which the request is defined.
	// +optional
	RegistryNamespace string `json:"registryNamespace,omitempty"`
	// +optional
	Description string `json:"description,omitempty"`
	// The bindings section is used to specify information about the access/configuration data that is to be shared.
	// +optional
	Bindings map[string]SecretConfigmap `json:"bindings,omitempty"`
}

// SecretConfigmap is a pair of Secret and/or Configmap
type SecretConfigmap struct {
	// The secret field names an existing secret, if any, that has been created and holds information that is to be shared with the adopter.
	// +optional
	Secret string `json:"secret,omitempty"`
	// The configmap field identifies a configmap object, if any, that should be shared with the adopter/requestor
	// +optional
	Configmap string `json:"configmap,omitempty"`
}

// OperandBindInfoStatus defines the observed state of OperandBindInfo
type OperandBindInfoStatus struct {
	// Phase describes the overall phase of OperandBindInfo
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase BindInfoPhase `json:"phase,omitempty"`
	// RequestNamespaces defines the namespaces of OperandRequest
	// +optional
	RequestNamespaces []string `json:"requestNamespaces,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandBindInfo is the Schema for the operandbindinfos API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandbindinfos,shortName=opbi,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=.status.phase,description="Current Phase"
// +kubebuilder:printcolumn:name="Created At",type=string,JSONPath=.metadata.creationTimestamp
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

// InitBindInfoStatus OperandConfig status
func (r *OperandBindInfo) InitBindInfoStatus() bool {
	isInitialized := true
	if r.Status.Phase == "" {
		isInitialized = false
		r.Status.Phase = BindInfoInit
	}
	return isInitialized
}

// SetUpdatingBindInfoPhase sets the Phase status as Updating
func (r *OperandBindInfo) SetUpdatingBindInfoPhase() {
	r.Status.Phase = BindInfoUpdating
}

// GetRegistryKey Set the default value for Request spec
func (r *OperandBindInfo) GetRegistryKey() types.NamespacedName {
	if r.Spec.RegistryNamespace != "" {
		return types.NamespacedName{Namespace: r.Spec.RegistryNamespace, Name: r.Spec.Registry}
	}
	return types.NamespacedName{Namespace: r.Namespace, Name: r.Spec.Registry}
}

func (r *OperandBindInfo) GeneralLabels() map[string]string {
	labels := make(map[string]string)
	registryKey := r.GetRegistryKey()
	labels[registryKey.Namespace+"."+registryKey.Name+"/registry"] = "true"
	return labels
}

// UpdateLabels update the labels for the OperandConfig and OperandRegistry used by this OperandRequest
// Return true if label changed, otherwise return false
func (r *OperandBindInfo) UpdateLabels() bool {
	isUpdated := false
	if r.Labels == nil {
		r.Labels = r.GeneralLabels()
		isUpdated = true
	} else {
		// Remove useless labels
		for label := range r.Labels {
			if strings.HasSuffix(label, "/registry") {
				if _, ok := r.GeneralLabels()[label]; !ok {
					delete(r.Labels, label)
					isUpdated = true
				}
			}
		}
		// Add new label
		for label := range r.GeneralLabels() {
			if _, ok := r.Labels[label]; !ok {
				r.Labels[label] = "true"
				isUpdated = true
			}
		}
	}
	return isUpdated
}
