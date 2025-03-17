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
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/stretchr/testify/mock"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
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
		sar := args.Get(1).(*authorizationv1.SelfSubjectAccessReview)
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

	// Test when SelfSubjectAccessReview is allowed
	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sar := args.Get(1).(*authorizationv1.SelfSubjectAccessReview)
		sar.Status.Allowed = true
	})

	if !operator.CheckResAuth(ctx, namespace, group, resource, verb) {
		t.Errorf("Expected CheckResAuth to return true, but got false")
	}

	// Test when SelfSubjectAccessReview is not allowed
	mockClient.mock.ExpectedCalls = nil
	mockClient.mock.On("Create", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		sar := args.Get(1).(*authorizationv1.SelfSubjectAccessReview)
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

type testOperator struct {
	*ODLMOperator
}

func TestEvaluateExpression(t *testing.T) {
	ctx := context.TODO()

	baseOperator := &ODLMOperator{
		Client: &MockClient{},
	}
	operator := &testOperator{baseOperator}

	type testCase struct {
		name           string
		expr           *util.LogicalExpression
		expectedResult bool
		expectError    bool
	}

	tests := []testCase{
		{
			name:           "Nil expression",
			expr:           nil,
			expectedResult: false,
			expectError:    true,
		},
		{
			name: "Equal comparison - true",
			expr: &util.LogicalExpression{
				Equal: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "abc"},
					Right: &util.ValueSource{Literal: "abc"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Equal comparison - false",
			expr: &util.LogicalExpression{
				Equal: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "abc"},
					Right: &util.ValueSource{Literal: "def"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "NotEqual comparison - true",
			expr: &util.LogicalExpression{
				NotEqual: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "abc"},
					Right: &util.ValueSource{Literal: "def"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "NotEqual comparison - false",
			expr: &util.LogicalExpression{
				NotEqual: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "abc"},
					Right: &util.ValueSource{Literal: "abc"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "GreaterThan comparison with numbers - true",
			expr: &util.LogicalExpression{
				GreaterThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "42"},
					Right: &util.ValueSource{Literal: "20"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "GreaterThan comparison with numbers - false",
			expr: &util.LogicalExpression{
				GreaterThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "20"},
					Right: &util.ValueSource{Literal: "42"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "GreaterThan comparison with strings",
			expr: &util.LogicalExpression{
				GreaterThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "xyz"},
					Right: &util.ValueSource{Literal: "abc"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "LessThan comparison with numbers - true",
			expr: &util.LogicalExpression{
				LessThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "20"},
					Right: &util.ValueSource{Literal: "42"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "LessThan comparison with numbers - false",
			expr: &util.LogicalExpression{
				LessThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "42"},
					Right: &util.ValueSource{Literal: "20"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "LessThan comparison with strings",
			expr: &util.LogicalExpression{
				LessThan: &util.ValueComparison{
					Left:  &util.ValueSource{Literal: "abc"},
					Right: &util.ValueSource{Literal: "xyz"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "And operator - all true",
			expr: &util.LogicalExpression{
				And: []*util.LogicalExpression{
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "abc"},
							Right: &util.ValueSource{Literal: "abc"},
						},
					},
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "def"},
							Right: &util.ValueSource{Literal: "def"},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "And operator - one false",
			expr: &util.LogicalExpression{
				And: []*util.LogicalExpression{
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "abc"},
							Right: &util.ValueSource{Literal: "abc"},
						},
					},
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "def"},
							Right: &util.ValueSource{Literal: "xyz"},
						},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Or operator - one true",
			expr: &util.LogicalExpression{
				Or: []*util.LogicalExpression{
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "abc"},
							Right: &util.ValueSource{Literal: "def"},
						},
					},
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "def"},
							Right: &util.ValueSource{Literal: "def"},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Or operator - all false",
			expr: &util.LogicalExpression{
				Or: []*util.LogicalExpression{
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "abc"},
							Right: &util.ValueSource{Literal: "def"},
						},
					},
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "ghi"},
							Right: &util.ValueSource{Literal: "xyz"},
						},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Not operator - true becomes false",
			expr: &util.LogicalExpression{
				Not: &util.LogicalExpression{
					Equal: &util.ValueComparison{
						Left:  &util.ValueSource{Literal: "abc"},
						Right: &util.ValueSource{Literal: "abc"},
					},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "Not operator - false becomes true",
			expr: &util.LogicalExpression{
				Not: &util.LogicalExpression{
					Equal: &util.ValueComparison{
						Left:  &util.ValueSource{Literal: "abc"},
						Right: &util.ValueSource{Literal: "def"},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Complex expression - And with Or",
			expr: &util.LogicalExpression{
				And: []*util.LogicalExpression{
					{
						Equal: &util.ValueComparison{
							Left:  &util.ValueSource{Literal: "abc"},
							Right: &util.ValueSource{Literal: "abc"},
						},
					},
					{
						Or: []*util.LogicalExpression{
							{
								Equal: &util.ValueComparison{
									Left:  &util.ValueSource{Literal: "def"},
									Right: &util.ValueSource{Literal: "xyz"},
								},
							},
							{
								Equal: &util.ValueComparison{
									Left:  &util.ValueSource{Literal: "ghi"},
									Right: &util.ValueSource{Literal: "ghi"},
								},
							},
						},
					},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "Error in GetValueFromSource",
			expr: &util.LogicalExpression{
				Equal: &util.ValueComparison{
					Left: &util.ValueSource{
						ObjectRef: &util.ObjectRef{
							Name: "errorObj",
							Path: "spec.value",
						},
					},
					Right: &util.ValueSource{Literal: "abc"},
				},
			},
			expectedResult: false,
			expectError:    true,
		},
		{
			name:           "Invalid expression format",
			expr:           &util.LogicalExpression{},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := operator.EvaluateExpression(ctx, tc.expr, "test", "test-name", "test-namespace")

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got %v", err)
				}
				if result != tc.expectedResult {
					t.Errorf("Expected result %v, but got %v", tc.expectedResult, result)
				}
			}
		})
	}
}

func (t *testOperator) ParseObjectRef(ctx context.Context, obj *util.ObjectRef, instanceType, instanceName, instanceNs string) (string, error) {
	if obj == nil {
		return "", nil
	}

	if obj.Name == "testObj" {
		if obj.Path == "spec.version" {
			return "1.2.3", nil
		} else if obj.Path == "spec.enabled" {
			return "true", nil
		} else if obj.Path == "spec.disabled" {
			return "false", nil
		} else if obj.Path == "spec.empty" {
			return "", nil
		}
	}
	if obj.Name == "errorObj" {
		return "", fmt.Errorf("mock error")
	}

	return "default", nil
}

func (t *testOperator) ParseConfigMapRef(ctx context.Context, cm *util.ConfigMapRef, instanceType, instanceName, instanceNs string) (string, error) {
	if cm == nil {
		return "", nil
	}

	if cm.Name == "test-cm" {
		if cm.Key == "test-key" {
			return "test-key-value", nil
		}
	}

	if cm.Name == "error-cm" {
		return "", fmt.Errorf("mock configmap error: %s", cm.Name)
	}

	return "mock-config-value", nil
}

func (t *testOperator) ParseSecretKeyRef(ctx context.Context, secret *util.SecretRef, instanceType, instanceName, instanceNs string) (string, error) {
	if secret == nil {
		return "", nil
	}

	if secret.Name == "test-secret" {
		if secret.Key == "token" {
			return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", nil
		}
	}

	if secret.Name == "error-secret" {
		return "", fmt.Errorf("mock secret error: %s", secret.Name)
	}

	return "mock-secret-value", nil
}

func (t *testOperator) GetValueFromSource(ctx context.Context, source *util.ValueSource, instanceType, instanceName, instanceNs string) (string, error) {
	if source == nil {
		return "", nil
	}

	// Return literal values directly
	if source.Literal != "" {
		return source.Literal, nil
	}

	// Use specific object/cm/secret references from the already defined method
	if source.ObjectRef != nil {
		return t.ParseObjectRef(ctx, source.ObjectRef, instanceType, instanceName, instanceNs)
	}

	if source.ConfigMapKeyRef != nil {
		return t.ParseConfigMapRef(ctx, source.ConfigMapKeyRef, instanceType, instanceName, instanceNs)
	}

	if source.SecretKeyRef != nil {
		return t.ParseSecretKeyRef(ctx, source.SecretKeyRef, instanceType, instanceName, instanceNs)
	}

	return "default", nil
}

func TestGetValueFromSource(t *testing.T) {
	ctx := context.TODO()

	baseOperator := &ODLMOperator{
		Client: &MockClient{},
	}
	operator := &testOperator{baseOperator}

	type testCase struct {
		name           string
		source         *util.ValueSource
		expectedResult string
		expectError    bool
	}

	tests := []testCase{
		{
			name:           "Nil source",
			source:         nil,
			expectedResult: "",
			expectError:    false,
		},
		{
			name: "Literal value",
			source: &util.ValueSource{
				Literal: "test-literal",
			},
			expectedResult: "test-literal",
			expectError:    false,
		},
		{
			name: "ConfigMap reference success",
			source: &util.ValueSource{
				ConfigMapKeyRef: &util.ConfigMapRef{
					Name: "test-cm",
					Key:  "test-key",
				},
			},
			expectedResult: "test-key-value",
			expectError:    false,
		},
		{
			name: "ConfigMap reference error",
			source: &util.ValueSource{
				ConfigMapKeyRef: &util.ConfigMapRef{
					Name: "error-cm",
					Key:  "test-key",
				},
			},
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "Secret reference success",
			source: &util.ValueSource{
				SecretKeyRef: &util.SecretRef{
					Name: "test-secret",
					Key:  "token",
				},
			},
			expectedResult: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectError:    false,
		},
		{
			name: "Secret reference error",
			source: &util.ValueSource{
				SecretKeyRef: &util.SecretRef{
					Name: "error-secret",
					Key:  "test-key",
				},
			},
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "Object reference success",
			source: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "testObj",
					Path: "spec.version",
				},
			},
			expectedResult: "1.2.3",
			expectError:    false,
		},
		{
			name: "Object reference error",
			source: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "errorObj",
					Path: "spec.value",
				},
			},
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "Object reference boolean - true",
			source: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "testObj",
					Path: "spec.enabled",
				},
			},
			expectedResult: "true",
			expectError:    false,
		},
		{
			name: "Object reference boolean - false",
			source: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "testObj",
					Path: "spec.disabled",
				},
			},
			expectedResult: "false",
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := operator.GetValueFromSource(ctx, tc.source, "test", "test-name", "test-namespace")

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got %v", err)
				}
				if result != tc.expectedResult {
					t.Errorf("Expected result %v, but got %v", tc.expectedResult, result)
				}
			}
		})
	}
}

func TestGetValueFromBranch(t *testing.T) {
	ctx := context.TODO()

	baseOperator := &ODLMOperator{
		Client: &MockClient{},
	}
	operator := &testOperator{baseOperator}

	type testCase struct {
		name           string
		branch         *util.ValueSource
		branchName     string
		key            string
		expectedResult string
		expectError    bool
	}

	tests := []testCase{
		{
			name:           "Nil branch",
			branch:         nil,
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "",
			expectError:    false,
		},
		{
			name: "Branch with literal value",
			branch: &util.ValueSource{
				Literal: "test-literal-value",
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "test-literal-value",
			expectError:    false,
		},
		{
			name: "Branch with array of literal values",
			branch: &util.ValueSource{
				Array: []util.ValueSource{
					{Literal: "value1"},
					{Literal: "value2"},
					{Literal: "value3"},
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: `["value1","value2","value3"]`,
			expectError:    false,
		},
		{
			name: "Branch with array containing map values",
			branch: &util.ValueSource{
				Array: []util.ValueSource{
					{
						Map: map[string]interface{}{
							"key1": "value1",
							"key2": map[string]interface{}{
								"literal": "nested-value",
							},
						},
					},
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: `[{"key1":"value1","key2":"nested-value"}]`,
			expectError:    false,
		},
		{
			name: "Branch with array containing map values with reference",
			branch: &util.ValueSource{
				Array: []util.ValueSource{
					{Literal: "value1"},
					{
						Map: map[string]interface{}{
							"hostname": "backend-service",
							"enabled":  true,
							"port":     8080,
						},
					},
					{
						Map: map[string]interface{}{
							"apikey": map[string]interface{}{
								"secretKeyRef": map[string]interface{}{
									"name": "test-secret",
									"key":  "token",
								},
							},
						},
					},
					{
						ConfigMapKeyRef: &util.ConfigMapRef{
							Name: "test-cm",
							Key:  "test-key",
						},
					},
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: `["value1",{"enabled":true,"hostname":"backend-service","port":8080},{"apikey":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},"test-key-value"]`,
			expectError:    false,
		},
		{
			name: "Branch with array containing reference values",
			branch: &util.ValueSource{
				Array: []util.ValueSource{
					{
						ObjectRef: &util.ObjectRef{
							Name: "testObj",
							Path: "spec.version",
						},
					},
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: `["1.2.3"]`,
			expectError:    false,
		},
		{
			name: "Branch with ObjectRef",
			branch: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "testObj",
					Path: "spec.version",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "1.2.3",
			expectError:    false,
		},
		{
			name: "Branch with SecretKeyRef",
			branch: &util.ValueSource{
				SecretKeyRef: &util.SecretRef{
					Name: "test-secret",
					Key:  "token",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectError:    false,
		},
		{
			name: "Branch with ConfigMapKeyRef",
			branch: &util.ValueSource{
				ConfigMapKeyRef: &util.ConfigMapRef{
					Name: "test-cm",
					Key:  "test-key",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "test-key-value",
			expectError:    false,
		},
		{
			name: "Error in ObjectRef",
			branch: &util.ValueSource{
				ObjectRef: &util.ObjectRef{
					Name: "errorObj",
					Path: "spec.value",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "Error in SecretKeyRef",
			branch: &util.ValueSource{
				SecretKeyRef: &util.SecretRef{
					Name: "error-secret",
					Key:  "test-key",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "",
			expectError:    true,
		},
		{
			name: "Error in ConfigMapKeyRef",
			branch: &util.ValueSource{
				ConfigMapKeyRef: &util.ConfigMapRef{
					Name: "error-cm",
					Key:  "test-key",
				},
			},
			branchName:     "test-branch",
			key:            "test-key",
			expectedResult: "",
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := operator.GetValueFromBranch(ctx, tc.branch, tc.branchName, tc.key, "test-type", "test-name", "test-namespace")

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got %v", err)
				}
				if result != tc.expectedResult {
					t.Errorf("Expected result %v, but got %v", tc.expectedResult, result)
				}
			}
		})
	}
}

func (t *testOperator) GetValueFromBranch(ctx context.Context, branch *util.ValueSource, branchName, key string, instanceType, instanceName, instanceNs string) (string, error) {
	if branch == nil {
		return "", nil
	}

	if branch.Literal != "" {
		return branch.Literal, nil
	}

	if len(branch.Array) > 0 {
		var processedValues []interface{}

		for _, item := range branch.Array {
			if item.Literal != "" {
				processedValues = append(processedValues, item.Literal)
			} else if len(item.Map) > 0 {
				resultMap := make(map[string]interface{})

				for k, v := range item.Map {
					if valueSourceMap, ok := v.(map[string]interface{}); ok {
						vsBytes, err := json.Marshal(valueSourceMap)
						if err != nil {
							return "", err
						}

						var vs util.ValueSource
						if err := json.Unmarshal(vsBytes, &vs); err != nil {
							return "", err
						}

						resolvedVal, err := t.GetValueFromSource(ctx, &vs, instanceType, instanceName, instanceNs)
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
				val, err := t.GetValueFromSource(ctx, &item, instanceType, instanceName, instanceNs)
				if err != nil {
					return "", err
				}
				processedValues = append(processedValues, val)
			}
		}

		jsonBytes, err := json.Marshal(processedValues)
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	}

	return t.GetValueFromSource(ctx, branch, instanceType, instanceName, instanceNs)
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
