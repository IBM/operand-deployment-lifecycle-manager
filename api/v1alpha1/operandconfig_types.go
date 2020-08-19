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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OperandConfigSpec defines the desired state of OperandConfig
type OperandConfigSpec struct {
	// Services is a list of configuration of service
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Services []ConfigService `json:"services,omitempty"`
}

// ConfigService defines the configuration of the service
type ConfigService struct {
	// Name is the subscription name
	Name string `json:"name"`
	// Spec is the configuration map of custom resource
	Spec map[string]runtime.RawExtension `json:"spec"`
	// State is a flag to enable or disable service
	State string `json:"state,omitempty"`
}

// OperandConfigStatus defines the observed state of OperandConfig
type OperandConfigStatus struct {
	// Phase describes the overall phase of operands in the OperandConfig
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase ServicePhase `json:"phase,omitempty"`
	// ServiceStatus defines all the status of a operator
	// +optional
	ServiceStatus map[string]CrStatus `json:"serviceStatus,omitempty"`
}

// CrStatus defines the status of the custom resource
type CrStatus struct {
	CrStatus map[string]ServicePhase `json:"customResourceStatus,omitempty"`
}

// OperandConfig is the Schema for the operandconfigs API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandconfigs,shortName=opcon,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=.status.phase,description="Current Phase"
// +kubebuilder:printcolumn:name="Created At",type=string,JSONPath=.metadata.creationTimestamp
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandConfig"
type OperandConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandConfigSpec   `json:"spec,omitempty"`
	Status OperandConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

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
	// ConfigFinalizer is the name for the finalizer to allow for deletion
	// reconciliation when an OperandConfig is deleted.
	ConfigFinalizer = "finalizer.config.ibm.com"

	ServiceNotReady ServicePhase = "NotReady"
	ServiceRunning  ServicePhase = "Running"
	ServiceFailed   ServicePhase = "Failed"
	ServiceInit     ServicePhase = "Initialized"
	ServiceNone     ServicePhase = ""
)

// GetService obtain the service definition with the operand name
func (r *OperandConfig) GetService(operandName string) *ConfigService {
	for _, s := range r.Spec.Services {
		if s.Name == operandName {
			return &s
		}
	}
	return nil
}

// InitConfigStatus OperandConfig status
func (r *OperandConfig) InitConfigStatus() bool {
	isInitialized := true
	if r.Status.Phase == "" {
		isInitialized = false
		r.Status.Phase = ServiceInit
	}
	return isInitialized
}

//InitConfigServiceStatus service status in the OperandConfig instance
func (r *OperandConfig) InitConfigServiceStatus() {
	r.Status.ServiceStatus = make(map[string]CrStatus)

	for _, operator := range r.Spec.Services {
		r.Status.ServiceStatus[operator.Name] = CrStatus{CrStatus: make(map[string]ServicePhase)}
		for service := range operator.Spec {
			r.Status.ServiceStatus[operator.Name].CrStatus[service] = ServiceInit
		}
	}
	r.UpdateOperandPhase()
}

// UpdateOperandPhase sets the current Phase status
func (r *OperandConfig) UpdateOperandPhase() {
	operandStatusStat := struct {
		notReadyNum int
		runningNum  int
		failedNum   int
	}{
		notReadyNum: 0,
		runningNum:  0,
		failedNum:   0,
	}
	for _, operator := range r.Status.ServiceStatus {
		for _, service := range operator.CrStatus {
			switch service {
			case ServiceNotReady:
				operandStatusStat.notReadyNum++
			case ServiceRunning:
				operandStatusStat.runningNum++
			case ServiceFailed:
				operandStatusStat.failedNum++
			}
		}
	}
	if operandStatusStat.failedNum > 0 {
		r.Status.Phase = ServiceFailed
	} else if operandStatusStat.notReadyNum > 0 {
		r.Status.Phase = ServiceNotReady
	} else if operandStatusStat.runningNum > 0 {
		r.Status.Phase = ServiceRunning
	} else {
		r.Status.Phase = ServiceInit
	}
}

// RemoveFinalizer removes the operator source finalizer from the
// OperatorSource ObjectMeta.
func (r *OperandConfig) RemoveFinalizer() bool {
	return RemoveFinalizer(&r.ObjectMeta, ConfigFinalizer)
}

// EnsureFinalizer ensures that the operator source finalizer is included
// in the ObjectMeta.Finalizer slice. If it already exists, no state change occurs.
// If it doesn't, the finalizer is appended to the slice.
func (r *OperandConfig) EnsureFinalizer() bool {
	return EnsureFinalizer(&r.ObjectMeta, ConfigFinalizer)
}

// CheckPhase checks if the OperandConfig phase are running
func (r *OperandConfig) CheckPhase() bool {
	return r.Status.Phase == ServiceRunning
}

func init() {
	SchemeBuilder.Register(&OperandConfig{}, &OperandConfigList{})
}
