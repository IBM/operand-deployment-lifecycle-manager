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
	"reflect"
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/stretchr/testify/mock"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestChannelCheck(t *testing.T) {
	channelName := "stable"
	channelList := []operatorsv1.PackageChannel{
		{
			Name: "alpha",
		},
		{
			Name: "beta",
		},
		{
			Name: "stable",
		},
	}

	if !channelCheck(channelName, channelList) {
		t.Errorf("Expected channelCheck to return true, but got false")
	}

	channelName = "dev"

	if channelCheck(channelName, channelList) {
		t.Errorf("Expected channelCheck to return false, but got true")
	}
}

func TestPrunedSemverChannel(t *testing.T) {
	fallbackChannels := []string{"stable-v1", "alpha-v1.5", "v2.0.1", "stable-v2.5.0", "v4.0"}
	expectedSemverList := []string{"v1", "v1.5", "v2.0.1", "v2.5.0", "v4.0"}
	expectedSemVerChannelMappings := map[string]string{
		"v1":     "stable-v1",
		"v1.5":   "alpha-v1.5",
		"v2.0.1": "v2.0.1",
		"v2.5.0": "stable-v2.5.0",
		"v4.0":   "v4.0",
	}

	semverList, semVerChannelMappings := prunedSemverChannel(fallbackChannels)

	if len(semverList) != len(expectedSemverList) {
		t.Errorf("Expected semver list length %d, but got %d", len(expectedSemverList), len(semverList))
	}

	for i, semver := range semverList {
		if semver != expectedSemverList[i] {
			t.Errorf("Expected semver %s at index %d, but got %s", expectedSemverList[i], i, semver)
		}
	}

	if len(semVerChannelMappings) != len(expectedSemVerChannelMappings) {
		t.Errorf("Expected semver channel mappings length %d, but got %d", len(expectedSemVerChannelMappings), len(semVerChannelMappings))
	}

	for semver, channel := range semVerChannelMappings {
		expectedChannel, ok := expectedSemVerChannelMappings[semver]
		if !ok {
			t.Errorf("Unexpected semver %s in semver channel mappings", semver)
		}
		if channel != expectedChannel {
			t.Errorf("Expected channel %s for semver %s, but got %s", expectedChannel, semver, channel)
		}
	}
}

func TestFindCatalogFromFallbackChannels(t *testing.T) {
	fallbackChannels := []string{"stable-v1", "alpha-v1.5", "v2.0.1", "stable-v2.5.0", "v4.0"}
	fallBackChannelAndCatalogMapping := map[string][]CatalogSource{
		"v1":     {{Name: "catalog1", Namespace: "namespace1"}},
		"v1.5":   {{Name: "catalog2", Namespace: "namespace2"}},
		"v2.0.1": {{Name: "catalog3", Namespace: "namespace3"}},
		"v2.5.0": {{Name: "catalog4", Namespace: "namespace4"}},
		"v4.0":   {{Name: "catalog5", Namespace: "namespace5"}},
	}

	fallbackChannel, fallbackCatalog := findCatalogFromFallbackChannels(fallbackChannels, fallBackChannelAndCatalogMapping)

	expectedFallbackChannel := "v4.0"
	expectedFallbackCatalog := []CatalogSource{{Name: "catalog5", Namespace: "namespace5"}}

	if fallbackChannel != expectedFallbackChannel {
		t.Errorf("Expected fallback channel %s, but got %s", expectedFallbackChannel, fallbackChannel)
	}

	if !reflect.DeepEqual(fallbackCatalog, expectedFallbackCatalog) {
		t.Errorf("Expected fallback catalog %v, but got %v", expectedFallbackCatalog, fallbackCatalog)
	}

	// Find empty catalog
	fallBackChannelAndCatalogMapping = map[string][]CatalogSource{}

	fallbackChannel, fallbackCatalog = findCatalogFromFallbackChannels(fallbackChannels, fallBackChannelAndCatalogMapping)

	expectedFallbackChannel = ""
	expectedFallbackCatalog = nil

	if fallbackChannel != expectedFallbackChannel {
		t.Errorf("Expected fallback channel %s, but got %s", expectedFallbackChannel, fallbackChannel)
	}

	if !reflect.DeepEqual(fallbackCatalog, expectedFallbackCatalog) {
		t.Errorf("Expected fallback catalog %v, but got %v", expectedFallbackCatalog, fallbackCatalog)
	}
}

func TestGetFirstAvailableSemverChannelFromCatalog(t *testing.T) {
	packageManifestList := &operatorsv1.PackageManifestList{
		Items: []operatorsv1.PackageManifest{
			{
				Status: operatorsv1.PackageManifestStatus{
					CatalogSource:          "catalog1",
					CatalogSourceNamespace: "namespace1",
					Channels: []operatorsv1.PackageChannel{
						{
							Name: "v1.0",
						},
						{
							Name: "v2.0",
						},
					},
				},
			},
			{
				Status: operatorsv1.PackageManifestStatus{
					CatalogSource:          "catalog2",
					CatalogSourceNamespace: "namespace2",
					Channels: []operatorsv1.PackageChannel{
						{
							Name: "v1.0",
						},
						{
							Name: "v2.0",
						},
						{
							Name: "v3.0",
						},
					},
				},
			},
		},
	}

	fallbackChannels := []string{}
	channel := "v1.0"

	catalogName := "catalog1"
	catalogNs := "namespace1"

	// Test with empty fallback channels and channel exists in the catalog
	result := getFirstAvailableSemverChannelFromCatalog(packageManifestList, fallbackChannels, channel, catalogName, catalogNs)
	expectedResult := "v1.0"

	if result != expectedResult {
		t.Errorf("Expected result to be %s, but got %s", expectedResult, result)
	}

	// Test with empty fallback channels and channel does not exist in the catalog
	channel = "alpha"
	result = getFirstAvailableSemverChannelFromCatalog(packageManifestList, fallbackChannels, channel, catalogName, catalogNs)
	expectedResult = ""

	if result != expectedResult {
		t.Errorf("Expected result to be %s, but got %s", expectedResult, result)
	}

	fallbackChannels = []string{"v1.0", "v2.0"}
	channel = "v3.0"

	// Test with fallback channels and channel does not exist in the catalog, but fallback channel exists
	result = getFirstAvailableSemverChannelFromCatalog(packageManifestList, fallbackChannels, channel, catalogName, catalogNs)
	expectedResult = "v2.0"

	if result != expectedResult {
		t.Errorf("Expected result to be %s, but got %s", expectedResult, result)
	}

	catalogName = "catalog2"
	catalogNs = "namespace2"
	channel = "v3.0"

	// Test with fallback channels, but channel exist in the catalog
	result = getFirstAvailableSemverChannelFromCatalog(packageManifestList, fallbackChannels, channel, catalogName, catalogNs)
	expectedResult = "v3.0"

	if result != expectedResult {
		t.Errorf("Expected result to be %s, but got %s", expectedResult, result)
	}

}

type MockReader struct {
	PackageManifestList *operatorsv1.PackageManifestList
	CatalogSourceList   *olmv1alpha1.CatalogSourceList
}

func (m *MockReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if catalogSource, ok := obj.(*olmv1alpha1.CatalogSource); ok {
		if m.CatalogSourceList != nil {
			for _, cs := range m.CatalogSourceList.Items {
				if cs.Name == key.Name && cs.Namespace == key.Namespace {
					*catalogSource = cs
					return nil
				}
			}
		}
		return client.IgnoreNotFound(nil)
	}
	return nil
}

func (m *MockReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if packageManifestList, ok := list.(*operatorsv1.PackageManifestList); ok {
		packageManifestList.Items = m.PackageManifestList.Items
	}
	return nil
}

func TestGetCatalogSourceAndChannelFromPackage(t *testing.T) {
	ctx := context.TODO()

	packageManifestList := &operatorsv1.PackageManifestList{
		Items: []operatorsv1.PackageManifest{
			{
				Status: operatorsv1.PackageManifestStatus{
					CatalogSource:          "catalog1",
					CatalogSourceNamespace: "namespace1",
					Channels: []operatorsv1.PackageChannel{
						{
							Name: "v3.0",
						},
						{
							Name: "v2.0",
						},
					},
				},
			},
			{
				Status: operatorsv1.PackageManifestStatus{
					CatalogSource:          "catalog2",
					CatalogSourceNamespace: "namespace1",
					Channels: []operatorsv1.PackageChannel{
						{
							Name: "v1.0",
						},
						{
							Name: "v2.0",
						},
					},
				},
			},
		},
	}
	CatalogSourceList := &olmv1alpha1.CatalogSourceList{
		Items: []olmv1alpha1.CatalogSource{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog1",
					Namespace: "namespace1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "catalog2",
					Namespace: "namespace1",
				},
				Spec: olmv1alpha1.CatalogSourceSpec{
					Priority: 100,
				},
			},
		},
	}

	fakeReader := &MockReader{
		PackageManifestList: packageManifestList,
		CatalogSourceList:   CatalogSourceList,
	}

	mockClient := &MockClient{}

	operator := &ODLMOperator{
		Reader: fakeReader,
		Client: mockClient,
	}

	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sar := args.Get(1).(*authorizationv1.SubjectAccessReview)
		sar.Status.Allowed = true
	})

	registryNs := "registry-namespace"
	odlmCatalog := "odlm-catalog"
	odlmCatalogNs := "odlm-namespace"
	excludedCatalogSources := []string{"excluded1", "excluded2"}

	// Test with setting catalog source explicitly
	opregCatalog := "catalog1"
	opregCatalogNs := "namespace1"
	packageName := "package1"
	namespace := "namespace1"

	// Test with channel exists in the catalog
	channel := "v3.0"
	fallbackChannels := []string{"v2.0", "v1.0"}

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog1", catalogSourceNs, "namespace1", availableChannel, "v3.0")
	}

	// Test with channel does not exist in the catalog, but fallback channel exists
	channel = "v4.0"
	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog1", catalogSourceNs, "namespace1", availableChannel, "v2.0")
	}

	// Test with not setting catalog source explicitly
	opregCatalog = ""
	opregCatalogNs = ""

	// Test with excluded catalog sources
	excludedCatalogSources = []string{"catalog1", "catalog2"}

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "", catalogSourceNs, "", availableChannel, "")
	}

	excludedCatalogSources = []string{}

	// Test with channel does not exist in the catalog, and fallback channel does not exist
	fallbackChannels = []string{"v4.0", "v5.0"}
	channel = "v6.0"

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "", catalogSourceNs, "", availableChannel, "")
	}

	// Test with channel does not exist in the catalog, and fallback channel exists
	fallbackChannels = []string{"v3.0", "v2.0"}
	channel = "v4.0"

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog1", catalogSourceNs, "namespace1", availableChannel, "v3.0")
	}

	// Test with channel does not exist in the catalog, and fallback channel exists, and found the catalog with higher priority
	fallbackChannels = []string{"v2.0", "v1.0"}
	channel = "v4.0"

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog2", catalogSourceNs, "namespace1", availableChannel, "v2.0")
	}

	// Test with channel already exist in the catalog
	channel = "v1.0"

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog2", catalogSourceNs, "namespace1", availableChannel, "v1.0")
	}

	// Test with channel already exist in the catalog, and found the catalog with higher priority
	channel = "v2.0"

	if catalogSourceName, catalogSourceNs, availableChannel, err := operator.GetCatalogSourceAndChannelFromPackage(ctx, opregCatalog, opregCatalogNs, packageName, namespace, channel, fallbackChannels, registryNs, odlmCatalog, odlmCatalogNs, excludedCatalogSources); err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else {
		assertCatalogSourceAndChannel(t, catalogSourceName, "catalog2", catalogSourceNs, "namespace1", availableChannel, "v2.0")
	}

}

type MockClient struct {
	mock mock.Mock
	client.Client
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.mock.Called(ctx, obj, opts)
	return args.Error(0)
}

func TestCheckResAuth(t *testing.T) {
	ctx := context.TODO()

	mockClient := &MockClient{}
	operator := &ODLMOperator{
		Client: mockClient,
	}

	namespace := "test-namespace"
	group := "test-group"
	resource := "test-resource"
	verb := "get"

	// Test when SubjectAccessReview is allowed
	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sar := args.Get(1).(*authorizationv1.SubjectAccessReview)
		sar.Status.Allowed = true
	})

	if !operator.CheckResAuth(ctx, namespace, group, resource, verb) {
		t.Errorf("Expected CheckResAuth to return true, but got false")
	}

	// Test when SubjectAccessReview is not allowed
	mockClient.mock.ExpectedCalls = nil
	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sar := args.Get(1).(*authorizationv1.SubjectAccessReview)
		sar.Status.Allowed = false
	})

	if operator.CheckResAuth(ctx, namespace, group, resource, verb) {
		t.Errorf("Expected CheckResAuth to return false, but got true")
	}

	// Test when Create returns an error
	mockClient.mock.ExpectedCalls = nil
	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(fmt.Errorf("create error"))

	if operator.CheckResAuth(ctx, namespace, group, resource, verb) {
		t.Errorf("Expected CheckResAuth to return false, but got true")
	}
}

func assertCatalogSourceAndChannel(t *testing.T, catalogSourceName, expectedCatalogSourceName, catalogSourceNs, expectedCatalogSourceNs, availableChannel, expectedAvailableChannel string) {
	t.Helper()

	if catalogSourceName != expectedCatalogSourceName {
		t.Errorf("Expected catalog source name %s, but got %s", expectedCatalogSourceName, catalogSourceName)
	}

	if catalogSourceNs != expectedCatalogSourceNs {
		t.Errorf("Expected catalog source namespace %s, but got %s", expectedCatalogSourceNs, catalogSourceNs)
	}

	if availableChannel != expectedAvailableChannel {
		t.Errorf("Expected available channel %s, but got %s", expectedAvailableChannel, availableChannel)
	}
}
