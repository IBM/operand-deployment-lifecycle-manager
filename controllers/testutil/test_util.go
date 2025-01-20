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

package testutil

import (
	"crypto/rand"
	"fmt"
	"time"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	// +kubebuilder:scaffold:imports
)

// CPU quantities
var cpu100 = resource.NewMilliQuantity(100, resource.DecimalSI) // 100m
var cpu500 = resource.NewMilliQuantity(500, resource.DecimalSI) // 500m

// Memory quantities
var memory300 = resource.NewQuantity(300*1024*1024, resource.BinarySI) // 300Mi
var memory500 = resource.NewQuantity(500*1024*1024, resource.BinarySI) // 500Mi

var SubConfig = &olmv1alpha1.SubscriptionConfig{
	Resources: &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    *cpu500,
			corev1.ResourceMemory: *memory500},
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    *cpu100,
			corev1.ResourceMemory: *memory300},
	},
}

// CreateNSName generates random namespace names. Namespaces are never deleted in test environment
func CreateNSName(prefix string) string {
	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%x", prefix, suffix)
}

// Return OperandRegistry obj
func OperandRegistryObj(name, namespace, subNamespace string) *apiv1alpha1.OperandRegistry {
	return &apiv1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1alpha1.OperandRegistrySpec{
			Operators: []apiv1alpha1.Operator{
				{
					Name:            "jaeger",
					Namespace:       subNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jaeger",
					Channel:         "stable",
					Scope:           "public",
				},
				{
					Name:            "mongodb-atlas-kubernetes",
					Namespace:       subNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "mongodb-atlas-kubernetes",
					Channel:         "stable",
					Scope:           "public",
				},
			},
		},
	}
}

// Return OperandRegistry obj
func OperandRegistryObjwithCfg(name, namespace, subNamespace string) *apiv1alpha1.OperandRegistry {
	return &apiv1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1alpha1.OperandRegistrySpec{
			Operators: []apiv1alpha1.Operator{
				{
					Name:               "jaeger",
					Namespace:          subNamespace,
					SourceName:         "community-operators",
					SourceNamespace:    "openshift-marketplace",
					PackageName:        "jaeger",
					Channel:            "stable",
					Scope:              "public",
					SubscriptionConfig: SubConfig,
				},
				{
					Name:            "mongodb-atlas-kubernetes",
					Namespace:       subNamespace,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "mongodb-atlas-kubernetes",
					Channel:         "stable",
					Scope:           "public",
				},
			},
		},
	}
}

// Return OperandConfig obj
func OperandConfigObj(name, namespace string) *apiv1alpha1.OperandConfig {
	return &apiv1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1alpha1.OperandConfigSpec{
			Services: []apiv1alpha1.ConfigService{
				{
					Name: "jaeger",
					Spec: map[string]apiv1alpha1.ExtensionWithMarker{
						"jaeger": {
							RawExtension: runtime.RawExtension{Raw: []byte(`{
								"strategy": {
								  "templatingValueFrom": {
									"required": true,
									"configMapKeyRef": {
									  "name": "jaeger-configmap-reference",
									  "key": "putStrategy"
									}
								  }
								}
							  }`),
							},
						},
					},
					Resources: []apiv1alpha1.ConfigResource{
						{
							Name:       "jaeger-configmap",
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Labels: map[string]string{
								"jaeger": "jaeger-configmap",
							},
							Annotations: map[string]string{
								"jaeger": "jaeger-configmap",
							},
							Data: &runtime.RawExtension{
								Raw: []byte(`{"data": {"strategy": "allinone"}}`),
							},
							Force: false,
						},
						{
							Name:       "jaeger-configmap-reference",
							APIVersion: "v1",
							Kind:       "ConfigMap",
							Labels: map[string]string{
								"jaeger": "jaeger-configmap-reference",
							},
							Annotations: map[string]string{
								"jaeger": "jaeger-configmap-reference",
							},
							Data: &runtime.RawExtension{
								Raw: []byte(`{
									"data": {
									  "getStrategy": {
										"templatingValueFrom": {
										  "required": true,
										  "objectRef": {
											"apiVersion": "v1",
											"kind": "ConfigMap",
											"name": "jaeger-configmap",
											"path": "data.strategy"
										  }
										}
									  },
									  "putStrategy": "streaming"
									}
								}`),
							},
							Force: true,
						},
					},
				},
				{
					Name: "mongodb-atlas-kubernetes",
					Spec: map[string]apiv1alpha1.ExtensionWithMarker{
						"atlasDeployment": {
							RawExtension: runtime.RawExtension{Raw: []byte(`{"deploymentSpec":{"name": "test-deployment"}, "projectRef": {"name": "my-updated-project"}}`)},
						},
					},
				},
			},
		},
	}
}

// Return OperandRequest obj
func OperandRequestObj(registryName, registryNamespace, requestName, requestNamespace string) *apiv1alpha1.OperandRequest {
	return &apiv1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
			Labels: map[string]string{
				registryNamespace + "." + registryName + "/registry": "true",
			},
		},
		Spec: apiv1alpha1.OperandRequestSpec{
			Requests: []apiv1alpha1.Request{
				{
					Registry:          registryName,
					RegistryNamespace: registryNamespace,
					Operands: []apiv1alpha1.Operand{
						{
							Name: "jaeger",
						},
						{
							Name: "mongodb-atlas-kubernetes",
							Bindings: map[string]apiv1alpha1.Bindable{
								"public": {
									Secret:    "secret4",
									Configmap: "cm4",
								},
							},
						},
					},
				},
			},
		},
	}
}

// Return OperandRequest obj
func OperandRequestObjWithCR(registryName, registryNamespace, requestName, requestNamespace string) *apiv1alpha1.OperandRequest {
	return &apiv1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
			Labels: map[string]string{
				registryNamespace + "." + registryName + "/registry": "true",
			},
		},
		Spec: apiv1alpha1.OperandRequestSpec{
			Requests: []apiv1alpha1.Request{
				{
					Registry:          registryName,
					RegistryNamespace: registryNamespace,
					Operands: []apiv1alpha1.Operand{
						{
							Name:         "jaeger",
							Kind:         "Jaeger",
							APIVersion:   "jaegertracing.io/v1",
							InstanceName: "my-jaeger",
							Spec: &runtime.RawExtension{
								Raw: []byte(`{"strategy": "allinone"}`),
							},
						},
						{
							Name:       "jaeger",
							Kind:       "Jaeger",
							APIVersion: "jaegertracing.io/v1",
							Spec: &runtime.RawExtension{
								Raw: []byte(`{"strategy": "streaming"}`),
							},
						},
						{
							Name:         "mongodb-atlas-kubernetes",
							Kind:         "AtlasDeployment",
							APIVersion:   "atlas.mongodb.com/v1",
							InstanceName: "my-atlas-deployment",
							Spec: &runtime.RawExtension{
								Raw: []byte(`{
								"deploymentSpec": {
									"name": "test-deployment",
									"providerSettings": {
									  "instanceSizeName": "M10",
									  "providerName": "AWS",
									  "regionName": "US_EAST_1"
									}
								},
								"projectRef": {"name": "my-project"}}`),
							},
						},
					},
				},
			},
		},
	}
}

// OperandRequestObjWithProtected returns OperandRequest obj
func OperandRequestObjWithProtected(registryName, registryNamespace, requestName, requestNamespace string) *apiv1alpha1.OperandRequest {
	return &apiv1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      requestName,
			Namespace: requestNamespace,
			Labels: map[string]string{
				registryNamespace + "." + registryName + "/registry": "true",
			},
		},
		Spec: apiv1alpha1.OperandRequestSpec{
			Requests: []apiv1alpha1.Request{
				{
					Registry:          registryName,
					RegistryNamespace: registryNamespace,
					Operands: []apiv1alpha1.Operand{
						{
							Name: "jaeger",
						},
						{
							Name: "mongodb-atlas-kubernetes",
							Bindings: map[string]apiv1alpha1.Bindable{
								"protected": {
									Secret:    "secret5",
									Configmap: "cm5",
								},
							},
						},
					},
				},
			},
		},
	}
}

// Return OperandBindInfo obj
func OperandBindInfoObj(name, namespace, registryName, registryNamespace string) *apiv1alpha1.OperandBindInfo {
	return &apiv1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: apiv1alpha1.OperandBindInfoSpec{
			Operand:           "mongodb-atlas-kubernetes",
			Registry:          registryName,
			RegistryNamespace: registryNamespace,
			Bindings: map[string]apiv1alpha1.Bindable{
				"public": {
					Secret:    "secret1",
					Configmap: "cm1",
				},
				"private": {
					Secret:    "secret2",
					Configmap: "cm2",
				},
				"protected": {
					Secret:    "secret3",
					Configmap: "cm3",
				},
			},
		},
	}
}

func NamespaceScopeObj(namespace string) *nssv1.NamespaceScope {
	return &nssv1.NamespaceScope{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.NamespaceScopeCrName,
			Namespace: namespace,
		},
		Spec: nssv1.NamespaceScopeSpec{
			NamespaceMembers: []string{},
		},
	}
}
func OdlmNssObj(namespace string) *nssv1.NamespaceScope {
	return &nssv1.NamespaceScope{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.OdlmScopeNssCrName,
			Namespace: namespace,
		},
		Spec: nssv1.NamespaceScopeSpec{
			NamespaceMembers: []string{},
		},
	}
}
func NamespaceObj(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func ConfigmapObj(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"test": name,
		},
	}
}

func SecretObj(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"test": name,
		},
	}
}

func CatalogSource(name, namespace string) *olmv1alpha1.CatalogSource {
	return &olmv1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func CatalogSourceStatus() olmv1alpha1.CatalogSourceStatus {
	return olmv1alpha1.CatalogSourceStatus{
		GRPCConnectionState: &olmv1alpha1.GRPCConnectionState{
			LastObservedState: "READY",
			LastConnectTime: metav1.Time{
				Time: time.Unix(10, 0),
			},
		},
	}
}

// Return Subscription obj
func Subscription(name, namespace string) *olmv1alpha1.Subscription {
	labels := map[string]string{
		constant.OpreqLabel: "true",
	}
	return &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                "alpha",
			Package:                name,
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
		},
	}
}

func SubscriptionStatus(name, namespace, csvVersion string) olmv1alpha1.SubscriptionStatus {
	return olmv1alpha1.SubscriptionStatus{
		CurrentCSV:   name + "-csv.v" + csvVersion,
		InstalledCSV: name + "-csv.v" + csvVersion,
		Install: &olmv1alpha1.InstallPlanReference{
			APIVersion: "operators.coreos.com/v1alpha1",
			Kind:       "InstallPlan",
			Name:       name + "-install-plan",
			UID:        types.UID("install-plan-uid"),
		},
		InstallPlanRef: &corev1.ObjectReference{
			APIVersion: "operators.coreos.com/v1alpha1",
			Kind:       "InstallPlan",
			Name:       name + "-install-plan",
			Namespace:  namespace,
			UID:        types.UID("install-plan-uid"),
		},
		LastUpdated: metav1.Time{
			Time: time.Unix(10, 0),
		},
	}
}

func ClusterServiceVersion(name, packageName, namespace, example string) *olmv1alpha1.ClusterServiceVersion {
	return &olmv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"alm-examples": example,
			},
			Labels: map[string]string{
				"operators.coreos.com/" + packageName + "." + namespace: "",
			},
		},
		Spec: olmv1alpha1.ClusterServiceVersionSpec{
			InstallStrategy: olmv1alpha1.NamedInstallStrategy{
				StrategySpec: olmv1alpha1.StrategyDetailsDeployment{
					DeploymentSpecs: []olmv1alpha1.StrategyDeploymentSpec{},
				},
			},
		},
	}
}

func ClusterServiceVersionStatus() olmv1alpha1.ClusterServiceVersionStatus {
	return olmv1alpha1.ClusterServiceVersionStatus{
		Phase: olmv1alpha1.CSVPhaseSucceeded,
	}
}

func InstallPlan(name, namespace string) *olmv1alpha1.InstallPlan {
	return &olmv1alpha1.InstallPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: olmv1alpha1.InstallPlanSpec{
			ClusterServiceVersionNames: []string{},
		},
	}
}

func InstallPlanStatus() olmv1alpha1.InstallPlanStatus {
	return olmv1alpha1.InstallPlanStatus{
		Phase:          olmv1alpha1.InstallPlanPhaseComplete,
		CatalogSources: []string{},
	}
}
