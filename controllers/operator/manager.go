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

package operator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// ODLMOperator is the struct for ODLM controllers
type ODLMOperator struct {
	client.Client
	client.Reader
	*rest.Config
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// NewODLMOperator is the method to initialize an Operator struct
func NewODLMOperator(mgr manager.Manager, name string) *ODLMOperator {
	return &ODLMOperator{
		Client:   mgr.GetClient(),
		Reader:   mgr.GetAPIReader(),
		Config:   mgr.GetConfig(),
		Recorder: mgr.GetEventRecorderFor(name),
		Scheme:   mgr.GetScheme(),
	}
}

// GetOperandRegistry gets the OperandRegistry instance with default value
func (m *ODLMOperator) GetOperandRegistry(ctx context.Context, key types.NamespacedName) (*apiv1alpha1.OperandRegistry, error) {
	reg := &apiv1alpha1.OperandRegistry{}
	if err := m.Client.Get(ctx, key, reg); err != nil {
		return nil, err
	}
	// Get excluded CatalogSource from annotation
	// excluded-catalogsource: catalogsource1, catalogsource2
	var excludedCatalogSources []string
	if reg.Annotations != nil && reg.Annotations["excluded-catalogsource"] != "" {
		excludedCatalogSources = strings.Split(reg.Annotations["excluded-catalogsource"], ",")
	}

	for i, o := range reg.Spec.Operators {
		if o.Scope == "" {
			reg.Spec.Operators[i].Scope = apiv1alpha1.ScopePrivate
		}
		if o.InstallMode == "" {
			reg.Spec.Operators[i].InstallMode = apiv1alpha1.InstallModeNamespace
		}
		if o.InstallPlanApproval == "" {
			reg.Spec.Operators[i].InstallPlanApproval = olmv1alpha1.ApprovalAutomatic
		}
		if o.SourceName == "" || o.SourceNamespace == "" {
			catalogSourceName, catalogSourceNs, err := m.GetCatalogSourceFromPackage(ctx, o.PackageName, o.Namespace, o.Channel, key.Namespace, excludedCatalogSources)
			if err != nil {
				return reg, err
			}

			if catalogSourceName == "" || catalogSourceNs == "" {
				klog.V(2).Infof("no catalogsource found for %v", o.PackageName)
			}

			reg.Spec.Operators[i].SourceName, reg.Spec.Operators[i].SourceNamespace = catalogSourceName, catalogSourceNs
		}
	}
	return reg, nil
}

type CatalogSource struct {
	Name              string
	Namespace         string
	OpNamespace       string
	RegistryNamespace string
}

type sortableCatalogSource []CatalogSource

func (s sortableCatalogSource) Len() int      { return len(s) }
func (s sortableCatalogSource) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableCatalogSource) Less(i, j int) bool {
	// Check if the catalogsource is in the same namespace as OperandRegistry
	inRegistryNsI, inRegistryNsJ := s[i].Namespace == s[i].RegistryNamespace, s[j].Namespace == s[j].RegistryNamespace
	if inRegistryNsI && !inRegistryNsJ {
		return true
	}
	if !inRegistryNsI && inRegistryNsJ {
		return false
	}
	// Check if the catalogsource is in the same namespace as operator
	inOpNsI, inOpNsJ := s[i].Namespace == s[i].OpNamespace, s[j].Namespace == s[j].OpNamespace
	if inOpNsI && !inOpNsJ {
		return true
	}
	if !inOpNsI && inOpNsJ {
		return false
	}
	// If their namespaces are the same, then compare the name of the catalogsource
	if s[i].Namespace == s[j].Namespace {
		return s[i].Name < s[j].Name
	}
	return s[i].Namespace < s[j].Namespace
}

func (m *ODLMOperator) GetCatalogSourceFromPackage(ctx context.Context, packageName, namespace, channel, registryNs string, excludedCatalogSources []string) (catalogSourceName string, catalogSourceNs string, err error) {
	packageManifestList := &operatorsv1.PackageManifestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": packageName},
		client.InNamespace(namespace),
	}
	if err := m.Reader.List(ctx, packageManifestList, opts...); err != nil {
		return "", "", err
	}
	number := len(packageManifestList.Items)

	switch number {
	case 0:
		klog.V(2).Infof("Not found PackageManifest %s in the namespace %s has channel %s", packageName, namespace, channel)
		return "", "", nil
	case 1:
		if excludedCatalogSources != nil && util.Contains(excludedCatalogSources, packageManifestList.Items[0].Status.CatalogSource) {
			klog.V(2).Infof("Not found available CatalogSource for PackageManifest %s in the namespace %s, CatalogSource %s is excluded from OperandRegistry annotations", packageName, namespace, packageManifestList.Items[0].Status.CatalogSource)
			return "", "", nil
		}
		return packageManifestList.Items[0].Status.CatalogSource, packageManifestList.Items[0].Status.CatalogSourceNamespace, nil
	default:
		var catalogSourceCandidate []CatalogSource
		for _, pm := range packageManifestList.Items {
			if !channelCheck(channel, pm.Status.Channels) || (excludedCatalogSources != nil && util.Contains(excludedCatalogSources, pm.Status.CatalogSource)) {
				continue
			}
			catalogSourceCandidate = append(catalogSourceCandidate, CatalogSource{Name: pm.Status.CatalogSource, Namespace: pm.Status.CatalogSourceNamespace, OpNamespace: namespace, RegistryNamespace: registryNs})
		}
		if len(catalogSourceCandidate) == 0 {
			klog.Errorf("Not found PackageManifest %s in the namespace %s has channel %s", packageName, namespace, channel)
			return "", "", nil
		}
		// Sort CatalogSources by priority
		sort.Sort(sortableCatalogSource(catalogSourceCandidate))
		return catalogSourceCandidate[0].Name, catalogSourceCandidate[0].Namespace, nil
	}
}

func channelCheck(channelName string, channelList []operatorsv1.PackageChannel) (found bool) {
	for _, channel := range channelList {
		if channelName == channel.Name {
			return true
		}
	}
	return false
}

// ListOperandRegistry lists the OperandRegistry instance with default value
func (m *ODLMOperator) ListOperandRegistry(ctx context.Context, label map[string]string) (*apiv1alpha1.OperandRegistryList, error) {
	registryList := &apiv1alpha1.OperandRegistryList{}
	opts := []client.ListOption{}
	if label != nil {
		opts = []client.ListOption{
			client.MatchingLabels(label),
		}
	}
	if err := m.Client.List(ctx, registryList, opts...); err != nil {
		return nil, err
	}
	for index, item := range registryList.Items {
		for i, o := range item.Spec.Operators {
			if o.Scope == "" {
				registryList.Items[index].Spec.Operators[i].Scope = apiv1alpha1.ScopePrivate
			}
			if o.InstallMode == "" {
				registryList.Items[index].Spec.Operators[i].InstallMode = apiv1alpha1.InstallModeNamespace
			}
			if o.InstallPlanApproval == "" {
				registryList.Items[index].Spec.Operators[i].InstallPlanApproval = olmv1alpha1.ApprovalAutomatic
			}
		}
	}

	return registryList, nil
}

// GetOperandConfig gets the OperandConfig
func (m *ODLMOperator) GetOperandConfig(ctx context.Context, key types.NamespacedName) (*apiv1alpha1.OperandConfig, error) {
	config := &apiv1alpha1.OperandConfig{}
	if err := m.Client.Get(ctx, key, config); err != nil {
		return nil, err
	}
	return config, nil
}

// GetOperandRequest gets OperandRequest
func (m *ODLMOperator) GetOperandRequest(ctx context.Context, key types.NamespacedName) (*apiv1alpha1.OperandRequest, error) {
	req := &apiv1alpha1.OperandRequest{}
	if err := m.Client.Get(ctx, key, req); err != nil {
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

// ListOperandRequests list all the OperandRequests with specific label
func (m *ODLMOperator) ListOperandRequests(ctx context.Context, label map[string]string) (*apiv1alpha1.OperandRequestList, error) {
	requestList := &apiv1alpha1.OperandRequestList{}
	opts := []client.ListOption{}
	if label != nil {
		opts = []client.ListOption{
			client.MatchingLabels(label),
		}
	}

	if err := m.Client.List(ctx, requestList, opts...); err != nil {
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

// ListOperandRequestsByRegistry list all the OperandRequests
// using the specific OperandRegistry
func (m *ODLMOperator) ListOperandRequestsByRegistry(ctx context.Context, key types.NamespacedName) (requestList []apiv1alpha1.OperandRequest, err error) {
	requestCandidates := &apiv1alpha1.OperandRequestList{}
	if err = m.Client.List(ctx, requestCandidates); err != nil {
		return
	}
	// Set default value for all the OperandRequest
	for _, item := range requestCandidates.Items {
		for _, r := range item.Spec.Requests {
			if r.RegistryNamespace == "" {
				r.RegistryNamespace = item.GetNamespace()
			}
			if r.Registry == key.Name && r.RegistryNamespace == key.Namespace {
				requestList = append(requestList, item)
			}
		}
	}
	return
}

// ListOperandRequestsByConfig list all the OperandRequests
// using the specific OperandConfig
func (m *ODLMOperator) ListOperandRequestsByConfig(ctx context.Context, key types.NamespacedName) (requestList []apiv1alpha1.OperandRequest, err error) {
	requestCandidates := &apiv1alpha1.OperandRequestList{}
	if err = m.Client.List(ctx, requestCandidates); err != nil {
		return
	}
	// Set default value for all the OperandRequest
	for _, item := range requestCandidates.Items {
		for _, r := range item.Spec.Requests {
			if r.RegistryNamespace == "" {
				r.RegistryNamespace = item.GetNamespace()
			}
			if r.Registry == key.Name && r.RegistryNamespace == key.Namespace {
				requestList = append(requestList, item)
			}
		}
	}
	return
}

// GetSubscription gets Subscription by name and package name
func (m *ODLMOperator) GetSubscription(ctx context.Context, name, namespace, packageName string) (*olmv1alpha1.Subscription, error) {
	klog.V(3).Infof("Fetch Subscription: %s/%s", namespace, name)
	sub := &olmv1alpha1.Subscription{}
	subKey := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := m.Client.Get(ctx, subKey, sub)
	if err == nil {
		return sub, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	subList := &olmv1alpha1.SubscriptionList{}
	if err := m.Client.List(ctx, subList, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return nil, err
	}

	var subCandidates []olmv1alpha1.Subscription
	for _, sub := range subList.Items {
		if sub.Spec.Package == packageName {
			subCandidates = append(subCandidates, sub)
		}
	}

	if len(subCandidates) == 0 {
		return nil, err
	}

	if len(subCandidates) > 1 {
		return nil, fmt.Errorf("there are multiple subscriptions using package %v", packageName)
	}

	return &subCandidates[0], nil
}

// GetClusterServiceVersion gets the ClusterServiceVersion from the subscription
func (m *ODLMOperator) GetClusterServiceVersion(ctx context.Context, sub *olmv1alpha1.Subscription) (*olmv1alpha1.ClusterServiceVersion, error) {
	// Check the ClusterServiceVersion status in the subscription
	if sub.Status.InstalledCSV == "" {
		klog.Warningf("The ClusterServiceVersion for Subscription %s is not ready. Will check it again", sub.Name)
		return nil, nil
	}

	csvName := sub.Status.InstalledCSV
	csvNamespace := sub.Namespace

	csv := &olmv1alpha1.ClusterServiceVersion{}
	csvKey := types.NamespacedName{
		Name:      csvName,
		Namespace: csvNamespace,
	}
	if err := m.Client.Get(ctx, csvKey, csv); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("ClusterServiceVersion %s is not ready. Will check it when it is stable", sub.Name)
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to get ClusterServiceVersion %s/%s", csvNamespace, csvName)
	}

	klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvName, csvNamespace)
	return csv, nil
}

// GetOperatorNamespace returns the operator namespace based on the install mode
func (m *ODLMOperator) GetOperatorNamespace(installMode, namespace string) string {
	if installMode == apiv1alpha1.InstallModeCluster {
		return constant.ClusterOperatorNamespace
	}
	return namespace
}

func (m *ODLMOperator) CheckLabel(unstruct unstructured.Unstructured, labels map[string]string) bool {
	for k, v := range labels {
		if !m.HasLabel(unstruct, k) {
			return false
		}
		if unstruct.GetLabels()[k] != v {
			return false
		}
	}
	return true
}

func (m *ODLMOperator) HasLabel(cr unstructured.Unstructured, labelName string) bool {
	if cr.GetLabels() == nil {
		return false
	}
	if _, ok := cr.GetLabels()[labelName]; !ok {
		return false
	}
	return true
}

func (m *ODLMOperator) EnsureLabel(cr unstructured.Unstructured, labels map[string]string) {
	if cr.GetLabels() == nil {
		cr.SetLabels(make(map[string]string))
	}
	existingLabels := cr.GetLabels()
	for k, v := range labels {
		existingLabels[k] = v
	}
	cr.SetLabels(existingLabels)
}

func (m *ODLMOperator) EnsureAnnotation(cr unstructured.Unstructured, annotations map[string]string) {
	if cr.GetAnnotations() == nil {
		cr.SetAnnotations(make(map[string]string))
	}
	existingAnnotations := cr.GetAnnotations()
	for k, v := range annotations {
		existingAnnotations[k] = v
	}
	cr.SetAnnotations(existingAnnotations)
}
