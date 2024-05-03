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

package operandrequest

import (
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
)

func TestGenerateClusterObjects(t *testing.T) {
	r := &Reconciler{}

	o := &operatorv1alpha1.Operator{
		Namespace:           "test-namespace",
		TargetNamespaces:    []string{"target-namespace"},
		InstallMode:         "namespace",
		Channel:             "test-channel",
		PackageName:         "test-package",
		SourceName:          "test-source",
		SourceNamespace:     "test-source-namespace",
		InstallPlanApproval: "Automatic",
		StartingCSV:         "test-csv",
		SubscriptionConfig: &olmv1alpha1.SubscriptionConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "key",
					Value: "value",
				},
			},
		},
	}

	registryKey := types.NamespacedName{
		Namespace: "test-registry-namespace",
		Name:      "test-registry-name",
	}
	requestKey := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-opreq",
	}

	co := r.generateClusterObjects(o, registryKey, requestKey)

	assert.NotNil(t, co.namespace)
	assert.Equal(t, "Namespace", co.namespace.Kind)
	assert.Equal(t, "v1", co.namespace.APIVersion)
	assert.Equal(t, "test-namespace", co.namespace.Name)
	assert.Equal(t, map[string]string{"operator.ibm.com/opreq-control": "true"}, co.namespace.Labels)

	assert.NotNil(t, co.operatorGroup)
	assert.Equal(t, "OperatorGroup", co.operatorGroup.Kind)
	assert.Equal(t, "test-namespace", co.operatorGroup.Namespace)
	assert.Equal(t, []string{"target-namespace"}, co.operatorGroup.Spec.TargetNamespaces)

	assert.NotNil(t, co.subscription)
	assert.Equal(t, "Subscription", co.subscription.Kind)
	assert.Equal(t, "test-namespace", co.subscription.Namespace)
	assert.Equal(t, "test-channel", co.subscription.Spec.Channel)
	assert.Equal(t, "test-package", co.subscription.Spec.Package)
	assert.Equal(t, "test-source", co.subscription.Spec.CatalogSource)
	assert.Equal(t, "test-source-namespace", co.subscription.Spec.CatalogSourceNamespace)
	assert.Equal(t, olmv1alpha1.Approval("Automatic"), co.subscription.Spec.InstallPlanApproval)
	assert.Equal(t, "test-csv", co.subscription.Spec.StartingCSV)
	assert.NotNil(t, co.subscription.Spec.Config)
	assert.Equal(t, "value", co.subscription.Spec.Config.Env[0].Value)
}

func TestCheckSubAnnotationsForUninstall(t *testing.T) {
	reqNameA := "common-service"
	reqNsA := "ibm-common-services"
	opNameA := "ibm-iam"
	opChannelA := "v3"

	reqNameB := "other-request"
	reqNsB := "other-namespace"
	opNameB := "ibm-im"
	opChannelB := "v4.0"

	reqNameC := "common-service"
	reqNsC := "ibm-common-services"
	opNameC := "common-service-postgresql"
	opChannelC := "stable-v1"

	reqNameD := "common-service"
	reqNsD := "ibm-common-services"
	opNameD := "edb-keycloak"
	opChannelD := "stable-v1"

	// The annotation key has prefix: <requestNamespace>.<requestName>.<operatorName>/*

	// When following conditions are met, the operator should be uninstalled:
	// If all remaining <prefix>/operatorNamespace annotations' values are not the same as subscription's namespace, the operator should be uninstalled.

	// When following conditions are met, the operand should be uninstalled:
	// 1. The removed/uninstalled <prefix>/request annotation's value is the same as all other <prefix>/request annotation's values.
	// 2. Operator name in removed/uninstalled <prefix>/request is different from all other <prefix>/request annotation's values.

	// Test case 1: uninstallOperator is true, uninstallOperand is true
	// The operator and operand should be uninstalled because only the remaining OperandRequest B is internal-opreq: true.
	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsA,
			Annotations: map[string]string{
				reqNsA + "." + reqNameA + "." + opNameA + "/request":           opChannelA,
				reqNsA + "." + reqNameA + "." + opNameA + "/operatorNamespace": reqNsA,
			},
		},
	}

	uninstallOperator, uninstallOperand := checkSubAnnotationsForUninstall(reqNameA, reqNsA, opNameA, sub)

	assert.True(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/request")
	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/operatorNamespace")

	// Test case 2: uninstallOperator is true, uninstallOperand is false
	// The operator should be uninstalled because only the remaining Subscription B requesting operator is not in the same namespace as the Subscription.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsA,
			Annotations: map[string]string{
				reqNsA + "." + reqNameA + "." + opNameA + "/request":           opChannelA,
				reqNsA + "." + reqNameA + "." + opNameA + "/operatorNamespace": reqNsA,
				reqNsB + "." + reqNameB + "." + opNameB + "/request":           opChannelB,
				reqNsB + "." + reqNameB + "." + opNameB + "/operatorNamespace": reqNsB,
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsA, opNameA, sub)

	assert.True(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/request")
	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameB+"/request")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameB+"/operatorNamespace")

	// Test case 3: uninstallOperator is false, uninstallOperand is true
	// The operator should not be uninstalled because the remaining Subscription B requesting operator is in the same namespace as the Subscription.
	// The operand should be uninstalled because
	// 1. the operator name in removed/uninstalled Subscription A is different from all other Subscription annotation's values,
	// 2. but the removed/uninstalled <prefix>/request annotation's value is the same as all other <prefix>/request annotation's values.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsC,
			Annotations: map[string]string{
				reqNsC + "." + reqNameC + "." + opNameC + "/request":           opChannelC,
				reqNsC + "." + reqNameC + "." + opNameC + "/operatorNamespace": reqNsC,
				reqNsD + "." + reqNameD + "." + opNameD + "/request":           opChannelD,
				reqNsD + "." + reqNameD + "." + opNameD + "/operatorNamespace": reqNsD,
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameC, reqNsC, opNameC, sub)

	assert.False(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsC+"."+reqNameC+"."+opNameC+"/request")
	assert.NotContains(t, sub.Annotations, reqNsC+"."+reqNameC+"."+opNameC+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsD+"."+reqNameD+"."+opNameD+"/request")
	assert.Contains(t, sub.Annotations, reqNsD+"."+reqNameD+"."+opNameD+"/operatorNamespace")

	// Test case 4: uninstallOperator is false, uninstallOperand is false
	// The operator and operand should not be uninstalled because at least one different OperandRequest requesting same operator
	// But the annotation of removed OperandRequest should be removed.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsA,
			Annotations: map[string]string{
				reqNsA + "." + reqNameA + "." + opNameA + "/request":           opChannelA,
				reqNsA + "." + reqNameA + "." + opNameA + "/operatorNamespace": reqNsA,
				// The remaining OperandRequest B is requesting the same operator
				reqNsB + "." + reqNameB + "." + opNameA + "/request":           opChannelA,
				reqNsB + "." + reqNameB + "." + opNameA + "/operatorNamespace": reqNsA,
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsA, opNameA, sub)

	assert.False(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/request")
	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameA+"/request")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameA+"/operatorNamespace")

	// Test case 5: uninstallOperator is false, uninstallOperand is false
	// The operator and operand should not be uninstalled because at OperandRequest request a different operator name and different channel, but the same operator namespace
	// But the annotation of removed OperandRequest should be removed.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsA,
			Annotations: map[string]string{
				reqNsA + "." + reqNameA + "." + opNameA + "/request":           opChannelA,
				reqNsA + "." + reqNameA + "." + opNameA + "/operatorNamespace": reqNsA,
				reqNsB + "." + reqNameB + "." + opNameB + "/request":           opChannelB,
				reqNsB + "." + reqNameB + "." + opNameB + "/operatorNamespace": reqNsA,
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsA, opNameA, sub)

	assert.False(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/request")
	assert.NotContains(t, sub.Annotations, reqNsA+"."+reqNameA+"."+opNameA+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameB+"/request")
	assert.Contains(t, sub.Annotations, reqNsB+"."+reqNameB+"."+opNameB+"/operatorNamespace")
}
