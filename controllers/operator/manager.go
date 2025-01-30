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
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	maxChannel := util.FindMaxSemver("", semverlList, semVerChannelMappings)
	if catalogSources, ok := fallBackChannelAndCatalogMapping[maxChannel]; ok {
		fallbackChannel = maxChannel
		fallbackCatalog = append(fallbackCatalog, catalogSources...)
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
		klog.V(1).Infof("Get Deployment %s with package name %s", deployment.Name, packageName)
		if !strings.Contains(deployment.Annotations["packageName"], packageName) {
			continue
		}
		klog.V(1).Infof("Get Deployment %s in the namespace %s", deployment.Name, deploymentNamespace)
		deployments = append(deployments, &deployment)
	}

	klog.V(1).Infof("Get %v / %v Deployment in the namespace %s", len(deployments), len(deploymentList.Items), deploymentNamespace)
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

func (m *ODLMOperator) ParseValueReferenceInObject(ctx context.Context, key string, object interface{}, finalObject map[string]interface{}, instanceType, instanceName, instanceNs string) error {
	switch object.(type) {
	case map[string]interface{}:
		for subKey, value := range object.(map[string]interface{}) {
			if subKey == "templatingValueFrom" {
				valueRef := ""
				if templateRef, ok := value.(map[string]interface{}); ok {
					// convert templateRef to templatingValueRef struct
					templateRefByte, err := json.Marshal(templateRef)
					if err != nil {
						klog.Errorf("Failed to convert templateRef to templatingValueRef struct for %s %s/%s: %v", instanceType, instanceNs, instanceName, err)
						return err
					}
					templateRefObj := &util.TemplateValueRef{}
					if err := json.Unmarshal(templateRefByte, templateRefObj); err != nil {
						klog.Errorf("Failed to convert templateRef to templatingValueRef struct for %s %s/%s: %v", instanceType, instanceNs, instanceName, err)
						return err
					}

					// get the defaultValue from template
					valueRef, err = m.GetDefaultValueFromTemplate(ctx, templateRefObj, instanceType, instanceName, instanceNs)
					if err != nil {
						klog.Errorf("Failed to get default value from template for %s %s/%s on field %s: %v", instanceType, instanceNs, instanceName, key, err)
					}

					// get the value from the ConfigMap reference
					if ref, err := m.ParseConfigMapRef(ctx, templateRefObj.ConfigMapKeyRef, instanceType, instanceName, instanceNs); err != nil {
						klog.Errorf("Failed to get value reference from ConfigMap for %s %s/%s on field %s: %v", instanceType, instanceNs, instanceName, key, err)
						return err
					} else if ref != "" {
						valueRef = ref
					}

					// get the value from the secret
					if ref, err := m.ParseSecretKeyRef(ctx, templateRefObj.SecretRef, instanceType, instanceName, instanceNs); err != nil {
						klog.Errorf("Failed to get value reference from Secret for %s %s/%s on field %s: %v", instanceType, instanceNs, instanceName, key, err)
						return err
					} else if ref != "" {
						valueRef = ref
					}

					// get the value from the object
					if ref, err := m.ParseObjectRef(ctx, templateRefObj.ObjectRef, instanceType, instanceName, instanceNs); err != nil {
						klog.Errorf("Failed to get value reference from Object for %s %s/%s on field %s: %v", instanceType, instanceNs, instanceName, key, err)
						return err
					} else if ref != "" {
						valueRef = ref
					}

					if valueRef == "" && templateRefObj.Required {
						return errors.Errorf("Found empty value reference from template for %s %s/%s on field %s, retry in few second", instanceType, instanceNs, instanceName, key)
					}
				}
				// overwrite the value with the value from the reference
				finalObject[key] = valueRef
			} else {
				if err := m.ParseValueReferenceInObject(ctx, subKey, object.(map[string]interface{})[subKey], finalObject[key].(map[string]interface{}), instanceType, instanceName, instanceNs); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for i := range finalObject[key].([]interface{}) {
			if _, ok := finalObject[key].([]interface{})[i].(map[string]interface{}); ok {
				for subKey, value := range finalObject[key].([]interface{})[i].(map[string]interface{}) {
					if err := m.ParseValueReferenceInObject(ctx, subKey, value, finalObject[key].([]interface{})[i].(map[string]interface{}), instanceType, instanceName, instanceNs); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
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
		if ref, err := m.ParseSecretKeyRef(ctx, template.Default.SecretRef, instanceType, instanceName, instanceNs); err != nil {
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

	// Set the Value Reference label for the object
	m.EnsureLabel(obj, map[string]string{
		constant.ODLMWatchedLabel: "true",
	})
	// Set the Value Reference Annotation for the Secret
	m.EnsureAnnotation(obj, map[string]string{
		constant.ODLMReferenceAnnotation: instanceType + "." + instanceNs + "." + instanceName,
	})
	// Update the object with the Value Reference label
	if err := m.Update(ctx, &obj); err != nil {
		return "", errors.Wrapf(err, "failed to update %s %s/%s", objKind, obj.GetNamespace(), obj.GetName())
	}
	klog.V(2).Infof("Set the Value Reference label for %s %s/%s", objKind, obj.GetNamespace(), obj.GetName())

	if path == "" {
		return "", nil
	}

	sanitizedString, err := util.SanitizeObjectString(path, obj.Object)
	if err != nil {
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
