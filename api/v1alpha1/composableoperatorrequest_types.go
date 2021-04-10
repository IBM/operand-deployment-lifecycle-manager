//
// Copyright 2021 IBM Corporation
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

// OperatorRequestPhase is the phase of the ComposableOperatorRequest.
type OperatorRequestPhase string

const (
	ComposableInstalling OperatorRequestPhase = "Installing"
	ComposableUpdating   OperatorRequestPhase = "Updating"
	ComposableFailed     OperatorRequestPhase = "Failed"
	ComposableInit       OperatorRequestPhase = "Initializing"
	ComposableRunning    OperatorRequestPhase = "Running"
	ComposableNone       OperatorRequestPhase = ""
)

// ComposableOperatorRequestSpec defines the desired state of ComposableOperatorRequest
type ComposableOperatorRequestSpec struct {
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Composable Components"
	ComposableComponents []Component `json:"composableComponents"`
}

// Component is used to enable or disable operators.
type Component struct {
	// OperatorName the name of the Operator.
	OperatorName string `json:"name"`
	// Enabled indicates if the operator is enabled
	Enabled bool `json:"enabled"`
}

// ComposableOperatorRequestStatus defines the observed state of ComposableOperatorRequest
type ComposableOperatorRequestStatus struct {
	// Phase is the cluster running phase.
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Phase",xDescriptors="urn:alm:descriptor:io.kubernetes.phase"
	// +optional
	Phase OperatorRequestPhase `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ComposableOperatorRequest is the Schema for the composableoperatorrequests API
type ComposableOperatorRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComposableOperatorRequestSpec   `json:"spec,omitempty"`
	Status ComposableOperatorRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComposableOperatorRequestList contains a list of ComposableOperatorRequest
type ComposableOperatorRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComposableOperatorRequest `json:"items"`
}

//InitRequestStatus initializes OperandRequest status.
func (r *ComposableOperatorRequest) InitRequestStatus() bool {
	isInitialized := true
	if r.Status.Phase == "" {
		isInitialized = false
		r.Status.Phase = ComposableInit
	}
	return isInitialized
}

func init() {
	SchemeBuilder.Register(&ComposableOperatorRequest{}, &ComposableOperatorRequestList{})
}
