//
// Copyright 2022 IBM Corporation
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
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Operator defines the desired state of Operators.
type Operator struct {
	// A unique name for the operator whose operand may be deployed.
	Name string `json:"name"`
	// A scope indicator, either public or private.
	// Valid values are:
	// - "private" (default): deployment only request from the containing names;
	// - "public": deployment can be requested from other namespaces;
	// +optional
	Scope scope `json:"scope,omitempty"`
	// The install mode of an operator, either namespace or cluster.
	// Valid values are:
	// - "namespace" (default): operator is deployed in namespace of OperandRegistry;
	// - "cluster": operator is deployed in "openshift-operators" namespace;
	// +optional
	InstallMode string `json:"installMode,omitempty"`
	// The namespace in which operator CR should be deployed.
	// Also the namespace in which operator should be deployed when InstallMode is empty or set to "namespace".
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Name of a CatalogSource that defines where and how to find the channel.
	SourceName string `json:"sourceName,omitempty"`
	// The Kubernetes namespace where the CatalogSource used is located.
	SourceNamespace string `json:"sourceNamespace,omitempty"`
	// The target namespace of the OperatorGroups.
	TargetNamespaces []string `json:"targetNamespaces,omitempty"`
	// Name of the package that defines the applications.
	PackageName string `json:"packageName"`
	// Name of the channel to track.
	Channel string `json:"channel"`
	// Description of a common service.
	// +optional
	Description string `json:"description,omitempty"`
	// Approval mode for emitted InstallPlans.
	// +optional
	// Valid values are:
	// - "Automatic" (default): operator will be installed automatically;
	// - "Manual": operator installation will be pending until users approve it;
	InstallPlanApproval olmv1alpha1.Approval `json:"installPlanApproval,omitempty"`
	// StartingCSV of the installation.
	// +optional
	StartingCSV string `json:"startingCSV,omitempty"`
	// SubscriptionConfig is used to override operator configuration.
	// +optional
	SubscriptionConfig *olmv1alpha1.SubscriptionConfig `json:"subscriptionConfig,omitempty"`
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

const (
	// InstallModeCluster means install the operator in all namespaces mode.
	InstallModeCluster string = "cluster"
	// InstallModeNamespace means install the operator in one namespace mode.
	InstallModeNamespace string = "namespace"
)

// OperandRegistrySpec defines the desired state of OperandRegistry.
type OperandRegistrySpec struct {
	// Operators is a list of operator OLM definition.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Operators Registry List"
	// +optional
	Operators []Operator `json:"operators,omitempty"`
}

// OperandRegistryStatus defines the observed state of OperandRegistry.
type OperandRegistryStatus struct {
	// Phase describes the overall phase of operators in the OperandRegistry.
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Phase",xDescriptors="urn:alm:descriptor:io.kubernetes.phase"
	// +optional
	Phase RegistryPhase `json:"phase,omitempty"`
	// OperatorsStatus defines operators status and the number of reconcile request.
	// +optional
	OperatorsStatus map[string]OperatorStatus `json:"operatorsStatus,omitempty"`
	// Conditions represents the current state of the Request Service.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions",xDescriptors="urn:alm:descriptor:io.kubernetes.conditions"
	Conditions []Condition `json:"conditions,omitempty"`
}

// OperatorStatus defines operators status and the number of reconcile request.
type OperatorStatus struct {
	// Phase is the state of operator.
	// +optional
	Phase OperatorPhase `json:"phase,omitempty"`
	// ReconcileRequests stores the namespace/name of all the requests.
	// +optional
	ReconcileRequests []ReconcileRequest `json:"reconcileRequests,omitempty"`
}

// ReconcileRequest records the information of the operandRequest.
type ReconcileRequest struct {
	// Name defines the name of request.
	Name string `json:"name"`
	// Namespace defines the namespace of request.
	Namespace string `json:"namespace"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandregistries,shortName=opreg,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=.status.phase,description="Current Phase"
// +kubebuilder:printcolumn:name="Created At",type=string,JSONPath=.metadata.creationTimestamp
// +operator-sdk:csv:customresourcedefinitions:displayName="OperandRegistry"

// OperandRegistry is the Schema for the operandregistries API.
type OperandRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandRegistrySpec   `json:"spec,omitempty"`
	Status OperandRegistryStatus `json:"status,omitempty"`
}

// RegistryPhase defines the operator status.
type RegistryPhase string

// Registry phase
const (
	// RegistryFinalizer is the name for the finalizer to allow for deletion
	// when an OperandRegistry is deleted.
	RegistryFinalizer = "finalizer.registry.ibm.com"

	RegistryReady    RegistryPhase = "Ready for Deployment"
	RegistryRunning  RegistryPhase = "Running"
	RegistryPending  RegistryPhase = "Pending"
	RegistryUpdating RegistryPhase = "Updating"
	RegistryFailed   RegistryPhase = "Failed"
	RegistryWaiting  RegistryPhase = "Waiting for CatalogSource being ready"
	RegistryInit     RegistryPhase = "Initialized"
	RegistryNone     RegistryPhase = ""
)

// +kubebuilder:object:root=true

// OperandRegistryList contains a list of OperandRegistry.
type OperandRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRegistry `json:"items"`
}

// GetReconcileRequest gets the position of request from OperandRegistry status.
func (r *OperandRegistry) GetReconcileRequest(name string, reconcileRequest reconcile.Request) int {
	s := r.Status.OperatorsStatus[name]
	for pos, r := range s.ReconcileRequests {
		if r.Name == reconcileRequest.Name && r.Namespace == reconcileRequest.Namespace {
			return pos
		}
	}
	return -1
}

// SetOperatorStatus sets the operator status in the OperandRegistry.
func (r *OperandRegistry) SetOperatorStatus(name string, phase OperatorPhase, request reconcile.Request) {
	s := r.Status.OperatorsStatus[name]
	if s.Phase != phase {
		s.Phase = phase
	}

	if pos := r.GetReconcileRequest(name, request); pos == -1 {
		s.ReconcileRequests = append(s.ReconcileRequests, ReconcileRequest{Name: request.Name, Namespace: request.Namespace})
	}
	r.Status.OperatorsStatus[name] = s
}

// GetOperator obtains the operator definition with the operand name.
func (r *OperandRegistry) GetOperator(operandName string) *Operator {
	for _, o := range r.Spec.Operators {
		if o.Name == operandName {
			return &o
		}
	}
	return nil
}

// GetAllReconcileRequest gets all the ReconcileRequest from OperandRegistry status.
func (r *OperandRegistry) GetAllReconcileRequest() []reconcile.Request {
	maprrs := make(map[string]reconcile.Request)
	for _, os := range r.Status.OperatorsStatus {
		for _, rr := range os.ReconcileRequests {
			key := rr.Namespace + "/" + rr.Name
			maprrs[key] = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      rr.Name,
				Namespace: rr.Namespace,
			}}
		}
	}

	rrs := []reconcile.Request{}
	for _, rr := range maprrs {
		rrs = append(rrs, rr)
	}
	return rrs
}

// SetReadyCondition creates a Condition to claim Ready.
func (r *OperandRegistry) SetReadyCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionReady, cs, string(rt)+" is ready", string(rt)+" "+name+" is ready")
	r.setCondition(*c)
}

// SetNotFoundCondition creates a Condition to claim NotFound.
func (r *OperandRegistry) SetNotFoundCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionNotFound, cs, "Not found "+string(rt), "Not found "+string(rt)+" "+name)
	r.setCondition(*c)
}

func (r *OperandRegistry) setCondition(c Condition) {
	pos, cp := getCondition(&r.Status.Conditions, c.Type, c.Message)
	if cp != nil {
		r.Status.Conditions[pos] = c
	} else {
		r.Status.Conditions = append(r.Status.Conditions, c)
	}
}

// UpdateRegistryPhase sets the current Phase status.
func (r *OperandRegistry) UpdateRegistryPhase(phase RegistryPhase) {
	r.Status.Phase = phase
}

// RemoveFinalizer removes the operator source finalizer from the
// OperatorSource ObjectMeta.
func (r *OperandRegistry) RemoveFinalizer() bool {
	return RemoveFinalizer(&r.ObjectMeta, RegistryFinalizer)
}

// EnsureFinalizer ensures that the operator source finalizer is included
// in the ObjectMeta.Finalizer slice. If it already exists, no state change occurs.
// If it doesn't, the finalizer is appended to the slice.
func (r *OperandRegistry) EnsureFinalizer() bool {
	return EnsureFinalizer(&r.ObjectMeta, RegistryFinalizer)
}

func init() {
	SchemeBuilder.Register(&OperandRegistry{}, &OperandRegistryList{})
}
