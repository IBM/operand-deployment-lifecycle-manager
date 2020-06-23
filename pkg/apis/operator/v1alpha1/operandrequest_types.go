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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// The OperandRequestSpec identifies one or more specific operands (from a specific Registry) that should actually be installed
// +k8s:openapi-gen=true
type OperandRequestSpec struct {
	// Requests defines a list of operands installation
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Requests []Request `json:"requests"`
}

// Request identifies a operand detail
type Request struct {
	// Operands deines a list of the OperandRegistry entry for the operand to be deployed
	Operands []Operand `json:"operands"`
	// Specifies the name in which the OperandRegistry reside.
	Registry string `json:"registry"`
	// Specifies the namespace in which the OperandRegistry reside.
	// The default is the current namespace in which the request is defined.
	// +optional
	RegistryNamespace string `json:"registryNamespace,omitempty"`
	// Description is an optional description for the request
	// +optional
	Description string `json:"description,omitempty"`
}

// Operand defines the name and binding information for one operator
type Operand struct {
	// Name of the operand to be deployed
	Name string `json:"name"`
	// The bindings section is used to specify names of secret and/or configmap.
	// +optional
	Bindings map[string]SecretConfigmap `json:"bindings,omitempty"`
}

// ConditionType is the condition of a service
type ConditionType string

// ClusterPhase is the phase of the installation
type ClusterPhase string

// ResourceType is the type of condition use
type ResourceType string

// OperatorPhase defines the operator status
type OperatorPhase string

// Constants are used for state
const (
	// RequestFinalizer is the name for the finalizer to allow for deletion
	// reconciliation when an OperandRequest is deleted.
	RequestFinalizer = "finalizer.request.ibm.com"

	ConditionCreating   ConditionType = "Creating"
	ConditionUpdating   ConditionType = "Updating"
	ConditionDeleting   ConditionType = "Deleting"
	ConditionNotFound   ConditionType = "NotFound"
	ConditionOutofScope ConditionType = "OutofScope"
	ConditionReady      ConditionType = "Ready"

	OperatorReady      OperatorPhase = "Ready for Deployment"
	OperatorRunning    OperatorPhase = "Running"
	OperatorInstalling OperatorPhase = "Installing"
	OperatorUpdating   OperatorPhase = "Updating"
	OperatorFailed     OperatorPhase = "Failed"
	OperatorInit       OperatorPhase = "Initialized"
	OperatorNone       OperatorPhase = ""

	ClusterPhaseNone       ClusterPhase = "Pending"
	ClusterPhaseCreating   ClusterPhase = "Creating"
	ClusterPhaseInstalling ClusterPhase = "Installing"
	ClusterPhaseUpdating   ClusterPhase = "Updating"
	ClusterPhaseRunning    ClusterPhase = "Running"
	ClusterPhaseFailed     ClusterPhase = "Failed"

	ResourceTypeCatalogSource ResourceType = "catalogsource"
	ResourceTypeSub           ResourceType = "subscription"
	ResourceTypeCsv           ResourceType = "csv"
	ResourceTypeOperator      ResourceType = "operator"
	ResourceTypeOperand       ResourceType = "operands"
)

// Condition represents the current state of the Request Service
// A condition might not show up if it is not happening
type Condition struct {
	// Type of condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another
	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition
	// +optional
	Message string `json:"message,omitempty"`
}

// OperandRequestStatus defines the observed state of OperandRequest
// +k8s:openapi-gen=true
type OperandRequestStatus struct {
	// Conditions represents the current state of the Request Service
	// +optional
	// +listType=set
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	Conditions []Condition `json:"conditions,omitempty"`
	// Members represnets the current operand status of the set
	// +optional
	// +listType=set
	Members []MemberStatus `json:"members,omitempty"`
	// Phase is the cluster running phase
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +optional
	Phase ClusterPhase `json:"phase,omitempty"`
}

// MemberPhase show the phase of the operator and operator instance
type MemberPhase struct {
	// OperatorPhase show the deploy phase of the operator
	// +optional
	OperatorPhase OperatorPhase `json:"operatorPhase,omitempty"`
	// OperandPhase show the deploy phase of the operator instance
	// +optional
	OperandPhase ServicePhase `json:"operandPhase,omitempty"`
}

// MemberStatus show if the Operator is ready
type MemberStatus struct {
	// The member name are the same as the subscription name
	Name string `json:"name"`
	// The operand phase include None, Creating, Running, Failed
	// +optional
	Phase MemberPhase `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequest is the Schema for the operandrequests API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=operandrequests,shortName=opreq,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=.status.phase,description="Current Phase"
// +kubebuilder:printcolumn:name="Created At",type=string,JSONPath=.metadata.creationTimestamp
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="OperandRequest"
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Namespace,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Deployment,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`ReplicaSet,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Service,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Pod,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Configmap,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Installplan,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Catalogsource,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Clusterserviceversion,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Operatorgroup,v1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Subscription,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Operandconfig,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Operandrequest,v1alpha1,""`
// +operator-sdk:gen-csv:customresourcedefinitions.resources=`Operandregistry,v1alpha1,""`
type OperandRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperandRequestSpec   `json:"spec,omitempty"`
	Status OperandRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OperandRequestList contains a list of OperandRequest
type OperandRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OperandRequest `json:"items"`
}

// SetCreatingCondition creates a new condition status
func (r *OperandRequest) SetCreatingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionCreating, cs, "Creating "+string(rt), "Creating "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetUpdatingCondition creates an updating condition status
func (r *OperandRequest) SetUpdatingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionUpdating, cs, "Updating "+string(rt), "Updating "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetDeletingCondition creates a deleting condition status
func (r *OperandRequest) SetDeletingCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionDeleting, cs, "Deleting "+string(rt), "Deleting "+string(rt)+" "+name)
	r.setCondition(*c)
}

// SetNotFoundOperatorFromRegistryCondition creates a NotFoundCondition
func (r *OperandRequest) SetNotFoundOperatorFromRegistryCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionNotFound, cs, "Not found "+string(rt), "Not found "+string(rt)+" "+name+" from registry")
	r.setCondition(*c)
}

// SetOutofScopeCondition creates a NotFoundCondition
func (r *OperandRequest) SetOutofScopeCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := newCondition(ConditionOutofScope, cs, string(rt)+" "+name+" is a private operator", string(rt)+" "+name+" is a private operator. It can only be request within the OperandRegistry namespace")
	r.setCondition(*c)
}

// SetReadyCondition creates a Condition to claim Ready
func (r *OperandRequest) SetReadyCondition(name string, rt ResourceType, cs corev1.ConditionStatus) {
	c := &Condition{}
	if rt == ResourceTypeOperator {
		c = newCondition(ConditionReady, cs, string(rt)+" is ready", string(rt)+" "+name+" is ready")
	} else if rt == ResourceTypeOperand {
		c = newCondition(ConditionReady, cs, string(rt)+" are created", string(rt)+" from "+name+" are created")
	}
	r.setCondition(*c)
}

func (r *OperandRequest) setCondition(c Condition) {
	pos, cp := getCondition(&r.Status.Conditions, c.Type, c.Message)
	if cp != nil {
		r.Status.Conditions[pos] = c
	} else {
		r.Status.Conditions = append(r.Status.Conditions, c)
	}
}

func getCondition(conds *[]Condition, t ConditionType, msg string) (int, *Condition) {
	for i, c := range *conds {
		if t == c.Type && msg == c.Message {
			return i, &c
		}
	}
	return -1, nil
}

func newCondition(condType ConditionType, status corev1.ConditionStatus, reason, message string) *Condition {
	now := time.Now().Format(time.RFC3339)
	return &Condition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

// SetMemberStatus appends a Member status in the Member status list
func (r *OperandRequest) SetMemberStatus(name string, operatorPhase OperatorPhase, operandPhase ServicePhase) {
	pos, m := getMemberStatus(&r.Status, name)
	if m != nil {
		if operatorPhase != "" && operatorPhase != m.Phase.OperatorPhase {
			r.Status.Members[pos].Phase.OperatorPhase = operatorPhase
			r.setOperatorReadyCondition(m, name)
		}
		if operandPhase != "" && operandPhase != m.Phase.OperandPhase {
			r.Status.Members[pos].Phase.OperandPhase = operandPhase
			r.setOperandReadyCondition(m, name)
		}
	} else {
		newM := newMemberStatus(name, operatorPhase, operandPhase)
		r.Status.Members = append(r.Status.Members, newM)
		r.setOperatorReadyCondition(&newM, name)
	}
}

func (r *OperandRequest) setOperatorReadyCondition(m *MemberStatus, name string) {
	if m.Phase.OperatorPhase == OperatorRunning {
		r.SetReadyCondition(name, ResourceTypeOperator, corev1.ConditionTrue)
	} else {
		r.SetReadyCondition(name, ResourceTypeOperator, corev1.ConditionFalse)
	}
}

func (r *OperandRequest) setOperandReadyCondition(m *MemberStatus, name string) {
	if m.Phase.OperandPhase == ServiceRunning {
		r.SetReadyCondition(name, ResourceTypeOperand, corev1.ConditionTrue)
	} else {
		r.SetReadyCondition(name, ResourceTypeOperand, corev1.ConditionFalse)
	}
}

// FreshMemberStatus cleanup Member status from the Member status list
func (r *OperandRequest) FreshMemberStatus() {
	for index, m := range r.Status.Members {
		if !r.getOperand(m.Name) {
			r.Status.Members = append(r.Status.Members[:index], r.Status.Members[index+1:]...)
		}
	}
}

func (r *OperandRequest) getOperand(name string) bool {
	for _, req := range r.Spec.Requests {
		for _, operand := range req.Operands {
			if name == operand.Name {
				return true
			}
		}
	}
	return false
}

func getMemberStatus(status *OperandRequestStatus, name string) (int, *MemberStatus) {
	for i, m := range status.Members {
		if name == m.Name {
			return i, &m
		}
	}
	return -1, nil
}

func newMemberStatus(name string, operatorPhase OperatorPhase, operandPhase ServicePhase) MemberStatus {
	return MemberStatus{
		Name: name,
		Phase: MemberPhase{
			OperatorPhase: operatorPhase,
			OperandPhase:  operandPhase,
		},
	}
}

// SetClusterPhase sets the current Phase status
func (r *OperandRequest) SetClusterPhase(p ClusterPhase) {
	r.Status.Phase = p
}

// UpdateClusterPhase will collect the phase of all the operators and operands.
// Then summarize the cluster phase of the OperandRequest.
func (r *OperandRequest) UpdateClusterPhase() {
	clusterStatusStat := struct {
		creatingNum   int
		runningNum    int
		installingNum int
		failedNum     int
	}{
		creatingNum:   0,
		runningNum:    0,
		installingNum: 0,
		failedNum:     0,
	}

	for _, m := range r.Status.Members {
		switch m.Phase.OperatorPhase {
		case OperatorReady:
			clusterStatusStat.creatingNum++
		case OperatorFailed:
			clusterStatusStat.failedNum++
		case OperatorRunning:
			clusterStatusStat.runningNum++
		case OperatorInstalling:
			clusterStatusStat.installingNum++
		default:
		}

		switch m.Phase.OperandPhase {
		case ServiceReady:
			clusterStatusStat.creatingNum++
		case ServiceRunning:
			clusterStatusStat.runningNum++
		case ServiceFailed:
			clusterStatusStat.failedNum++
		default:
		}
	}

	var clusterPhase ClusterPhase
	if clusterStatusStat.failedNum > 0 {
		clusterPhase = ClusterPhaseFailed
	} else if clusterStatusStat.installingNum > 0 {
		clusterPhase = ClusterPhaseInstalling
	} else if clusterStatusStat.creatingNum > 0 {
		clusterPhase = ClusterPhaseCreating
	} else if clusterStatusStat.runningNum > 0 {
		clusterPhase = ClusterPhaseRunning
	} else {
		clusterPhase = ClusterPhaseNone
	}
	r.SetClusterPhase(clusterPhase)
}

// GetRegistryKey Set the default value for Request spec
func (r *OperandRequest) GetRegistryKey(req Request) types.NamespacedName {
	regName := req.Registry
	regNs := req.RegistryNamespace
	if regNs == "" {
		regNs = r.Namespace
	}
	return types.NamespacedName{Namespace: regNs, Name: regName}
}

//InitConfigStatus OperandConfig status
func (r *OperandRequest) InitRequestStatus() bool {
	isInitialized := true
	if r.Status.Phase == "" {
		isInitialized = false
		r.Status.Phase = ClusterPhaseNone
	}
	return isInitialized
}

func (r *OperandRequest) GeneralLabels() map[string]string {
	labels := make(map[string]string)
	for _, req := range r.Spec.Requests {
		registryKey := r.GetRegistryKey(req)
		labels[registryKey.Namespace+"."+registryKey.Name+"/registry"] = "true"
		labels[registryKey.Namespace+"."+registryKey.Name+"/config"] = "true"
	}
	return labels
}

// UpdateLabels update the labels for the OperandConfig and OperandRegistry used by this OperandRequest
// Return true if label changed, otherwise return false
func (r *OperandRequest) UpdateLabels() bool {
	isUpdated := false
	if r.Labels == nil {
		r.Labels = r.GeneralLabels()
		isUpdated = true
	} else {
		// Remove useless labels
		for label := range r.Labels {
			if strings.HasSuffix(label, "/registry") || strings.HasSuffix(label, "/config") {
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

// GetAllRegistryReconcileRequest gets all the Registry ReconcileRequest
func (r *OperandRequest) GetAllRegistryReconcileRequest() []reconcile.Request {
	rrs := []reconcile.Request{}
	for _, req := range r.Spec.Requests {
		rrs = append(rrs, reconcile.Request{NamespacedName: r.GetRegistryKey(req)})
	}
	return rrs
}

func init() {
	SchemeBuilder.Register(&OperandRequest{}, &OperandRequestList{})
}

// RemoveFinalizer removes the operator source finalizer from the
// OperatorSource ObjectMeta.
func (r *OperandRequest) RemoveFinalizer() bool {
	return RemoveFinalizer(&r.ObjectMeta, RequestFinalizer)
}

// EnsureFinalizer ensures that the operator source finalizer is included
// in the ObjectMeta.Finalizer slice. If it already exists, no state change occurs.
// If it doesn't, the finalizer is appended to the slice.
func (r *OperandRequest) EnsureFinalizer() bool {
	return EnsureFinalizer(&r.ObjectMeta, RequestFinalizer)
}
