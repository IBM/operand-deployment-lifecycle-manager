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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
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
