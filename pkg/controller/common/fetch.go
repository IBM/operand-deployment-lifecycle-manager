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

package common

import (
	"context"
	"fmt"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/pkg/constant"
)

// FetchOperandRegistry fetch the OperandRegistry instance with default value
func FetchOperandRegistry(c client.Client, key types.NamespacedName) (*apiv1alpha1.OperandRegistry, error) {
	reg := &apiv1alpha1.OperandRegistry{}
	if err := c.Get(context.TODO(), key, reg); err != nil {
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

// Fetch the OperandConfig
func FetchOperandConfig(c client.Client, key types.NamespacedName) (*apiv1alpha1.OperandConfig, error) {
	config := &apiv1alpha1.OperandConfig{}
	if err := c.Get(context.TODO(), key, config); err != nil {
		return nil, err
	}
	return config, nil
}

// FetchAllOperandRequests fetch all the OperandRequests with specific label
func FetchAllOperandRequests(c client.Client, label map[string]string) (*apiv1alpha1.OperandRequestList, error) {
	requestList := &apiv1alpha1.OperandRequestList{}
	opts := []client.ListOption{}
	if label != nil {
		opts = []client.ListOption{
			client.MatchingLabels(label),
		}
	}

	if err := c.List(context.TODO(), requestList, opts...); err != nil {
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
func FetchOperandRequest(c client.Client, key types.NamespacedName) (*apiv1alpha1.OperandRequest, error) {
	req := &apiv1alpha1.OperandRequest{}
	if err := c.Get(context.TODO(), key, req); err != nil {
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

// FetchSubscription fetch Subscription from name
func FetchSubscription(olmClient olmclient.Interface, name, namespace string, packageName ...string) (*olmv1alpha1.Subscription, error) {
	sub, err := olmClient.OperatorsV1alpha1().Subscriptions(namespace).Get(name, metav1.GetOptions{})

	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if errors.IsNotFound(err) {
		sub, err = olmClient.OperatorsV1alpha1().Subscriptions(namespace).Get(packageName[0], metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	return sub, nil
}

// FetchClusterServiceVersion fetch the ClusterServiceVersion from the subscription
func FetchClusterServiceVersion(olmClient olmclient.Interface, sub *olmv1alpha1.Subscription) (*olmv1alpha1.ClusterServiceVersion, error) {
	// Check the ClusterServiceVersion status in the subscription
	if sub.Status.CurrentCSV == "" {
		klog.V(3).Infof("The ClusterServiceVersion for Subscription %s is not ready. Will check it again", sub.Name)
		return nil, nil
	}

	csvName := sub.Status.CurrentCSV
	csvNamespace := sub.Namespace

	if sub.Status.Install == nil || sub.Status.InstallPlanRef.Name == "" {
		klog.V(3).Infof("The Installplan for Subscription %s is not ready. Will check it again", sub.Name)
		return nil, nil
	}

	ipName := sub.Status.InstallPlanRef.Name
	ipNamespace := sub.Namespace

	// If the installplan is deleted after is completed, ODLM won't block the CR update.
	ip, err := olmClient.OperatorsV1alpha1().InstallPlans(ipNamespace).Get(ipName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("Failed to get Installplan %s in the namespace %s: %s", ipName, ipNamespace, err)
		return nil, err
	} else if !errors.IsNotFound(err) && ip.Status.Phase == olmv1alpha1.InstallPlanPhaseFailed {
		klog.Errorf("Installplan %s in the namespace %s is failed", ipName, ipNamespace)
		return nil, fmt.Errorf("installplan %s in the namespace %s is failed", ipName, ipNamespace)
	} else if !errors.IsNotFound(err) && ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
		klog.Infof("Installplan %s in the namespace %s is not ready", ipName, ipNamespace)
		return nil, nil
	}

	csv, getCSVErr := olmClient.OperatorsV1alpha1().ClusterServiceVersions(csvNamespace).Get(csvName, metav1.GetOptions{})
	if getCSVErr != nil {
		if errors.IsNotFound(getCSVErr) {
			klog.V(3).Infof("The Subscription %s is upgrading. Will check it when it is stable", sub.Name)
			return nil, nil
		}
		klog.Errorf("Failed to get ClusterServiceVersion %s in the namespace %s: %s", csvName, csvNamespace, getCSVErr)
		return nil, getCSVErr
	}

	klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvName, csvNamespace)
	return csv, nil
}

// GetOperatorNamespace returns the operator namespace based on the install mode
func GetOperatorNamespace(installMode, namespace string) string {
	if installMode == apiv1alpha1.InstallModeCluster {
		return constant.ClusterOperatorNamespace
	}

	return namespace
}
