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

package helpers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// newOperandConfigCR is return an OperandConfig CR object
func newOperandConfigCR(name, namespace string) *v1alpha1.OperandConfig {
	return &v1alpha1.OperandConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandConfigSpec{
			Services: []v1alpha1.ConfigService{
				{
					Name: "etcd",
					Spec: map[string]runtime.RawExtension{
						"etcdCluster": {Raw: []byte(`{"size": 1}`)},
					},
				},
				{
					Name: "jenkins",
					Spec: map[string]runtime.RawExtension{
						"jenkins": {Raw: []byte(`{"service":{"port": 8081}}`)},
					},
				},
				{
					Name: "jaeger",
					Spec: map[string]runtime.RawExtension{},
				},
			},
		},
	}
}

// newOperandConfigCR is return an OperandRegistry CR object
func newOperandRegistryCR(name, namespace string) *v1alpha1.OperandRegistry {
	return &v1alpha1.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRegistrySpec{
			Operators: []v1alpha1.Operator{
				{
					Name:            "etcd",
					Namespace:       config.TestNamespace1,
					InstallMode:     "cluster",
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "etcd",
					Channel:         "clusterwide-alpha",
				},
				{
					Name:            "jenkins",
					Namespace:       config.TestNamespace1,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jenkins-operator",
					Channel:         "alpha",
					Scope:           v1alpha1.ScopePublic,
				},
				{
					Name:            "jaeger",
					Namespace:       config.TestNamespace2,
					SourceName:      "community-operators",
					SourceNamespace: "openshift-marketplace",
					PackageName:     "jaeger",
					Channel:         "stable",
					Scope:           v1alpha1.ScopePublic,
				},
			},
		},
	}
}

// NewOperandRequestCR1 is return an OperandRequest CR object
func NewOperandRequestCR1(name, namespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          config.OperandRegistryCrName,
					RegistryNamespace: config.TestNamespace1,
					Operands: []v1alpha1.Operand{
						{
							Name: "etcd",
						},
						{
							Name: "jenkins",
						},
					},
				},
			},
		},
	}
}

// NewOperandRequestCR2 is return an OperandRequest CR object
func NewOperandRequestCR2(name, namespace string) *v1alpha1.OperandRequest {
	return &v1alpha1.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandRequestSpec{
			Requests: []v1alpha1.Request{
				{
					Registry:          config.OperandRegistryCrName,
					RegistryNamespace: config.TestNamespace1,
					Operands: []v1alpha1.Operand{
						{
							Name: "jenkins",
							Bindings: map[string]v1alpha1.SecretConfigmap{
								"public": {
									Secret:    "jenkins-operator-credentials-example",
									Configmap: "jenkins-operator-init-configuration-example",
								},
							},
						},
						{
							Name: "jaeger",
						},
					},
				},
			},
		},
	}
}

// newOperandBindInfoCR is return an OperandBindInfo CR object
func newOperandBindInfoCR(name, namespace string) *v1alpha1.OperandBindInfo {
	return &v1alpha1.OperandBindInfo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.OperandBindInfoSpec{
			Operand:           "jenkins",
			Registry:          config.OperandRegistryCrName,
			RegistryNamespace: config.TestNamespace1,

			Bindings: map[string]v1alpha1.SecretConfigmap{
				"public": {
					Secret:    "jenkins-operator-credentials-example",
					Configmap: "jenkins-operator-init-configuration-example",
				},
			},
		},
	}
}
