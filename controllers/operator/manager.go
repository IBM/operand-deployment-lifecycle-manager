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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	constant "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
)

// ODLMOperator is the struct for ODLM controllers
type ODLMOperator struct {
	client.Client
	client.Reader
	*rest.Config
	Recorder                record.EventRecorder
	Scheme                  *runtime.Scheme
	MaxConcurrentReconciles int
}

// NewODLMOperator is the method to initialize an Operator struct
func NewODLMOperator(mgr manager.Manager, name string) *ODLMOperator {
	return &ODLMOperator{
		Client:                  mgr.GetClient(),
		Reader:                  mgr.GetAPIReader(),
		Config:                  mgr.GetConfig(),
		Recorder:                mgr.GetEventRecorderFor(name),
		Scheme:                  mgr.GetScheme(),
		MaxConcurrentReconciles: 3,
	}
}

// GetOperandRegistry gets the OperandRegistry instance with default value
func (m *ODLMOperator) GetOperandRegistry(ctx context.Context, key types.NamespacedName) (*apiv1alpha1.OperandRegistry, error) {
	reg := &apiv1alpha1.OperandRegistry{}
	if err := m.Client.Get(ctx, key, reg); err != nil {
		return nil, err
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
		if o.Namespace == "" {
			reg.Spec.Operators[i].Namespace = key.Namespace
		}
	}
	return reg, nil
}

type CatalogSource struct {
	Name                 string
	Namespace            string
	OpNamespace          string
	RegistryNamespace    string
	Priority             int
	ODLMCatalog          string
	ODLMCatalogNamespace string
}

type sortableCatalogSource []CatalogSource

func (s sortableCatalogSource) Len() int      { return len(s) }
func (s sortableCatalogSource) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableCatalogSource) Less(i, j int) bool {

	// Check if the catalogsource is in the same namespace as operator.
	// CatalogSources in operator namespace (private CatalogSource) have a higher priority than those in other namespaces (global CatalogSource).
	inOpNsI, inOpNsJ := s[i].Namespace == s[i].OpNamespace, s[j].Namespace == s[j].OpNamespace
	if inOpNsI && !inOpNsJ {
		return true
	}
	if !inOpNsI && inOpNsJ {
		return false
	}

	// Compare catalogsource priorities first, higher priority comes first
	iPriority, jPriority := s[i].Priority, s[j].Priority
	if iPriority != jPriority {
		return iPriority > jPriority
	}

	// Check if the catalogsource is in the same catalog as ODLM itself.
	// CatalogSources in the same catalog as ODLM have a higher priority than those in other catalogs.
	inODLMNsI, inODLMNsJ := s[i].Name == s[i].ODLMCatalog && s[i].Namespace == s[i].ODLMCatalogNamespace, s[j].Name == s[j].ODLMCatalog && s[j].Namespace == s[j].ODLMCatalogNamespace
	if inODLMNsI && !inODLMNsJ {
		return true
	}
	if !inODLMNsI && inODLMNsJ {
		return false
	}

	// If their namespaces are the same, then compare the name of the catalogsource
	if s[i].Namespace == s[j].Namespace {
		return s[i].Name < s[j].Name
	}
	return s[i].Namespace < s[j].Namespace
}

func (m *ODLMOperator) GetCatalogSourceAndChannelFromPackage(ctx context.Context, opregCatalog, opregCatalogNs, packageName, namespace, channel string, fallbackChannels []string,
	registryNs, odlmCatalog, odlmCatalogNs string, excludedCatalogSources []string) (catalogSourceName string, catalogSourceNs string, availableChannel string, err error) {

	packageManifestList := &operatorsv1.PackageManifestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": packageName},
		client.InNamespace(namespace),
	}
	if err := m.Reader.List(ctx, packageManifestList, opts...); err != nil {
		return "", "", "", err
	}
	number := len(packageManifestList.Items)

	switch number {
	case 0:
		klog.V(2).Infof("Not found PackageManifest %s in the namespace %s has channel %s", packageName, namespace, channel)
		return opregCatalog, opregCatalogNs, channel, nil
	default:
		// Check if the CatalogSource and CatalogSource namespace are specified in OperandRegistry
		if opregCatalog != "" && opregCatalogNs != "" {
			curChannel := getFirstAvailableSemverChannelFromCatalog(packageManifestList, fallbackChannels, channel, opregCatalog, opregCatalogNs)
			if curChannel == "" {
				klog.Errorf("Not found PackageManifest %s in the namespace %s has channel %s or fallback channels %s in the CatalogSource %s in the namespace %s", packageName, namespace, channel, fallbackChannels, opregCatalog, opregCatalogNs)
				return "", "", "", nil
			}
			return opregCatalog, opregCatalogNs, curChannel, nil
		}
		// Get the CatalogSource and CatalogSource namespace from the PackageManifest
		var primaryCatalogCandidate []CatalogSource
		var fallBackChannelAndCatalogMapping = make(map[string][]CatalogSource)
		for _, pm := range packageManifestList.Items {
			if excludedCatalogSources != nil && util.Contains(excludedCatalogSources, pm.Status.CatalogSource) {
				continue
			}

			hasCatalogPermission := m.CheckResAuth(ctx, pm.Status.CatalogSourceNamespace, "operators.coreos.com", "catalogsources", "get")
			if !hasCatalogPermission {
				klog.V(2).Infof("No permission to get CatalogSource %s in the namespace %s", pm.Status.CatalogSource, pm.Status.CatalogSourceNamespace)
				continue
			}
			// Fetch the CatalogSource if cluster permission allows
			catalogsource := &olmv1alpha1.CatalogSource{}
			if err := m.Reader.Get(ctx, types.NamespacedName{Name: pm.Status.CatalogSource, Namespace: pm.Status.CatalogSourceNamespace}, catalogsource); err != nil {
				klog.Warning(err)
				continue
			}

			currentCatalog := CatalogSource{
				Name:                 pm.Status.CatalogSource,
				Namespace:            pm.Status.CatalogSourceNamespace,
				OpNamespace:          namespace,
				RegistryNamespace:    registryNs,
				Priority:             catalogsource.Spec.Priority,
				ODLMCatalog:          odlmCatalog,
				ODLMCatalogNamespace: odlmCatalogNs,
			}
			if channelCheck(channel, pm.Status.Channels) {
				primaryCatalogCandidate = append(primaryCatalogCandidate, currentCatalog)
			}
			for _, fc := range fallbackChannels {
				if channelCheck(fc, pm.Status.Channels) {
					fallBackChannelAndCatalogMapping[fc] = append(fallBackChannelAndCatalogMapping[fc], currentCatalog)
				}
			}
		}
		if len(primaryCatalogCandidate) == 0 {
			klog.Warningf("Not found PackageManifest %s in the namespace %s has channel %s", packageName, namespace, channel)
			if len(fallBackChannelAndCatalogMapping) == 0 {
				klog.Errorf("Not found PackageManifest %s in the namespace %s has fallback channels %v", packageName, namespace, fallbackChannels)
				return "", "", "", nil
			}
			fallbackChannel, fallbackCatalog := findCatalogFromFallbackChannels(fallbackChannels, fallBackChannelAndCatalogMapping)
			if len(fallbackCatalog) == 0 {
				klog.Errorf("Not found PackageManifest %s in the namespace %s has fallback channels %v", packageName, namespace, fallbackChannels)
				return "", "", "", nil
			}
			klog.Infof("Found %v CatalogSources for PackageManifest %s in the namespace %s has fallback channel %s", len(fallbackCatalog), packageName, namespace, fallbackChannel)
			primaryCatalogCandidate = fallbackCatalog
			channel = fallbackChannel
		}
		klog.V(2).Infof("Found %v CatalogSources for PackageManifest %s in the namespace %s has channel %s", len(primaryCatalogCandidate), packageName, namespace, channel)
		// Sort CatalogSources by priority
		sort.Sort(sortableCatalogSource(primaryCatalogCandidate))
		for i, c := range primaryCatalogCandidate {
			klog.V(2).Infof("The %vth sorted CatalogSource is %s in namespace %s with priority: %v", i, c.Name, c.Namespace, c.Priority)
		}
		return primaryCatalogCandidate[0].Name, primaryCatalogCandidate[0].Namespace, channel, nil
	}
}

func (m *ODLMOperator) CheckResAuth(ctx context.Context, namespace, group, resource, verb string) bool {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Group:     group,
				Resource:  resource,
				Verb:      verb,
			},
		},
	}
	if err := m.Create(ctx, sar); err != nil {
		klog.Errorf("Failed to check operator permission for Kind %s in namespace %s: %v", resource, namespace, err)
		return false
	}

	klog.V(2).Infof("Operator %s permission in namespace %s for Kind: %s, Allowed: %t, Denied: %t, Reason: %s", verb, namespace, resource, sar.Status.Allowed, sar.Status.Denied, sar.Status.Reason)

	return sar.Status.Allowed
}

func channelCheck(channelName string, channelList []operatorsv1.PackageChannel) bool {
	for _, channel := range channelList {
		if channelName == channel.Name {
			return true
		}
	}
	return false
}

func findCatalogFromFallbackChannels(fallbackChannels []string, fallBackChannelAndCatalogMapping map[string][]CatalogSource) (string, []CatalogSource) {
	var fallbackChannel string
	var fallbackCatalog []CatalogSource

	// sort fallback channels by semantic version
	semverlList, semVerChannelMappings := prunedSemverChannel(fallbackChannels)

	sort.Sort(semver.ByVersion(semverlList))
	// find the first available channel from the sorted list in descending order
	for i := len(semverlList) - 1; i >= 0; i-- {
		if catalogs, ok := fallBackChannelAndCatalogMapping[semVerChannelMappings[semverlList[i]]]; ok {
			fallbackChannel = semVerChannelMappings[semverlList[i]]
			fallbackCatalog = catalogs
			break
		}
	}
	return fallbackChannel, fallbackCatalog
}

func prunedSemverChannel(fallbackChannels []string) ([]string, map[string]string) {
	var semverlList []string
	var semVerChannelMappings = make(map[string]string)
	for _, fc := range fallbackChannels {
		semVerChannelMappings[util.FindSemantic(fc)] = fc
		semverlList = append(semverlList, util.FindSemantic(fc))
	}
	return semverlList, semVerChannelMappings
}

func getFirstAvailableSemverChannelFromCatalog(packageManifestList *operatorsv1.PackageManifestList, fallbackChannels []string, channel, catalogName, catalogNs string) string {
	semverlList, semVerChannelMappings := prunedSemverChannel(fallbackChannels)
	sort.Sort(semver.ByVersion(semverlList))

	for _, pm := range packageManifestList.Items {
		if pm.Status.CatalogSource == catalogName && pm.Status.CatalogSourceNamespace == catalogNs {
			if channelCheck(channel, pm.Status.Channels) {
				return channel
			}
			// iterate the sorted semver list in reverse order to get the first available channel
			for i := len(semverlList) - 1; i >= 0; i-- {
				if channelCheck(semverlList[i], pm.Status.Channels) {
					return semVerChannelMappings[semverlList[i]]
				}
			}
		}
	}
	return ""
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
func (m *ODLMOperator) GetSubscription(ctx context.Context, name, operatorNs, servicesNs, packageName string) (*olmv1alpha1.Subscription, error) {
	klog.V(3).Infof("Fetch Subscription %s in operatorNamespace %s and servicesNamespace %s", name, operatorNs, servicesNs)

	tenantScope := make(map[string]struct{})
	for _, ns := range []string{operatorNs, servicesNs} {
		tenantScope[ns] = struct{}{}
	}

	var subCandidates []olmv1alpha1.Subscription
	for ns := range tenantScope {
		subList := &olmv1alpha1.SubscriptionList{}
		if err := m.Client.List(ctx, subList, &client.ListOptions{
			Namespace: ns,
		}); err != nil {
			return nil, err
		}

		for _, sub := range subList.Items {
			if sub.Name == name || sub.Spec.Package == packageName {
				subCandidates = append(subCandidates, sub)
			}
		}

	}

	if len(subCandidates) == 0 {
		return nil, nil
	}

	if len(subCandidates) > 1 {
		return nil, fmt.Errorf("there are multiple subscriptions using package %v", packageName)
	}

	return &subCandidates[0], nil
}

func (m *ODLMOperator) GetOpReqCM(ctx context.Context, operatorName, operatorNs, servicesNs string) (*corev1.ConfigMap, error) {
	klog.V(3).Infof("Fetch tracking configmap %s in operatorNamespace %s and servicesNamespace %s", operatorName, operatorNs, servicesNs)

	tenantScope := []string{operatorNs}
	if operatorNs != servicesNs {
		tenantScope = append(tenantScope, servicesNs)
	}

	var cmCandidates []corev1.ConfigMap
	for _, ns := range tenantScope {
		cmList := &corev1.ConfigMapList{}
		if err := m.Client.List(ctx, cmList, &client.ListOptions{Namespace: ns}); err != nil {
			return nil, err
		}

		for _, cm := range cmList.Items {
			if cm.Annotations != nil {
				if pkg, exists := cm.Annotations["packageName"]; exists && pkg == operatorName {
					cmCandidates = append(cmCandidates, cm)
				}
			}
		}
	}

	if len(cmCandidates) == 0 {
		return nil, nil
	}

	if len(cmCandidates) > 1 {
		return nil, fmt.Errorf("there are multiple Configmaps using package %v", operatorName)
	}

	return &cmCandidates[0], nil
}

// GetClusterServiceVersion gets the ClusterServiceVersion from the subscription
func (m *ODLMOperator) GetClusterServiceVersion(ctx context.Context, sub *olmv1alpha1.Subscription) (*olmv1alpha1.ClusterServiceVersion, error) {
	// Check if subscription is nil
	if sub == nil {
		klog.Error("The subscription is nil")
		return nil, fmt.Errorf("the subscription is nil")
	}
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
	if err := m.Reader.Get(ctx, csvKey, csv); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("ClusterServiceVersion %s is not ready. Will check it when it is stable", sub.Name)
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to get ClusterServiceVersion %s/%s", csvNamespace, csvName)
	}

	klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvName, csvNamespace)
	return csv, nil
}

// GetClusterServiceVersionList gets a list of ClusterServiceVersions from the subscription
func (m *ODLMOperator) GetClusterServiceVersionList(ctx context.Context, sub *olmv1alpha1.Subscription) ([]*olmv1alpha1.ClusterServiceVersion, error) {
	// Check if subscription is nil
	if sub == nil {
		klog.Error("The subscription is nil")
		return nil, fmt.Errorf("the subscription is nil")
	}

	packageName := sub.Spec.Package
	csvNamespace := sub.Namespace
	labelKey := packageName + "." + csvNamespace
	labelKey = util.GetFirstNCharacter(labelKey, 63)

	// Get the ClusterServiceVersion list with label operators.coreos.com/packageName.csvNamespace=''
	csvList := &olmv1alpha1.ClusterServiceVersionList{}
	opts := []client.ListOption{
		client.MatchingLabels{fmt.Sprintf("operators.coreos.com/%s", labelKey): ""},
		client.InNamespace(csvNamespace),
	}

	if err := m.Reader.List(ctx, csvList, opts...); err != nil {
		if apierrors.IsNotFound(err) || len(csvList.Items) == 0 {
			klog.V(3).Infof("No ClusterServiceVersion found with label operators.coreos.com/%s", labelKey)
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to list ClusterServiceVersions with label operators.coreos.com/%s", labelKey)
	} else if len(csvList.Items) > 1 {
		klog.Warningf("Multiple ClusterServiceVersions found with label operators.coreos.com/%s", labelKey)
	}

	var csvs []*olmv1alpha1.ClusterServiceVersion
	for i := range csvList.Items {
		klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csvList.Items[i].Name, csvNamespace)
		csvs = append(csvs, &csvList.Items[i])
	}

	klog.V(3).Infof("Get %v ClusterServiceVersions in the namespace %s", len(csvs), csvNamespace)
	return csvs, nil
}

// GetClusterServiceVersionList gets a list of ClusterServiceVersions from the subscription
func (m *ODLMOperator) GetClusterServiceVersionListFromPackage(ctx context.Context, name, namespace string) ([]*olmv1alpha1.ClusterServiceVersion, error) {
	packageName := name
	csvNamespace := namespace

	csvList := &olmv1alpha1.ClusterServiceVersionList{}

	opts := []client.ListOption{
		client.InNamespace(csvNamespace),
	}

	if err := m.Reader.List(ctx, csvList, opts...); err != nil {
		if apierrors.IsNotFound(err) || len(csvList.Items) == 0 {
			klog.V(3).Infof("No ClusterServiceVersion found")
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to list ClusterServiceVersions")
	}

	var csvs []*olmv1alpha1.ClusterServiceVersion
	// filter csvList to find one(s) that contain packageName
	for _, v := range csvList.Items {
		csv := v
		if csv.Annotations == nil {
			continue
		}
		if _, ok := csv.Annotations["operatorframework.io/properties"]; !ok {
			continue
		}
		annotation := fmt.Sprintf("\"packageName\":\"%s\"", packageName)
		if !strings.Contains(csv.Annotations["operatorframework.io/properties"], annotation) {
			continue
		}
		klog.V(3).Infof("Get ClusterServiceVersion %s in the namespace %s", csv.Name, csvNamespace)
		csvs = append(csvs, &csv)
	}

	klog.V(3).Infof("Get %v ClusterServiceVersions in the namespace %s", len(csvs), csvNamespace)
	return csvs, nil
}

func (m *ODLMOperator) GetDeploymentListFromPackage(ctx context.Context, name, namespace string) ([]*appsv1.Deployment, error) {
	packageName := name
	deploymentNamespace := namespace

	deploymentList := &appsv1.DeploymentList{}

	opts := []client.ListOption{
		client.InNamespace(deploymentNamespace),
	}

	if err := m.Reader.List(ctx, deploymentList, opts...); err != nil {
		if apierrors.IsNotFound(err) || len(deploymentList.Items) == 0 {
			klog.V(1).Infof("No Deployment found")
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to list Deployments")
	}

	var deployments []*appsv1.Deployment
	// filter deploymentList to find one(s) that contain packageName
	for _, v := range deploymentList.Items {
		deployment := v
		if deployment.Annotations == nil {
			continue
		}
		// if _, ok := deployment.Annotations["operatorframework.io/properties"]; !ok {
		// 	continue
		// }
		// annotation := fmt.Sprintf("\"packageName\":\"%s\"", packageName)
		klog.V(2).Infof("Check if deployment %s matches package name %s", deployment.Name, packageName)
		if !strings.Contains(deployment.Annotations["packageName"], packageName) {
			continue
		}
		klog.V(2).Infof("Deployment %s in the namespace %s matches package name %s.", deployment.Name, deploymentNamespace, packageName)
		deployments = append(deployments, &deployment)
	}

	if len(deployments) == 0 {
		// give an error message if no deployment found and return  error
		klog.Errorf("No Deployment found with package name %s:", packageName)
		return nil, fmt.Errorf("missing deployment with package name %s, please install the %s first", packageName, packageName)
	}

	klog.V(1).Infof("Found deployment matching package name %s in the namespace %s", packageName, deploymentNamespace)
	return deployments, nil
}

func (m *ODLMOperator) DeleteRedundantCSV(ctx context.Context, csvName, operatorNs, serviceNs, packageName string) error {
	// Get the csv by its name and namespace
	csv := &olmv1alpha1.ClusterServiceVersion{}
	csvKey := types.NamespacedName{
		Name:      csvName,
		Namespace: serviceNs,
	}
	if err := m.Reader.Get(ctx, csvKey, csv); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("ClusterServiceVersion %s is not found", csvName)
			return nil
		}
		return errors.Wrapf(err, "failed to get ClusterServiceVersion %s/%s", serviceNs, csvName)
	}

	if csv.GetLabels() == nil {
		klog.V(2).Infof("ClusterServiceVersion %s in the namespace %s has no label", csvName, serviceNs)
		return nil
	}
	// Delete the CSV if the csv does not contain label operators.coreos.com/packageName.operatorNs='' AND does not contains label olm.copiedFrom: operatorNs
	if _, ok := csv.GetLabels()[fmt.Sprintf("operators.coreos.com/%s.%s", packageName, operatorNs)]; !ok {
		if _, ok := csv.Labels["olm.copiedFrom"]; !ok {
			klog.Infof("Delete the redundant ClusterServiceVersion %s in the namespace %s", csvName, serviceNs)
			if err := m.Client.Delete(ctx, csv); err != nil {
				return errors.Wrapf(err, "failed to delete ClusterServiceVersion %s/%s", serviceNs, csvName)
			}
		}
	}
	return nil
}

// GetOperatorNamespace returns the operator namespace based on the install mode
func (m *ODLMOperator) GetOperatorNamespace(installMode, namespace string) string {
	if installMode == apiv1alpha1.InstallModeCluster {
		return constant.ClusterOperatorNamespace
	}
	return namespace
}

// GetOperandFromRegistry gets the Operand from the OperandRegistry
func (m *ODLMOperator) GetOperandFromRegistry(ctx context.Context, reg *apiv1alpha1.OperandRegistry, operandName string) (*apiv1alpha1.Operator, error) {
	opt := reg.GetOperator(operandName)
	if opt == nil {
		return nil, nil
	}
	if no_olm := util.GetNoOLM(); no_olm != "true" {
		// Get excluded CatalogSource from annotation
		// excluded-catalogsource: catalogsource1, catalogsource2
		var excludedCatalogSources []string
		if reg.Annotations != nil && reg.Annotations["excluded-catalogsource"] != "" {
			excludedCatalogSources = strings.Split(reg.Annotations["excluded-catalogsource"], ",")
		}
		// Get catalog used by ODLM itself by check its own subscription
		labelKey := util.GetFirstNCharacter("ibm-odlm"+"."+util.GetOperatorNamespace(), 63)
		opts := []client.ListOption{
			client.MatchingLabels{fmt.Sprintf("operators.coreos.com/%s", labelKey): ""},
			client.InNamespace(util.GetOperatorNamespace()),
		}
		odlmCatalog := ""
		odlmCatalogNs := ""
		odlmSubList := &olmv1alpha1.SubscriptionList{}
		if err := m.Reader.List(ctx, odlmSubList, opts...); err != nil || len(odlmSubList.Items) == 0 {
			klog.Warningf("No Subscription found for ibm-odlm in the namespace %s", util.GetOperatorNamespace())
		} else {
			odlmCatalog = odlmSubList.Items[0].Spec.CatalogSource
			odlmCatalogNs = odlmSubList.Items[0].Spec.CatalogSourceNamespace
		}

		catalogSourceName, catalogSourceNs, channel, err := m.GetCatalogSourceAndChannelFromPackage(ctx, opt.SourceName, opt.SourceNamespace, opt.PackageName, opt.Namespace, opt.Channel, opt.FallbackChannels, reg.Namespace, odlmCatalog, odlmCatalogNs, excludedCatalogSources)
		if err != nil {
			return nil, err
		}

		if catalogSourceName == "" || catalogSourceNs == "" {
			klog.V(2).Infof("no catalogsource found for %v", opt.PackageName)
		}

		opt.SourceName, opt.SourceNamespace, opt.Channel = catalogSourceName, catalogSourceNs, channel
	}

	return opt, nil
}

func (m *ODLMOperator) GetOperandFromRegistryNoOLM(ctx context.Context, reg *apiv1alpha1.OperandRegistry, operandName string) (*apiv1alpha1.Operator, error) {
	opt := reg.GetOperator(operandName)
	if opt == nil {
		return nil, nil
	}
	return opt, nil
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

// ParseValueReferenceInObject processes templates in objects and resolves value references
func (m *ODLMOperator) ParseValueReferenceInObject(ctx context.Context, key string, object interface{}, finalObject map[string]interface{}, instanceType, instanceName, instanceNs string) error {
	switch obj := object.(type) {
	case map[string]any:
		return m.processMapObject(ctx, key, obj, finalObject, instanceType, instanceName, instanceNs)
	case []any:
		return m.processArrayObject(ctx, key, finalObject, instanceType, instanceName, instanceNs)
	}
	return nil
}

// processMapObject handles map-type objects and checks for templating values
func (m *ODLMOperator) processMapObject(ctx context.Context, key string, mapObj map[string]interface{}, finalObject map[string]interface{}, instanceType, instanceName, instanceNs string) error {
	for subKey, value := range mapObj {
		if subKey == "templatingValueFrom" {
			valueRef, err := m.processTemplateValue(ctx, value, key, instanceType, instanceName, instanceNs)
			if err != nil {
				return err
			}

			if valueRef == "true" {
				finalObject[key] = true
				continue
			} else if valueRef == "false" {
				finalObject[key] = false
				continue
			}

			if valueRef != "" {
				// Check if the returned value is a JSON array string and the field should be an array
				if strings.HasPrefix(valueRef, "[") && strings.HasSuffix(valueRef, "]") {
					var arrayValue []interface{}
					if err := json.Unmarshal([]byte(valueRef), &arrayValue); err == nil {
						finalObject[key] = arrayValue
						continue
					}
				}
				finalObject[key] = valueRef
			} else {
				klog.V(3).Infof("Empty value reference returned for key %s, deleting key %s from finalObject", key, key)
				delete(finalObject, key)
			}
		} else {
			if finalObject[key] == nil {
				// Skip if the key doesn't exist in finalObject
				continue
			}

			if mapValue, ok := finalObject[key].(map[string]interface{}); ok {
				if err := m.ParseValueReferenceInObject(ctx, subKey, mapObj[subKey], mapValue, instanceType, instanceName, instanceNs); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// processArrayObject handles array-type objects
func (m *ODLMOperator) processArrayObject(ctx context.Context, key string, finalObject map[string]interface{}, instanceType, instanceName, instanceNs string) error {
	if finalArray, ok := finalObject[key].([]interface{}); ok {
		for i := range finalArray {
			if mapItem, ok := finalArray[i].(map[string]interface{}); ok {
				for subKey, value := range mapItem {
					if err := m.ParseValueReferenceInObject(ctx, subKey, value, mapItem, instanceType, instanceName, instanceNs); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// processTemplateValue converts and processes a template value reference
func (m *ODLMOperator) processTemplateValue(ctx context.Context, value interface{}, key string, instanceType, instanceName, instanceNs string) (string, error) {
	valueRef := ""

	templateRef, ok := value.(map[string]interface{})
	if !ok {
		return valueRef, nil
	}

	if boolValue, hasBool := templateRef["boolean"]; hasBool {
		if b, ok := boolValue.(bool); ok {
			if b {
				return "true", nil
			}
			return "false", nil
		}
	}

	templateRefObj, err := m.convertToTemplateValueRef(templateRef, instanceType, instanceName, instanceNs)
	if err != nil {
		return "", err
	}

	// Check for conditional logic
	if templateRefObj.Conditional != nil {
		if templateRefObj.Conditional.Expression != nil {
			return m.processExpressionCondition(ctx, templateRefObj, key, instanceType, instanceName, instanceNs)
		}
		return "", fmt.Errorf("unsupported conditional type for %s %s/%s on field %s", instanceType, instanceNs, instanceName, key)
	} else {
		// Process non-conditional templates
		return m.processStandardTemplate(ctx, templateRefObj, key, instanceType, instanceName, instanceNs)
	}
}

// convertToTemplateValueRef converts a map to a TemplateValueRef struct
func (m *ODLMOperator) convertToTemplateValueRef(templateRef map[string]interface{}, instanceType, instanceName, instanceNs string) (*util.TemplateValueRef, error) {
	templateRefByte, err := json.Marshal(templateRef)
	if err != nil {
		klog.Errorf("Failed to convert templateRef to templatingValueRef struct for %s %s/%s: %v",
			instanceType, instanceNs, instanceName, err)
		return nil, err
	}

	templateRefObj := &util.TemplateValueRef{}
	if err := json.Unmarshal(templateRefByte, templateRefObj); err != nil {
		klog.Errorf("Failed to convert templateRef to templatingValueRef struct for %s %s/%s: %v",
			instanceType, instanceNs, instanceName, err)
		return nil, err
	}

	return templateRefObj, nil
}

// processExpressionCondition evaluates expression-based conditions
func (m *ODLMOperator) processExpressionCondition(ctx context.Context, templateRefObj *util.TemplateValueRef, key string, instanceType, instanceName, instanceNs string) (string, error) {
	result, err := m.EvaluateExpression(ctx, templateRefObj.Conditional.Expression, instanceType, instanceName, instanceNs)
	if err != nil {
		klog.Errorf("Failed to evaluate expression for %s %s/%s on field %s: %v",
			instanceType, instanceNs, instanceName, key, err)
		return "", err
	}

	if result {
		// Use 'then' branch when condition is true
		return m.GetValueFromBranch(ctx, templateRefObj.Conditional.Then, "then", key, instanceType, instanceName, instanceNs)
	} else {
		// Use 'else' branch when condition is false
		return m.GetValueFromBranch(ctx, templateRefObj.Conditional.Else, "else", key, instanceType, instanceName, instanceNs)
	}
}

// GetValueFromBranch retrieves a value from a specific branch (then/else)
func (m *ODLMOperator) GetValueFromBranch(ctx context.Context, branch *util.ValueSource, branchName, key string, instanceType, instanceName, instanceNs string) (string, error) {
	if branch == nil {
		return "", nil
	}

	if branch.Boolean != nil {
		if *branch.Boolean {
			return "true", nil
		}
		return "false", nil
	}

	if branch.Literal != "" {
		return branch.Literal, nil
	}

	// Handle array values
	if len(branch.Array) > 0 {
		// Create a slice to hold processed values
		var processedValues []interface{}

		for _, item := range branch.Array {
			if item.Boolean != nil {
				if *item.Boolean {
					processedValues = append(processedValues, true)
				} else {
					processedValues = append(processedValues, false)
				}
			} else if item.Literal != "" {
				// For literal values in array, add directly
				processedValues = append(processedValues, item.Literal)
			} else if len(item.Map) > 0 {
				// Handle map items
				resultMap := make(map[string]interface{})

				for k, v := range item.Map {
					if valueSourceMap, ok := v.(map[string]interface{}); ok {
						vsBytes, err := json.Marshal(valueSourceMap)
						if err != nil {
							klog.Errorf("Failed to marshal map value to JSON: %v", err)
							return "", err
						}

						var vs util.ValueSource
						if err := json.Unmarshal(vsBytes, &vs); err != nil {
							klog.Errorf("Failed to unmarshal to ValueSource: %v", err)
							return "", err
						}

						resolvedVal, err := m.GetValueFromSource(ctx, &vs, instanceType, instanceName, instanceNs)
						if err != nil {
							return "", err
						}
						resultMap[k] = resolvedVal
					} else {
						resultMap[k] = v
					}
				}
				processedValues = append(processedValues, resultMap)
			} else {
				// Handle direct reference types in array items
				val, err := m.GetValueFromSource(ctx, &item, instanceType, instanceName, instanceNs)
				if err != nil {
					return "", err
				}
				processedValues = append(processedValues, val)
			}
		}

		// Marshal the array to JSON
		jsonBytes, err := json.Marshal(processedValues)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	}

	return m.GetValueFromSource(ctx, branch, instanceType, instanceName, instanceNs)
}

// processStandardTemplate handles non-conditional templates
func (m *ODLMOperator) processStandardTemplate(ctx context.Context, templateRefObj *util.TemplateValueRef, key string, instanceType, instanceName, instanceNs string) (string, error) {
	valueRef, err := m.GetDefaultValueFromTemplate(ctx, templateRefObj, instanceType, instanceName, instanceNs)
	if err != nil {
		klog.Errorf("Failed to get default value from template for %s %s/%s on field %s: %v",
			instanceType, instanceNs, instanceName, key, err)
	}

	// Get the value from the ConfigMap reference
	if ref, err := m.ParseConfigMapRef(ctx, templateRefObj.ConfigMapKeyRef, instanceType, instanceName, instanceNs); err != nil {
		klog.Errorf("Failed to get value reference from ConfigMap for %s %s/%s on field %s: %v",
			instanceType, instanceNs, instanceName, key, err)
		return "", err
	} else if ref != "" {
		valueRef = ref
	}

	// Get the value from the secret
	if ref, err := m.ParseSecretKeyRef(ctx, templateRefObj.SecretKeyRef, instanceType, instanceName, instanceNs); err != nil {
		klog.Errorf("Failed to get value reference from Secret for %s %s/%s on field %s: %v",
			instanceType, instanceNs, instanceName, key, err)
		return "", err
	} else if ref != "" {
		valueRef = ref
	}

	// Get the value from the object
	if ref, err := m.ParseObjectRef(ctx, templateRefObj.ObjectRef, instanceType, instanceName, instanceNs); err != nil {
		klog.Errorf("Failed to get value reference from Object for %s %s/%s on field %s: %v",
			instanceType, instanceNs, instanceName, key, err)
		return "", err
	} else if ref != "" {
		valueRef = ref
	}

	if valueRef == "" && templateRefObj.Required {
		return "", errors.Errorf("Found empty value reference from template for %s %s/%s on field %s, retry in few second",
			instanceType, instanceNs, instanceName, key)
	}

	return valueRef, nil
}

func (m *ODLMOperator) EvaluateExpression(ctx context.Context, expr *util.LogicalExpression, instanceType, instanceName, instanceNs string) (bool, error) {
	if expr == nil {
		return false, errors.New("expression is nil")
	}

	// Helper function to get comparison values
	getComparisonValues := func(left, right *util.ValueSource) (string, string, error) {
		leftVal, err := m.GetValueFromSource(ctx, left, instanceType, instanceName, instanceNs)

		if err != nil {
			return "", "", err
		}
		rightVal, err := m.GetValueFromSource(ctx, right, instanceType, instanceName, instanceNs)

		if err != nil {
			return "", "", err
		}
		return leftVal, rightVal, nil
	}

	// Helper function to compare values with support for Kubernetes resource units
	compareValues := func(leftVal, rightVal string) (leftIsGreater bool, rightIsEqual bool, rightIsGreater bool) {
		// Check if both are resource quantities
		leftQty, leftErr := resource.ParseQuantity(strings.TrimSpace(leftVal))
		rightQty, rightErr := resource.ParseQuantity(strings.TrimSpace(rightVal))

		// If both are valid Kubernetes quantities, compare their values
		if leftErr == nil && rightErr == nil {
			cmp := leftQty.Cmp(rightQty)
			return cmp > 0, cmp == 0, cmp < 0
		}

		// Fall back to standard float parsing
		leftNum, leftFloatErr := strconv.ParseFloat(strings.TrimSpace(leftVal), 64)
		rightNum, rightFloatErr := strconv.ParseFloat(strings.TrimSpace(rightVal), 64)

		if leftFloatErr == nil && rightFloatErr == nil {
			return leftNum > rightNum, leftNum == rightNum, leftNum < rightNum
		}

		// Fall back to string comparison
		return leftVal > rightVal, leftVal == rightVal, leftVal < rightVal
	}

	// Handle basic comparison operators
	if expr.Equal != nil {
		leftVal, rightVal, err := getComparisonValues(expr.Equal.Left, expr.Equal.Right)
		if err != nil {
			return false, err
		}
		_, isEqual, _ := compareValues(leftVal, rightVal)
		return isEqual, nil
	}

	if expr.NotEqual != nil {
		leftVal, rightVal, err := getComparisonValues(expr.NotEqual.Left, expr.NotEqual.Right)
		if err != nil {
			return false, err
		}
		_, isEqual, _ := compareValues(leftVal, rightVal)
		return !isEqual, nil
	}

	if expr.GreaterThan != nil {
		leftVal, rightVal, err := getComparisonValues(expr.GreaterThan.Left, expr.GreaterThan.Right)
		if err != nil {
			return false, err
		}

		leftGreater, _, _ := compareValues(leftVal, rightVal)
		return leftGreater, nil
	}

	if expr.LessThan != nil {
		leftVal, rightVal, err := getComparisonValues(expr.LessThan.Left, expr.LessThan.Right)
		if err != nil {
			return false, err
		}

		_, _, leftLess := compareValues(leftVal, rightVal)
		return leftLess, nil
	}

	// Handle logical operators
	if len(expr.And) > 0 {
		for _, subExpr := range expr.And {
			result, err := m.EvaluateExpression(ctx, subExpr, instanceType, instanceName, instanceNs)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	if len(expr.Or) > 0 {
		for _, subExpr := range expr.Or {
			result, err := m.EvaluateExpression(ctx, subExpr, instanceType, instanceName, instanceNs)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	if expr.Not != nil {
		result, err := m.EvaluateExpression(ctx, expr.Not, instanceType, instanceName, instanceNs)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Handle direct value evaluation
	if expr.Value != nil {
		val, err := m.GetValueFromSource(ctx, expr.Value, instanceType, instanceName, instanceNs)
		if err != nil {
			return false, err
		}
		// Handle common boolean string representations
		switch strings.ToLower(val) {
		case "true", "1":
			return true, nil
		case "false", "0", "":
			return false, nil
		default:
			return val != "", nil
		}
	}

	return false, errors.New("invalid expression format")
}

// GetValueFromSource gets a value from the specified source
func (m *ODLMOperator) GetValueFromSource(ctx context.Context, source *util.ValueSource, instanceType, instanceName, instanceNs string) (string, error) {
	if source == nil {
		return "", nil
	}

	if source.Boolean != nil {
		if *source.Boolean {
			return "true", nil
		}
		return "false", nil
	}

	if source.Literal != "" {
		return source.Literal, nil
	}

	// Get value from Object
	if source.ObjectRef != nil {
		val, err := m.ParseObjectRef(ctx, source.ObjectRef, instanceType, instanceName, instanceNs)
		if err != nil {
			return "", err
		} else if val == "" && source.Required {
			return "", errors.Errorf("Failed to get required value from source %s, retry in few second", source.ObjectRef.Name)
		}
		return val, nil
	}

	// Get value from Secret
	if source.SecretKeyRef != nil {
		val, err := m.ParseSecretKeyRef(ctx, source.SecretKeyRef, instanceType, instanceName, instanceNs)
		if err != nil {
			return "", err
		} else if val == "" && source.Required {
			return "", errors.Errorf("Failed to get required value from source %s, retry in few second", source.SecretKeyRef.Name)
		}
		return val, nil
	}

	// Get value from ConfigMap
	if source.ConfigMapKeyRef != nil {
		val, err := m.ParseConfigMapRef(ctx, source.ConfigMapKeyRef, instanceType, instanceName, instanceNs)
		if err != nil {
			return "", err
		} else if val == "" && source.Required {
			return "", errors.Errorf("Failed to get required value from source %s, retry in few second", source.ConfigMapKeyRef.Name)
		}
		return val, nil
	}
	return "", nil
}

func (m *ODLMOperator) GetDefaultValueFromTemplate(ctx context.Context, template *util.TemplateValueRef, instanceType, instanceName, instanceNs string) (string, error) {
	if template == nil {
		return "", nil
	}
	if template.Default != nil {
		defaultValue := template.Default.DefaultValue
		if ref, err := m.ParseConfigMapRef(ctx, template.Default.ConfigMapKeyRef, instanceType, instanceName, instanceNs); err != nil {
			return "", err
		} else if ref != "" {
			defaultValue = ref
		}
		if ref, err := m.ParseSecretKeyRef(ctx, template.Default.SecretKeyRef, instanceType, instanceName, instanceNs); err != nil {
			return "", err
		} else if ref != "" {
			defaultValue = ref
		}
		if ref, err := m.ParseObjectRef(ctx, template.Default.ObjectRef, instanceType, instanceName, instanceNs); err != nil {
			return "", err
		} else if ref != "" {
			defaultValue = ref
		}

		if defaultValue == "" && template.Default.Required {
			return "", errors.Errorf("Failed to get default value from template, retry in few second")
		}
		return defaultValue, nil
	}
	return "", nil
}

func (m *ODLMOperator) ParseConfigMapRef(ctx context.Context, cm *util.ConfigMapRef, instanceType, instanceName, instanceNs string) (string, error) {
	if cm == nil {
		return "", nil
	}
	if cm.Namespace == "" {
		cm.Namespace = instanceNs
	}
	cmData, err := m.GetValueRefFromConfigMap(ctx, instanceType, instanceName, instanceNs, cm.Name, cm.Namespace, cm.Key)
	if err != nil {
		klog.Errorf("Failed to get value reference from ConfigMap %s/%s with key %s: %v", cm.Namespace, cm.Name, cm.Key, err)
		return "", err
	}
	return cmData, nil
}

func (m *ODLMOperator) ParseSecretKeyRef(ctx context.Context, secret *util.SecretRef, instanceType, instanceName, instanceNs string) (string, error) {
	if secret == nil {
		return "", nil
	}
	if secret.Namespace == "" {
		secret.Namespace = instanceNs
	}
	secretData, err := m.GetValueRefFromSecret(ctx, instanceType, instanceName, instanceNs, secret.Name, secret.Namespace, secret.Key)
	if err != nil {
		klog.Errorf("Failed to get value reference from Secret %s/%s with key %s: %v", secret.Namespace, secret.Name, secret.Key, err)
		return "", err
	}
	return secretData, nil
}

func (m *ODLMOperator) ParseObjectRef(ctx context.Context, obj *util.ObjectRef, instanceType, instanceName, instanceNs string) (string, error) {
	if obj == nil {
		return "", nil
	}
	if obj.Namespace == "" {
		obj.Namespace = instanceNs
	}
	if obj.APIVersion == "" {
		return "", errors.New("apiVersion is empty")
	}
	if obj.Kind == "" {
		return "", errors.New("kind is empty")
	}
	// get the value from the object
	objData, err := m.GetValueRefFromObject(ctx, instanceType, instanceName, instanceNs, obj.APIVersion, obj.Kind, obj.Name, obj.Namespace, obj.Path)
	if err != nil {
		klog.Errorf("Failed to get value reference from Object %s/%s with path %s: %v", obj.Namespace, obj.Name, obj.Path, err)
		return "", err
	}
	return objData, nil
}

func (m *ODLMOperator) GetValueRefFromConfigMap(ctx context.Context, instanceType, instanceName, instanceNs, cmName, cmNs, configMapKey string) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := m.Reader.Get(ctx, types.NamespacedName{Name: cmName, Namespace: cmNs}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Configmap %s/%s is not found", cmNs, cmName)
			return "", nil
		}
		return "", errors.Wrapf(err, "failed to get Configmap %s/%s", cmNs, cmName)
	}

	// Set the Value Reference label for the ConfigMap
	util.EnsureLabelsForConfigMap(cm, map[string]string{
		constant.ODLMWatchedLabel: "true",
	})
	// Set the Value Reference Annotation for the ConfigMap
	util.EnsureAnnotationForConfigMap(cm, map[string]string{
		constant.ODLMReferenceAnnotation: instanceType + "." + instanceNs + "." + instanceName,
	})
	// Update the ConfigMap with the Value Reference label
	if err := m.Update(ctx, cm); err != nil {
		return "", errors.Wrapf(err, "failed to update ConfigMap %s/%s", cm.Namespace, cm.Name)
	}
	klog.V(2).Infof("Set the Value Reference label for ConfigMap %s/%s", cm.Namespace, cm.Name)

	if cm.Data != nil {
		if data, ok := cm.Data[configMapKey]; ok {
			return data, nil
		}
	}
	return "", nil
}

func (m *ODLMOperator) GetValueRefFromSecret(ctx context.Context, instanceType, instanceName, instanceNs, secretName, secretNs, secretKey string) (string, error) {
	secret := &corev1.Secret{}
	if err := m.Reader.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNs}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Secret %s/%s is not found", secretNs, secretName)
			return "", nil
		}
		return "", errors.Wrapf(err, "failed to get Secret %s/%s", secretNs, secretName)
	}

	// Set the Value Reference label for the Secret
	util.EnsureLabelsForSecret(secret, map[string]string{
		constant.ODLMWatchedLabel: "true",
	})
	// Set the Value Reference Annotation for the Secret
	util.EnsureAnnotationsForSecret(secret, map[string]string{
		constant.ODLMReferenceAnnotation: instanceType + "." + instanceNs + "." + instanceName,
	})
	// Update the Secret with the Value Reference label
	if err := m.Update(ctx, secret); err != nil {
		return "", errors.Wrapf(err, "failed to update Secret %s/%s", secret.Namespace, secret.Name)
	}
	klog.V(2).Infof("Set the Value Reference label for Secret %s/%s", secret.Namespace, secret.Name)

	if secret.Data != nil {
		if data, ok := secret.Data[secretKey]; ok {
			return string(data), nil
		}
	}
	return "", nil
}

func (m *ODLMOperator) GetValueRefFromObject(ctx context.Context, instanceType, instanceName, instanceNs, objAPIVersion, objKind, objName, objNs, path string) (string, error) {
	var obj unstructured.Unstructured
	obj.SetAPIVersion(objAPIVersion)
	obj.SetKind(objKind)
	if err := m.Reader.Get(ctx, types.NamespacedName{Name: objName, Namespace: objNs}, &obj); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("%s %s/%s is not found", objKind, objNs, objName)
			return "", nil
		}
		return "", errors.Wrapf(err, "failed to get %s %s/%s", objKind, objNs, objName)
	}

	// Comment out the following lines as we do not need to set labels and annotations for the CRD

	// // Set the Value Reference label for the object
	// m.EnsureLabel(obj, map[string]string{
	// 	constant.ODLMWatchedLabel: "true",
	// })
	// // Set the Value Reference Annotation for the Secret
	// m.EnsureAnnotation(obj, map[string]string{
	// 	constant.ODLMReferenceAnnotation: instanceType + "." + instanceNs + "." + instanceName,
	// })
	// // Update the object with the Value Reference label
	// if err := m.Update(ctx, &obj); err != nil {
	// 	return "", errors.Wrapf(err, "failed to update %s %s/%s", objKind, obj.GetNamespace(), obj.GetName())
	// }
	// klog.V(2).Infof("Set the Value Reference label for %s %s/%s", objKind, obj.GetNamespace(), obj.GetName())

	if path == "" {
		return "", nil
	}

	sanitizedString, err := util.SanitizeObjectString(path, obj.Object)
	if err != nil {
		// Instead of returning an error for a missing path, log it as a warning and return empty string
		if strings.Contains(err.Error(), "not found") {
			klog.Warningf("Path %v from %s %s/%s was not found: %v", path, objKind, objNs, objName, err)
			return "", nil
		}
		return "", errors.Wrapf(err, "failed to parse path %v from %s %s/%s", path, obj.GetKind(), obj.GetNamespace(), obj.GetName())
	}

	klog.V(2).Infof("Get value %s from %s %s/%s", sanitizedString, objKind, obj.GetNamespace(), obj.GetName())
	return sanitizedString, nil
}

// ObjectIsUpdatedWithException checks if the object is updated except for the ODLMWatchedLabel and ODLMReferenceAnnotation
func (m *ODLMOperator) ObjectIsUpdatedWithException(oldObj, newObj *client.Object) bool {
	oldObject := *oldObj
	newObject := *newObj

	// Check if labels are the same except for ODLMWatchedLabel
	oldLabels := oldObject.GetLabels()
	newLabels := newObject.GetLabels()
	if oldLabels != nil && newLabels != nil {
		delete(oldLabels, constant.ODLMWatchedLabel)
		delete(newLabels, constant.ODLMWatchedLabel)
	}
	if !reflect.DeepEqual(oldLabels, newLabels) {
		return true
	}

	// Check if annotations are the same except for ODLMReferenceAnnotation
	oldAnnotations := oldObject.GetAnnotations()
	newAnnotations := newObject.GetAnnotations()
	if oldAnnotations != nil && newAnnotations != nil {
		delete(oldAnnotations, constant.ODLMReferenceAnnotation)
		delete(newAnnotations, constant.ODLMReferenceAnnotation)
	}
	if !reflect.DeepEqual(oldAnnotations, newAnnotations) {
		return true
	}

	// Check if other parts of the object are unchanged
	oldObject.SetLabels(nil)
	oldObject.SetAnnotations(nil)
	newObject.SetLabels(nil)
	newObject.SetAnnotations(nil)
	return !reflect.DeepEqual(oldObject, newObject)
}
