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

package deploy

import (
	"context"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
)

type ODLMManager struct {
	client.Client
	client.Reader
	*rest.Config
}

// NewODLMManager is the way to create a Manager struct
func NewODLMManager(mgr manager.Manager) *ODLMManager {
	return &ODLMManager{
		Client: mgr.GetClient(),
		Reader: mgr.GetAPIReader(),
		Config: mgr.GetConfig(),
	}
}

// FetchOperandRegistry fetch the OperandRegistry instance with default value
func (m *ODLMManager) FetchOperandRegistry(key types.NamespacedName) (*apiv1alpha1.OperandRegistry, error) {
	reg := &apiv1alpha1.OperandRegistry{}
	if err := m.Reader.Get(context.TODO(), key, reg); err != nil {
		return nil, err
	}
	for i, o := range reg.Spec.Operators {
		if o.Scope == "" {
			reg.Spec.Operators[i].Scope = apiv1alpha1.ScopePrivate
		}
		if o.InstallMode == "" {
			reg.Spec.Operators[i].InstallMode = apiv1alpha1.InstallModeNamespace
		}
	}
	return reg, nil
}

// FetchOperandConfig fetch the OperandConfig
func (m *ODLMManager) FetchOperandConfig(key types.NamespacedName) (*apiv1alpha1.OperandConfig, error) {
	config := &apiv1alpha1.OperandConfig{}
	if err := m.Reader.Get(context.TODO(), key, config); err != nil {
		return nil, err
	}
	return config, nil
}

// FetchAllOperandRequests fetch all the OperandRequests with specific label
func (m *ODLMManager) FetchAllOperandRequests(label map[string]string) (*apiv1alpha1.OperandRequestList, error) {
	requestList := &apiv1alpha1.OperandRequestList{}
	opts := []client.ListOption{}
	if label != nil {
		opts = []client.ListOption{
			client.MatchingLabels(label),
		}
	}

	if err := m.Reader.List(context.TODO(), requestList, opts...); err != nil {
		return nil, err
	}
	// Set default value for all the OperandRequest
	for i, item := range requestList.Items {
		for j, r := range item.Spec.Requests {
			if r.RegistryNamespace == "" {
				requestList.Items[i].Spec.Requests[j].RegistryNamespace = item.GetNamespace()
			}
		}
	}
	return requestList, nil
}

// FetchOperandRequest fetch OperandRequest
func (m *ODLMManager) FetchOperandRequest(key types.NamespacedName) (*apiv1alpha1.OperandRequest, error) {
	req := &apiv1alpha1.OperandRequest{}
	if err := m.Reader.Get(context.TODO(), key, req); err != nil {
		return nil, err
	}
	// Set default value for the OperandRequest
	for i, r := range req.Spec.Requests {
		if r.RegistryNamespace == "" {
			req.Spec.Requests[i].RegistryNamespace = req.GetNamespace()
		}
	}
	return req, nil
}

// FetchSubscription fetch Subscription from a name
func (m *ODLMManager) FetchSubscription(name, namespace string, packageName ...string) (*olmv1alpha1.Subscription, error) {
	klog.V(3).Infof("Fetch Subscription: %s/%s", namespace, name)
	sub := &olmv1alpha1.Subscription{}
	subKey := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := m.Reader.Get(context.TODO(), subKey, sub)
	if err == nil {
		return sub, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	subPkgKey := types.NamespacedName{
		Name:      packageName[0],
		Namespace: namespace,
	}
	err = m.Reader.Get(context.TODO(), subPkgKey, sub)
	return sub, err
}

// FetchClusterServiceVersion fetch the ClusterServiceVersion from the subscription
func (m *ODLMManager) FetchClusterServiceVersion(sub *olmv1alpha1.Subscription) (*olmv1alpha1.ClusterServiceVersion, error) {
	// Check the ClusterServiceVersion status in the subscription
	if sub.Status.CurrentCSV == "" {
		klog.Warningf("The ClusterServiceVersion for Subscription %s is not ready. Will check it again", sub.Name)
		return nil, nil
	}

	csvName := sub.Status.CurrentCSV
	csvNamespace := sub.Namespace

	if sub.Status.Install == nil || sub.Status.InstallPlanRef.Name == "" {
		klog.Warningf("The Installplan for Subscription %s is not ready. Will check it again", sub.Name)
		return nil, nil
	}

	// If the installplan is deleted after is completed, ODLM won't block the CR update.

	ipName := sub.Status.InstallPlanRef.Name
	ipNamespace := sub.Namespace
	ip := &olmv1alpha1.InstallPlan{}
	ipKey := types.NamespacedName{
		Name:      ipName,
		Namespace: ipNamespace,
	}
	if err := m.Reader.Get(context.TODO(), ipKey, ip); err != nil {
		if !errors.IsNotFound(err) {
			klog.Errorf("failed to get Installplan %s in the namespace %s: %s", ipName, ipNamespace, err)
			return nil, err
		}
	} else {
		if ip.Status.Phase == olmv1alpha1.InstallPlanPhaseFailed {
			klog.Errorf("installplan %s in the namespace %s is failed", ipName, ipNamespace)
		} else if ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
			klog.Warningf("Installplan %s in the namespace %s is not ready", ipName, ipNamespace)
			return nil, nil
		}
	}

	csv := &olmv1alpha1.ClusterServiceVersion{}
	csvKey := types.NamespacedName{
		Name:      csvName,
		Namespace: csvNamespace,
	}
	if err := m.Reader.Get(context.TODO(), csvKey, csv); err != nil {
		if errors.IsNotFound(err) {
			klog.V(3).Infof("ClusterServiceVersion %s is not ready. Will check it when it is stable", sub.Name)
			return nil, nil
		}
		klog.Errorf("failed to get ClusterServiceVersion %s in the namespace %s: %v", csvName, csvNamespace, err)
		return nil, err
	}

	klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvName, csvNamespace)
	return csv, nil
}

// GetOperatorNamespace returns the operator namespace based on the install mode
func (m *ODLMManager) GetOperatorNamespace(installMode, namespace string) string {
	if installMode == apiv1alpha1.InstallModeCluster {
		return constant.ClusterOperatorNamespace
	}
	return namespace
}
